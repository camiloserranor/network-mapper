// Package builder transforms raw gNMI collection results into the v2
// hierarchical topology schema. It is a pure-function package with no I/O,
// no side effects, and no network calls — making it fully testable with
// saved collection data.
//
// Usage:
//
//	result := collector.CollectRaw(ctx, cfg)  // stage 1: raw gNMI data
//	topo := builder.Build(result)             // stage 2: structured topology
//	json.Marshal(topo)                        // stage 3: serialize for UI / humans
package builder

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/camiloserranor/network-mapper/internal/collector"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// ToolVersion is embedded at build time. Builder consumers may override it.
var ToolVersion = "dev"

// Build converts raw collection results into the v2 hierarchical topology.
// It is a pure function: deterministic output for a given input, no I/O.
func Build(cr *collector.CollectionResult) *topology.TopologyV2 {
	b := &buildState{
		cr:             cr,
		deviceMap:      make(map[string]*deviceEntry),
		systemNameToID: buildSystemNameIndex(cr.Switches),
	}

	b.ingestSwitches()
	b.ingestLLDPNeighbors()
	b.discoverHostsFromDescriptions()
	b.enrichHosts(cr)
	b.mergeDualHomedHosts()
	b.promoteUnknownOnPhysicalPorts()
	b.correlateEndpoints(cr)
	b.buildVLANs(cr)

	return b.assemble()
}

// BuildFromV1 converts a legacy v1 Topology to a v2 TopologyV2. This is
// useful for converting existing snapshot files.
func BuildFromV1(v1 *topology.Topology) *topology.TopologyV2 {
	v2 := &topology.TopologyV2{
		SchemaVersion: "2.0",
		Metadata: topology.Metadata{
			CollectedAt:    v1.CollectedAt,
			Tool:           "network-mapper",
			ToolVersion:    ToolVersion,
			SourceSwitches: v1.SourceSwitches,
		},
	}

	// Classify devices into fabric switches, compute hosts, and unknown
	for _, d := range v1.Devices {
		switch d.Type {
		case "switch":
			sw := deviceToFabricSwitch(d)
			v2.Fabric.Switches = append(v2.Fabric.Switches, sw)
		case "host":
			h := deviceToComputeHost(d)
			v2.Compute.Hosts = append(v2.Compute.Hosts, h)
		default:
			ud := topology.UnknownDevice{
				ID:                d.ID,
					DeviceType:        d.Type,
					ChassisID:         d.ChassisID,
					ManagementAddress: d.ManagementAddress,
					SystemDescription: d.SystemDescription,
				}
			if v2.UnknownDevices == nil {
				v2.UnknownDevices = &topology.UnknownDeviceSet{}
			}
			v2.UnknownDevices.Items = append(v2.UnknownDevices.Items, ud)
		}
	}

	// Distribute links as peer_links (switch↔switch) or connected_hosts
	switchIDs := make(map[string]bool)
	for _, sw := range v2.Fabric.Switches {
		switchIDs[sw.ID] = true
	}

	for _, link := range v1.Links {
		localIsSwitch := switchIDs[link.LocalDevice]
		remoteIsSwitch := switchIDs[link.RemoteDevice]

		if localIsSwitch && remoteIsSwitch {
			// Peer link — add to local switch
			addPeerLink(v2, link)
		} else if localIsSwitch && !remoteIsSwitch {
			// Host/unknown connected to switch
			addConnectedHost(v2, link)
			addHostConnection(v2, link, switchIDs)
		}
		// Add attachment info for unknown devices
		if !localIsSwitch || !remoteIsSwitch {
			addUnknownAttachment(v2, link, switchIDs)
		}
	}

	// Convert v1 endpoints
	if len(v1.Endpoints) > 0 {
		v2.Compute.UnattributedEndpoints = &topology.UnattributedEndpointSet{}
		for _, ep := range v1.Endpoints {
			he := topology.HostEndpoint{
				MAC:             ep.MAC,
				IPs:             ep.IPs,
				VLANs:           ep.VLANs,
				Type:            ep.Type,
				LearnedOnSwitch: ep.SwitchID,
				LearnedOnPort:   ep.HostPort,
			}
			if ep.HostDevice != "" {
				// Try to attribute to a host
				attributed := false
				for i := range v2.Compute.Hosts {
					if v2.Compute.Hosts[i].ID == ep.HostDevice {
						v2.Compute.Hosts[i].Endpoints = append(v2.Compute.Hosts[i].Endpoints, he)
						attributed = true
						break
					}
				}
				if !attributed {
					v2.Compute.UnattributedEndpoints.Items = append(v2.Compute.UnattributedEndpoints.Items, he)
				}
			} else {
				v2.Compute.UnattributedEndpoints.Items = append(v2.Compute.UnattributedEndpoints.Items, he)
			}
		}
		v2.Compute.UnattributedEndpoints.Count = len(v2.Compute.UnattributedEndpoints.Items)
		if v2.Compute.UnattributedEndpoints.Count == 0 {
			v2.Compute.UnattributedEndpoints = nil
		}
	}

	// Convert v1 VLANs
	for _, vlan := range v1.VLANs {
		entry := topology.VLANEntry{ID: vlan.ID}
		// V1 VLANs use MemberPorts (flat list) with SourceSwitch
		if vlan.SourceSwitch != "" && len(vlan.MemberPorts) > 0 {
			vs := topology.VLANSwitch{SwitchName: vlan.SourceSwitch}
			vs.AccessPorts = append(vs.AccessPorts, vlan.MemberPorts...)
			sort.Strings(vs.AccessPorts)
			entry.Switches = append(entry.Switches, vs)
		}
		v2.VLANs.Items = append(v2.VLANs.Items, entry)
	}

	// Compute warnings from partial failures
	for _, pf := range v1.PartialFailures {
		v2.Warnings = append(v2.Warnings, pf)
	}

	computeSummary(v2)
	return v2
}

// --- internal build state ---

type deviceEntry struct {
	device    topology.Device
	neighbors []transform.LLDPNeighbor
	isSwitch  bool
}

type buildState struct {
	cr             *collector.CollectionResult
	deviceMap      map[string]*deviceEntry
	systemNameToID map[string]string
	links          []linkInfo
	endpoints      []topology.Endpoint
}

type linkInfo struct {
	localDevice  string
	localPort    string
	remoteDevice string
	remotePort   string
	chassisID    string
	operStatus   string
	speed        string
	mtu          string
}

// connectionInfo describes a device's connection to a switch port.
type connectionInfo struct {
	switchID  string
	localPort string
}

// buildSystemNameIndex creates a mapping from system hostname → switch config ID.
func buildSystemNameIndex(switches []collector.SwitchData) map[string]string {
	idx := make(map[string]string)
	for _, sw := range switches {
		sysName := sw.Device.SystemName
		if sysName != "" && sysName != sw.SwitchID {
			idx[sysName] = sw.SwitchID
		}
	}
	return idx
}

// ingestSwitches adds all directly-queried switches to the device map.
func (b *buildState) ingestSwitches() {
	for _, sw := range b.cr.Switches {
		dev := sw.Device
		dev.Interfaces = sw.Interfaces
		dev.BGPSessions = bgpNeighborsToSessions(sw.BGPNeighbors)
		if dev.ID != "" {
			b.deviceMap[dev.ID] = &deviceEntry{device: dev, isSwitch: true}
		}
	}
}

// ingestLLDPNeighbors creates devices and links from LLDP data.
func (b *buildState) ingestLLDPNeighbors() {
	now := b.cr.CollectedAt

	for _, sw := range b.cr.Switches {
		for _, nbr := range sw.Neighbors {
			remoteID := nbr.SystemName
			if remoteID == "" {
				remoteID = nbr.ChassisID
			}
			if remoteID == "" {
				continue
			}
			remoteID = resolveDeviceID(remoteID, b.systemNameToID)

			// Merge or create remote device
			if existing, ok := b.deviceMap[remoteID]; ok {
				if existing.device.ChassisID == "" {
					existing.device.ChassisID = nbr.ChassisID
				}
				if existing.device.ManagementAddress == "" {
					existing.device.ManagementAddress = nbr.ManagementAddress
				}
				if existing.device.SystemDescription == "" {
					existing.device.SystemDescription = nbr.SystemDescription
				}
			} else {
				devType := transform.ClassifyDevice(nbr.SystemDescription, nbr.SystemName, nbr.Capabilities)
					annotations := map[string]string{}
					switch {
					case nbr.Capabilities != "":
						annotations["classification_source"] = "lldp_capabilities"
					case devType != "unknown":
						annotations["classification_source"] = "lldp_description"
					}
					b.deviceMap[remoteID] = &deviceEntry{
						device: topology.Device{
							ID:                remoteID,
							Type:              devType,
							ChassisID:         nbr.ChassisID,
							SystemName:        nbr.SystemName,
							SystemDescription: nbr.SystemDescription,
							ManagementAddress: nbr.ManagementAddress,
							Annotations:       annotations,
						},
						isSwitch: devType == "switch",
					}
				}

			// Build link with interface enrichment
			li := linkInfo{
				localDevice:  sw.SwitchID,
				localPort:    nbr.LocalPort,
				remoteDevice: remoteID,
				remotePort:   nbr.PortID,
				chassisID:    nbr.ChassisID,
			}
			for _, iface := range sw.Interfaces {
				if iface.Name == nbr.LocalPort {
					li.operStatus = iface.OperStatus
					li.speed = iface.Speed
					if iface.MTU > 0 {
						li.mtu = fmt.Sprintf("%d", iface.MTU)
					}
					break
				}
			}
			b.links = append(b.links, li)
			_ = now // collected_at available for future use
		}
	}
}

// discoverHostsFromDescriptions infers host connections from interface port
// descriptions on UP ports that have no LLDP neighbor. This covers environments
// where hosts don't run LLDP but switches are configured with descriptive port names.
func (b *buildState) discoverHostsFromDescriptions() {
	// Build set of switch ports that already have links (from LLDP)
	linkedPorts := make(map[string]bool) // "switchID|portName"
	for _, li := range b.links {
		linkedPorts[li.localDevice+"|"+li.localPort] = true
	}

	// Known switch-indicator keywords in port descriptions (case-insensitive matching)
	switchKeywords := []string{"spine", "tor", "leaf", "switch", "router", "fw", "firewall"}

	for _, sw := range b.cr.Switches {
		for _, iface := range sw.Interfaces {
			// Only consider UP ports with descriptions that aren't already linked
			if iface.OperStatus != "UP" || iface.Description == "" {
				continue
			}
			portKey := sw.SwitchID + "|" + iface.Name
			if linkedPorts[portKey] {
				continue
			}

			// Skip loopback, management, and virtual interfaces
			name := strings.ToLower(iface.Name)
			if strings.HasPrefix(name, "lo") || strings.HasPrefix(name, "mgmt") ||
				strings.HasPrefix(name, "nve") || strings.HasPrefix(name, "vlan") ||
				strings.HasPrefix(name, "sup") {
				continue
			}

			// Skip descriptions that look like switch-to-switch links.
				// Use word-boundary matching: "switch" should match "to-switch-1"
				// but NOT "Switched-Compute" where "switch" is a prefix of "switched".
				descLower := strings.ToLower(iface.Description)
				isSwitch := false
				for _, kw := range switchKeywords {
					if containsWholeWord(descLower, kw) {
						isSwitch = true
						break
					}
				}
				if isSwitch {
					continue
				}

			// Create or merge a host device from the port description.
				// If a device with this ID already exists (e.g., from LLDP), just add
				// the link — don't overwrite the existing classification.
				hostID := iface.Description
				if _, exists := b.deviceMap[hostID]; !exists {
					b.deviceMap[hostID] = &deviceEntry{
						device: topology.Device{
							ID:         hostID,
							Type:       "host",
							SystemName: hostID,
							Annotations: map[string]string{
								"classification_source": "port_description",
							},
						},
						isSwitch: false,
					}
				}

			// Create a link from switch port to inferred host
			li := linkInfo{
				localDevice: sw.SwitchID,
				localPort:   iface.Name,
				remoteDevice: hostID,
				remotePort:   "", // unknown — host port not learned
				operStatus:   iface.OperStatus,
				speed:        iface.Speed,
			}
			if iface.MTU > 0 {
				li.mtu = fmt.Sprintf("%d", iface.MTU)
			}
			b.links = append(b.links, li)
			linkedPorts[portKey] = true
		}
	}
}
func (b *buildState) enrichHosts(cr *collector.CollectionResult) {
	// Build a temporary v1 topology with just the devices we've found so far,
	// then call the existing enrichment logic.
	tempTopo := &topology.Topology{}
	for _, entry := range b.deviceMap {
		tempTopo.Devices = append(tempTopo.Devices, entry.device)
	}

	var inputs []transform.HostEnrichmentInput
	for _, sw := range cr.Switches {
		if len(sw.MACEntries) > 0 || len(sw.ARPEntries) > 0 {
			inputs = append(inputs, transform.HostEnrichmentInput{
				SwitchID:   sw.SwitchID,
				Neighbors:  sw.Neighbors,
				MACEntries: sw.MACEntries,
				ARPEntries: sw.ARPEntries,
			})
		}
	}
	if len(inputs) > 0 {
		transform.EnrichDevicesFromSwitchData(tempTopo, inputs, transform.HostEnrichmentConfig{
			ReverseDNS: false, // builder is pure — no DNS calls
		})
	}

	// Sync enriched data back into our device map
	for _, d := range tempTopo.Devices {
		if entry, ok := b.deviceMap[d.ID]; ok {
			entry.device = d
			if d.Type == "host" {
				entry.isSwitch = false
			}
		}
	}
}

// promoteUnknownOnPhysicalPorts reclassifies "unknown" devices as "host" when
// they are connected to a switch on a physical port (Ethernet/Eth). In datacenter
// environments, physical switch ports connect to hosts, BMCs, or other switches.
// If a device is NOT identifiable as a switch/BMC and is on a physical port, it
// is almost certainly a host. This handles cases where LLDP capabilities are
// absent and no MAC/ARP data is available for enrichment.
func (b *buildState) promoteUnknownOnPhysicalPorts() {
	// Collect all known switch IDs
	switchIDs := make(map[string]bool)
	for id, entry := range b.deviceMap {
		if entry.isSwitch || entry.device.Type == "switch" {
			switchIDs[id] = true
		}
	}

	// Build map: remote device ID → local ports connecting it to switches
	type portInfo struct {
		localPort string
	}
	devicePorts := make(map[string][]portInfo)
	for _, li := range b.links {
		if switchIDs[li.localDevice] && !switchIDs[li.remoteDevice] {
			devicePorts[li.remoteDevice] = append(devicePorts[li.remoteDevice], portInfo{
				localPort: li.localPort,
			})
		}
	}

	promoted := 0
	for id, entry := range b.deviceMap {
		if entry.device.Type != "unknown" {
			continue
		}
		ports, hasPorts := devicePorts[id]
		if !hasPorts {
			continue
		}

		// Check if at least one connection is on a physical port
		onPhysical := false
		for _, p := range ports {
			if isPhysicalPort(p.localPort) {
				onPhysical = true
				break
			}
		}
		if !onPhysical {
			continue
		}

		// Skip if it looks like a switch (by description/name)
		if looksLikeSwitchDevice(entry.device) {
			continue
		}

		entry.device.Type = "host"
		if entry.device.Annotations == nil {
			entry.device.Annotations = make(map[string]string)
		}
		entry.device.Annotations["classification_source"] = "physical_port_heuristic"
		promoted++
	}

	if promoted > 0 {
		log.Printf("[host-enrichment] Promoted %d unknown devices to host (physical port heuristic)", promoted)
	}
}

// isPhysicalPort returns true if the port name indicates a physical interface
// (Ethernet, Eth, HundredGigE, etc.) as opposed to virtual interfaces.
func isPhysicalPort(portName string) bool {
	lower := strings.ToLower(portName)
	// Physical port prefixes used by NX-OS and other platforms
	physicalPrefixes := []string{
		"ethernet", "eth", "hundredgige", "fortygige", "tengige",
		"gigabitethernet", "ge-", "xe-", "et-",
	}
	for _, pfx := range physicalPrefixes {
		if strings.HasPrefix(lower, pfx) {
			return true
		}
	}
	return false
}

// containsWholeWord checks if s contains keyword as a whole word — not as a
// prefix/suffix of a longer word. A word boundary is any non-alphanumeric
// character or start/end of string. Examples:
//   - containsWholeWord("to-switch-1", "switch") → true
//   - containsWholeWord("Switched-Compute", "switch") → false ("switched" ≠ "switch")
//   - containsWholeWord("fw-external", "fw") → true
func containsWholeWord(s, word string) bool {
	idx := 0
	for {
		pos := strings.Index(s[idx:], word)
		if pos < 0 {
			return false
		}
		absPos := idx + pos
		endPos := absPos + len(word)

		// Check left boundary: start of string or non-alphanumeric
		leftOK := absPos == 0 || !isAlphaNum(s[absPos-1])
		// Check right boundary: end of string or non-alphanumeric
		rightOK := endPos >= len(s) || !isAlphaNum(s[endPos])

		if leftOK && rightOK {
			return true
		}
		idx = absPos + 1
		if idx >= len(s) {
			return false
		}
	}
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// looksLikeSwitchDevice checks if a device has switch-like characteristics.
func looksLikeSwitchDevice(dev topology.Device) bool {
	desc := strings.ToLower(dev.SystemDescription)
	name := strings.ToLower(dev.SystemName)
	for _, hint := range []string{"sonic", "nx-os", "arista", "cumulus", "ftos", "cisco", "dell emc", "junos"} {
		if strings.Contains(desc, hint) || strings.Contains(name, hint) {
			return true
		}
	}
	// Also check for switch/router/spine/leaf in name
	for _, hint := range []string{"spine", "leaf", "switch", "router", "fw-", "firewall"} {
		if strings.Contains(name, hint) {
			return true
		}
	}
	return false
}

// mergeDualHomedHosts identifies "unknown" devices that are the second NIC of
// a dual-homed host (common in Azure Local). In these environments, each server
// has two NICs — one per TOR — with adjacent MAC addresses (differ by ±1 or ±2).
//
// To reduce false positives, merging requires TWO confirming signals:
//   - MAC adjacency: chassis-IDs within ±2 of each other
//   - Port-position match: both connected on the same-numbered port on different switches
//
// If port-position cannot be confirmed (e.g., port names are non-numeric), MAC
// adjacency alone is accepted as a fallback.
func (b *buildState) mergeDualHomedHosts() {
	// Build index: chassis-ID (MAC) → device ID for all hosts
	type hostInfo struct {
		id  string
		mac uint64
	}
	hosts := make([]hostInfo, 0)
	for _, entry := range b.deviceMap {
		if entry.device.Type != "host" {
			continue
		}
		mac := parseMACToUint64(entry.device.ChassisID)
		if mac == 0 {
			continue
		}
		hosts = append(hosts, hostInfo{id: entry.device.ID, mac: mac})
	}

	if len(hosts) == 0 {
		return
	}

	// Build device → port connections index for port-position confirmation
	deviceConnections := make(map[string][]connectionInfo)
	for _, li := range b.links {
		deviceConnections[li.remoteDevice] = append(deviceConnections[li.remoteDevice], connectionInfo{
			switchID:  li.localDevice,
			localPort: li.localPort,
		})
	}

	// For each unknown device, try to match to a host via MAC adjacency + port confirmation
	merged := 0
	for id, entry := range b.deviceMap {
		if entry.device.Type != "unknown" {
			continue
		}
		unknownMAC := parseMACToUint64(entry.device.ChassisID)
		if unknownMAC == 0 {
			continue
		}

		// Find a host with MAC within ±2
		var matchedHostID string
		for _, h := range hosts {
			diff := int64(unknownMAC) - int64(h.mac)
			if diff < 0 {
				diff = -diff
			}
			if diff >= 1 && diff <= 2 {
				matchedHostID = h.id
				break
			}
		}
		if matchedHostID == "" {
			continue
		}

		// Confirm via port-position: the unknown and host should be on the
		// same-numbered port on different switches.
		if !confirmDualHomedByPort(deviceConnections[matchedHostID], deviceConnections[id]) {
			// Port confirmation failed — only merge if we can't extract port numbers
			// at all (i.e., non-standard naming where confirmation is impossible).
			hostPorts := deviceConnections[matchedHostID]
			unknownPorts := deviceConnections[id]
			if hasExtractablePortNumber(hostPorts) && hasExtractablePortNumber(unknownPorts) {
				// Both have parseable port numbers but they don't match — skip merge
				continue
			}
			// Cannot extract port numbers from either side — allow MAC-only merge as fallback
		}

		// Merge: transfer the unknown's links to the matched host
		hostEntry := b.deviceMap[matchedHostID]
		for i := range b.links {
			if b.links[i].remoteDevice == id {
				b.links[i].remoteDevice = matchedHostID
			}
		}

		// If the host has no management address but the unknown has one, adopt it
		if hostEntry.device.ManagementAddress == "" && entry.device.ManagementAddress != "" {
			hostEntry.device.ManagementAddress = entry.device.ManagementAddress
		}

		// Remove the unknown device from the map
		delete(b.deviceMap, id)
		merged++
	}

	if merged > 0 {
		log.Printf("[host-enrichment] Dual-homed merge: %d unknown devices merged into hosts", merged)
	}
}

// confirmDualHomedByPort checks if two sets of connections share the same port
// number on different switches — the expected pattern for dual-homed hosts.
func confirmDualHomedByPort(hostConns, unknownConns []connectionInfo) bool {
	for _, hc := range hostConns {
		hNum := extractPortNumber(hc.localPort)
		if hNum == "" {
			continue
		}
		for _, uc := range unknownConns {
			// Must be on different switches
			if uc.switchID == hc.switchID {
				continue
			}
			uNum := extractPortNumber(uc.localPort)
			if uNum == hNum {
				return true
			}
		}
	}
	return false
}

// hasExtractablePortNumber returns true if any connection has a parseable port number.
func hasExtractablePortNumber(conns []connectionInfo) bool {
	for _, c := range conns {
		if extractPortNumber(c.localPort) != "" {
			return true
		}
	}
	return false
}

// extractPortNumber extracts the numeric port identifier from interface names
// like "Eth1/15", "Ethernet1/15", "GigabitEthernet0/0/1". Returns the last
// numeric segment (the port number within a slot/module).
func extractPortNumber(portName string) string {
	// Find the last '/' and return everything after it
	lastSlash := strings.LastIndex(portName, "/")
	if lastSlash >= 0 && lastSlash < len(portName)-1 {
		suffix := portName[lastSlash+1:]
		// Verify it's numeric
		for _, c := range suffix {
			if c < '0' || c > '9' {
				return ""
			}
		}
		return suffix
	}
	return ""
}

// parseMACToUint64 converts a colon-separated MAC address to a 48-bit integer.
func parseMACToUint64(mac string) uint64 {
	mac = strings.ToLower(strings.TrimSpace(mac))
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		// Try dot-separated (xxxx.xxxx.xxxx)
		parts = strings.Split(mac, ".")
		if len(parts) == 3 {
			// Convert to 6-octet form
			full := strings.Join(parts, "")
			if len(full) == 12 {
				parts = make([]string, 6)
				for i := 0; i < 6; i++ {
					parts[i] = full[i*2 : i*2+2]
				}
			} else {
				return 0
			}
		} else {
			return 0
		}
	}
	var result uint64
	for _, p := range parts {
		b, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return 0
		}
		result = (result << 8) | b
	}
	return result
}

// correlateEndpoints discovers VM endpoints from MAC/ARP data.
func (b *buildState) correlateEndpoints(cr *collector.CollectionResult) {
	var inputs []transform.CorrelationInput
	for _, sw := range cr.Switches {
		if len(sw.MACEntries) > 0 {
			inputs = append(inputs, transform.CorrelationInput{
				SwitchID:   sw.SwitchID,
				Neighbors:  sw.Neighbors,
				MACEntries: sw.MACEntries,
				ARPEntries: sw.ARPEntries,
				NVEPeers:   sw.NVEPeers,
				L2RIBMacs:  sw.L2RIBMacs,
			})
		}
	}
	if len(inputs) > 0 {
		b.endpoints = transform.CorrelateEndpoints(inputs)
	}
}

// buildVLANs enriches interfaces with observed VLANs from MAC data and
// assigns VLAN memberships to devices.
func (b *buildState) buildVLANs(cr *collector.CollectionResult) {
	// Enrich interfaces with observed VLANs
	type portVLANs map[string]map[int]bool
	switchPortVLANs := make(map[string]portVLANs)

	for _, sw := range cr.Switches {
		for _, mac := range sw.MACEntries {
			if mac.Port == "" || mac.VLAN == 0 {
				continue
			}
			port := transform.NormalizeInterfaceName(mac.Port)
			if switchPortVLANs[sw.SwitchID] == nil {
				switchPortVLANs[sw.SwitchID] = make(portVLANs)
			}
			if switchPortVLANs[sw.SwitchID][port] == nil {
				switchPortVLANs[sw.SwitchID][port] = make(map[int]bool)
			}
			switchPortVLANs[sw.SwitchID][port][mac.VLAN] = true
		}
	}

	for id, pvs := range switchPortVLANs {
		entry, ok := b.deviceMap[id]
		if !ok {
			continue
		}
		for j := range entry.device.Interfaces {
			vlanSet, ok := pvs[entry.device.Interfaces[j].Name]
			if !ok {
				continue
			}
			var vlans []int
			for v := range vlanSet {
				vlans = append(vlans, v)
			}
			sort.Ints(vlans)
			entry.device.Interfaces[j].ObservedVLANs = vlans
		}
	}

	// Assign VLAN memberships to devices based on endpoint data
	deviceVLANs := make(map[string]map[int]bool)
	for _, ep := range b.endpoints {
		if ep.HostDevice != "" {
			if deviceVLANs[ep.HostDevice] == nil {
				deviceVLANs[ep.HostDevice] = make(map[int]bool)
			}
			for _, vid := range ep.VLANs {
				deviceVLANs[ep.HostDevice][vid] = true
			}
		}
	}
	for id, vlans := range deviceVLANs {
		if entry, ok := b.deviceMap[id]; ok {
			for vid := range vlans {
				entry.device.VLANs = append(entry.device.VLANs, vid)
			}
		}
	}
}

// assemble creates the final TopologyV2 from the build state.
func (b *buildState) assemble() *topology.TopologyV2 {
	v2 := &topology.TopologyV2{
		SchemaVersion: "2.0",
		Metadata: topology.Metadata{
			CollectedAt: b.cr.CollectedAt,
			Tool:        "network-mapper",
			ToolVersion: ToolVersion,
		},
	}

	// Collect source switches
	for _, sw := range b.cr.Switches {
		v2.Metadata.SourceSwitches = append(v2.Metadata.SourceSwitches, sw.SwitchID)
	}

	// Classify devices
	switchIDs := make(map[string]bool)
	for id, entry := range b.deviceMap {
		if entry.isSwitch || entry.device.Type == "switch" {
			switchIDs[id] = true
		}
	}

	// Build fabric switches
	for _, entry := range b.deviceMap {
		if !switchIDs[entry.device.ID] {
			continue
		}
		sw := deviceToFabricSwitch(entry.device)
		v2.Fabric.Switches = append(v2.Fabric.Switches, sw)
	}

	// Enrich fabric switches with QoS/PFC data from collection results
	for i := range v2.Fabric.Switches {
		fsw := &v2.Fabric.Switches[i]
		for _, crSw := range b.cr.Switches {
			if crSw.SwitchID != fsw.ID {
				continue
			}
			// Wire QoS stats
			for _, qs := range crSw.QoSStats {
				for _, q := range qs.Queues {
					fsw.QoSStats = append(fsw.QoSStats, topology.QoSStatEntry{
						InterfaceName:     qs.InterfaceName,
						QueueName:         q.QueueName,
						Direction:         q.Direction,
							TxBytes:           q.TxBytes,
							TxPackets:         q.TxPackets,
							PFCPauseFramesTx:  q.PFCPauseFramesTx,
							PFCPauseFramesRx:  q.PFCPauseFramesRx,
							PFCWatchdogDrops:  q.PFCWatchdogDrops,
							ECNMarkedPackets:  q.ECNMarkedPackets,
							DropPackets:       q.DropPackets,
							CurrentQueueDepth: q.CurrentQueueDepth,
							MaxQueueDepth:     q.MaxQueueDepth,
						})
					}
				}
			// Wire PFC config
			for _, pfc := range crSw.PFCConfig {
				fsw.PFCConfig = append(fsw.PFCConfig, topology.PFCConfigEntry{
					InterfaceName: pfc.InterfaceName,
					Mode:          pfc.Mode,
					SendTLV:       pfc.SendTLV,
					LosslessCos:   pfc.LosslessCos,
				})
			}
			break
		}
	}

	// Build links: peer_links and connected_hosts
	for _, li := range b.links {
		localIsSwitch := switchIDs[li.localDevice]
		remoteIsSwitch := switchIDs[li.remoteDevice]

		if localIsSwitch && remoteIsSwitch {
			addPeerLinkFromInfo(v2, li)
		} else if localIsSwitch {
			addConnectedHostFromInfo(v2, li)
		}
	}

	// Build compute hosts
	for _, entry := range b.deviceMap {
		if switchIDs[entry.device.ID] || entry.device.Type == "switch" {
			continue
		}
		if entry.device.Type == "host" {
			host := deviceToComputeHost(entry.device)
			// Add connections from links
			for _, li := range b.links {
				if li.remoteDevice == host.ID {
					host.Connections = append(host.Connections, topology.HostConnection{
						SwitchName: li.localDevice,
						SwitchID:   li.localDevice,
						SwitchPort: li.localPort,
						OperStatus: li.operStatus,
						Speed:      li.speed,
						MTU:        li.mtu,
					})
				}
			}
			// Enrich connections with VLAN config from switch interfaces
			for i, conn := range host.Connections {
				if swEntry, ok := b.deviceMap[conn.SwitchID]; ok {
					for _, iface := range swEntry.device.Interfaces {
						if iface.Name == conn.SwitchPort {
							host.Connections[i].VLANMode = iface.Mode
							host.Connections[i].AccessVLAN = iface.AccessVLAN
							host.Connections[i].NativeVLAN = iface.NativeVLAN
							host.Connections[i].TrunkVLANs = iface.TrunkVLANs
							break
						}
					}
				}
			}
			v2.Compute.Hosts = append(v2.Compute.Hosts, host)
		} else {
				// Non-host, non-switch device (BMC, unknown, etc.)
			ud := topology.UnknownDevice{
				ID:                entry.device.ID,
					DeviceType:        entry.device.Type,
					ChassisID:         entry.device.ChassisID,
					ManagementAddress: entry.device.ManagementAddress,
					SystemDescription: entry.device.SystemDescription,
				}
			for _, li := range b.links {
				if li.remoteDevice == entry.device.ID {
					ud.ConnectedTo = append(ud.ConnectedTo, topology.DeviceAttachment{
						Switch:     li.localDevice,
						Port:       li.localPort,
						OperStatus: li.operStatus,
						MTU:        li.mtu,
					})
				}
			}
			if v2.UnknownDevices == nil {
				v2.UnknownDevices = &topology.UnknownDeviceSet{}
			}
			v2.UnknownDevices.Items = append(v2.UnknownDevices.Items, ud)
		}
	}

	// Distribute endpoints to hosts, VTEP groups, or unattributed
	// Build host mgmt IP lookup for VTEP→host resolution
	hostMgmtIPToID := make(map[string]string)
	for _, host := range v2.Compute.Hosts {
		if host.ManagementAddress != "" {
			hostMgmtIPToID[host.ManagementAddress] = host.ID
		}
	}

	// Collect all NVE peer data for VTEP MAC lookup
	nvePeerMAC := make(map[string]string) // VTEP IP → VTEP MAC
	for _, sw := range b.cr.Switches {
		for _, peer := range sw.NVEPeers {
			if peer.PeerIP != "" && peer.PeerMAC != "" {
				nvePeerMAC[peer.PeerIP] = peer.PeerMAC
			}
		}
	}

	vtepGroupMap := make(map[string]*topology.VTEPGroup) // VTEP IP → group

	for _, ep := range b.endpoints {
		he := topology.HostEndpoint{
			MAC:             ep.MAC,
			IPs:             ep.IPs,
			VLANs:           ep.VLANs,
			Type:            ep.Type,
			LearnedOnSwitch: ep.SwitchID,
			LearnedOnPort:   ep.HostPort,
		}
		attributed := false

		// Try port-based attribution first
		if ep.HostDevice != "" {
			for i := range v2.Compute.Hosts {
				if v2.Compute.Hosts[i].ID == ep.HostDevice {
					v2.Compute.Hosts[i].Endpoints = append(v2.Compute.Hosts[i].Endpoints, he)
					attributed = true
					break
				}
			}
		}

		// Try VTEP-based grouping for NVE-learned MACs
		if !attributed && ep.VTEPIP != "" {
			group, exists := vtepGroupMap[ep.VTEPIP]
			if !exists {
				group = &topology.VTEPGroup{
					VTEPIP:           ep.VTEPIP,
					VTEPMAC:          nvePeerMAC[ep.VTEPIP],
					ResolutionSource: "unresolved",
				}
				// Conservative VTEP→host resolution: only if VTEP IP matches
				// an LLDP host's management address (deterministic match)
				if hostID, ok := hostMgmtIPToID[ep.VTEPIP]; ok {
					group.HostID = hostID
					group.ResolutionSource = "lldp-mgmt-ip"
				}
				vtepGroupMap[ep.VTEPIP] = group
			}
			group.Endpoints = append(group.Endpoints, he)
			attributed = true
		}

		if !attributed {
			if v2.Compute.UnattributedEndpoints == nil {
				v2.Compute.UnattributedEndpoints = &topology.UnattributedEndpointSet{}
			}
			v2.Compute.UnattributedEndpoints.Items = append(v2.Compute.UnattributedEndpoints.Items, he)
		}
	}

	// Finalize VTEP groups
	for _, group := range vtepGroupMap {
		group.EndpointCount = len(group.Endpoints)
		v2.Compute.VTEPGroups = append(v2.Compute.VTEPGroups, *group)
	}

	if v2.Compute.UnattributedEndpoints != nil {
		v2.Compute.UnattributedEndpoints.Count = len(v2.Compute.UnattributedEndpoints.Items)
	}

	// Build VLANs cross-reference
	b.assembleVLANs(v2)

	// Collect warnings from all switches
	for _, sw := range b.cr.Switches {
		v2.Warnings = append(v2.Warnings, sw.Errors...)
	}

	computeSummary(v2)
	return v2
}

// assembleVLANs builds the network-wide VLAN view.
func (b *buildState) assembleVLANs(v2 *topology.TopologyV2) {
	// Collect all VLANs from switch data
	vlanMap := make(map[int]*topology.VLANEntry)

	for _, sw := range b.cr.Switches {
		for _, vlan := range sw.VLANs {
			entry, ok := vlanMap[vlan.ID]
			if !ok {
				entry = &topology.VLANEntry{ID: vlan.ID}
				vlanMap[vlan.ID] = entry
			}

			vs := topology.VLANSwitch{SwitchName: sw.SwitchID}
			// Use MemberPorts from v1 VLAN + classify by interface mode from switch data
			for _, portName := range vlan.MemberPorts {
				mode := getPortMode(sw, portName)
				if mode == "trunk" {
					vs.TrunkPorts = append(vs.TrunkPorts, portName)
				} else {
					vs.AccessPorts = append(vs.AccessPorts, portName)
				}
			}
			sort.Strings(vs.AccessPorts)
			sort.Strings(vs.TrunkPorts)
			entry.Switches = append(entry.Switches, vs)
		}
	}

	// Add host VLAN membership info from port assignments (access, native, and trunk VLANs)
	for _, host := range v2.Compute.Hosts {
		for _, conn := range host.Connections {
			addedVLANs := make(map[int]bool)
			hostRef := topology.VLANHost{
				ChassisID:    host.ChassisID,
				ManagementIP: host.ManagementAddress,
				SwitchPort:   conn.SwitchPort,
			}

			addHostToVLAN := func(vid int) {
				if vid <= 0 || addedVLANs[vid] {
					return
				}
				addedVLANs[vid] = true
				if entry, ok := vlanMap[vid]; ok {
					entry.Hosts = append(entry.Hosts, hostRef)
				}
			}

			addHostToVLAN(conn.AccessVLAN)
			addHostToVLAN(conn.NativeVLAN)
			for _, vid := range conn.TrunkVLANs {
				addHostToVLAN(vid)
			}
		}
	}

	for _, entry := range vlanMap {
		v2.VLANs.Items = append(v2.VLANs.Items, *entry)
	}
	sort.Slice(v2.VLANs.Items, func(i, j int) bool {
		return v2.VLANs.Items[i].ID < v2.VLANs.Items[j].ID
	})
}

// --- helper functions ---

func resolveDeviceID(id string, systemNameToID map[string]string) string {
	if mapped, ok := systemNameToID[id]; ok {
		return mapped
	}
	return id
}

// getPortMode looks up the interface mode for a port on a switch.
func getPortMode(sw collector.SwitchData, portName string) string {
	for _, iface := range sw.Interfaces {
		if iface.Name == portName {
			return iface.Mode
		}
	}
	return ""
}

func deviceToFabricSwitch(d topology.Device) topology.FabricSwitch {
	sw := topology.FabricSwitch{
		ID:                d.ID,
		Name:              d.SystemName,
		ChassisID:         d.ChassisID,
		ManagementAddress: d.ManagementAddress,
		SoftwareVersion:   d.SoftwareVersion,
		SystemDescription: d.SystemDescription,
		Uptime:            d.Uptime,
		Interfaces:        stripCounters(d.Interfaces),
		BGPSessions:       d.BGPSessions,
		Annotations:       d.Annotations,
	}
	if sw.Name == "" {
		sw.Name = d.ID
	}
	if d.CPUUtilization > 0 || d.MemoryUsed > 0 {
		sw.Health = &topology.SwitchHealth{
			CPUUtilizationPct: d.CPUUtilization,
			MemoryUsedBytes:   d.MemoryUsed,
			MemoryTotalBytes:  d.MemoryTotal,
		}
	}
	return sw
}

func deviceToComputeHost(d topology.Device) topology.ComputeHost {
	return topology.ComputeHost{
		ID:                   d.ID,
		ChassisID:            d.ChassisID,
		Name:                 d.SystemName,
		ManagementAddress:    d.ManagementAddress,
		ClassificationSource: d.Type,
		Annotations:          d.Annotations,
	}
}

func addPeerLinkFromInfo(v2 *topology.TopologyV2, li linkInfo) {
	for i := range v2.Fabric.Switches {
		if v2.Fabric.Switches[i].ID == li.localDevice {
			v2.Fabric.Switches[i].PeerLinks = append(v2.Fabric.Switches[i].PeerLinks, topology.PeerLink{
				LocalPort:    li.localPort,
				RemoteSwitch: li.remoteDevice,
				RemotePort:   li.remotePort,
				OperStatus:   li.operStatus,
				Speed:        li.speed,
				MTU:          li.mtu,
			})
			return
		}
	}
}

func addConnectedHostFromInfo(v2 *topology.TopologyV2, li linkInfo) {
	for i := range v2.Fabric.Switches {
		if v2.Fabric.Switches[i].ID == li.localDevice {
			v2.Fabric.Switches[i].ConnectedHosts = append(v2.Fabric.Switches[i].ConnectedHosts, topology.ConnectedHost{
				Port:       li.localPort,
				HostID:     li.remoteDevice,
				OperStatus: li.operStatus,
				MTU:        li.mtu,
			})
			return
		}
	}
}

// stripCounters returns a copy of the interface slice with Counters cleared.
func stripCounters(ifaces []topology.Interface) []topology.Interface {
	out := make([]topology.Interface, len(ifaces))
	copy(out, ifaces)
	for i := range out {
		out[i].Counters = nil
	}
	return out
}

func addPeerLink(v2 *topology.TopologyV2, link topology.Link) {
	for i := range v2.Fabric.Switches {
		if v2.Fabric.Switches[i].ID == link.LocalDevice {
			v2.Fabric.Switches[i].PeerLinks = append(v2.Fabric.Switches[i].PeerLinks, topology.PeerLink{
				LocalPort:    link.LocalPort,
				RemoteSwitch: link.RemoteDevice,
				RemotePort:   link.RemotePort,
				OperStatus:   link.OperStatus,
				Speed:        link.Speed,
				MTU:          link.MTU,
			})
			return
		}
	}
}

func addConnectedHost(v2 *topology.TopologyV2, link topology.Link) {
	for i := range v2.Fabric.Switches {
		if v2.Fabric.Switches[i].ID == link.LocalDevice {
			v2.Fabric.Switches[i].ConnectedHosts = append(v2.Fabric.Switches[i].ConnectedHosts, topology.ConnectedHost{
				Port:       link.LocalPort,
				HostID:     link.RemoteDevice,
				OperStatus: link.OperStatus,
				MTU:        link.MTU,
			})
			return
		}
	}
}

func addHostConnection(v2 *topology.TopologyV2, link topology.Link, switchIDs map[string]bool) {
	hostID := link.RemoteDevice
	if switchIDs[hostID] {
		return
	}
	for i := range v2.Compute.Hosts {
		if v2.Compute.Hosts[i].ID == hostID {
			v2.Compute.Hosts[i].Connections = append(v2.Compute.Hosts[i].Connections, topology.HostConnection{
				SwitchName: link.LocalDevice,
				SwitchID:   link.LocalDevice,
				SwitchPort: link.LocalPort,
				OperStatus: link.OperStatus,
				Speed:      link.Speed,
				MTU:        link.MTU,
			})
			return
		}
	}
}

func addUnknownAttachment(v2 *topology.TopologyV2, link topology.Link, switchIDs map[string]bool) {
	if v2.UnknownDevices == nil {
		return
	}
	targetID := link.RemoteDevice
	if switchIDs[targetID] {
		return
	}
	for i := range v2.UnknownDevices.Items {
		if v2.UnknownDevices.Items[i].ID == targetID {
			v2.UnknownDevices.Items[i].ConnectedTo = append(v2.UnknownDevices.Items[i].ConnectedTo, topology.DeviceAttachment{
				Switch:     link.LocalDevice,
				Port:       link.LocalPort,
				OperStatus: link.OperStatus,
				MTU:        link.MTU,
			})
			return
		}
	}
}

func bgpNeighborsToSessions(neighbors []transform.BGPNeighbor) []topology.BGPSession {
	if len(neighbors) == 0 {
		return nil
	}
	sessions := make([]topology.BGPSession, len(neighbors))
	for i, n := range neighbors {
		sessions[i] = topology.BGPSession{
			NeighborAddress:        n.NeighborAddress,
			PeerAS:                 n.PeerAS,
			LocalAS:                n.LocalAS,
			PeerType:               n.PeerType,
			Description:            n.Description,
			SessionState:           n.SessionState,
			Enabled:                n.Enabled,
			EstablishedTransitions: n.EstablishedTransitions,
			LastEstablished:        n.LastEstablished,
			VRFName:                n.VRFName,
			MessagesReceived:       n.MessagesReceived,
			MessagesSent:           n.MessagesSent,
			PrefixesReceived:       n.PrefixesReceived,
			PrefixesSent:           n.PrefixesSent,
		}
	}
	return sessions
}

func computeSummary(v2 *topology.TopologyV2) {
	s := &v2.Metadata.Summary
	s.SwitchCount = len(v2.Fabric.Switches)
	s.HostCount = len(v2.Compute.Hosts)
	s.VLANCount = len(v2.VLANs.Items)
	s.PartialFailures = len(v2.Warnings)

	if v2.UnknownDevices != nil {
		s.UnknownDeviceCount = len(v2.UnknownDevices.Items)
	}

	// Count links
	for _, sw := range v2.Fabric.Switches {
		s.InterSwitchLinks += len(sw.PeerLinks)
		s.HostLinks += len(sw.ConnectedHosts)
	}
	s.TotalLinks = s.InterSwitchLinks + s.HostLinks

	// Count endpoints
	for _, h := range v2.Compute.Hosts {
		s.AttributedEndpoints += len(h.Endpoints)
	}
	// VTEP-grouped endpoints count as attributed (they have a known VTEP owner)
	for _, g := range v2.Compute.VTEPGroups {
		s.AttributedEndpoints += g.EndpointCount
	}
	if v2.Compute.UnattributedEndpoints != nil {
		s.UnattributedEndpoints = v2.Compute.UnattributedEndpoints.Count
	}
	s.EndpointCount = s.AttributedEndpoints + s.UnattributedEndpoints
}

// unused but reserved for future enrichment
var _ = time.Now
