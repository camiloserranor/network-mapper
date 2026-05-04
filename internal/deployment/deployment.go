// Package deployment loads Azure Local deployment design JSON files and
// enriches discovered topology with authoritative infrastructure metadata.
//
// The deployment JSON is an optional input — when provided, it complements
// LLDP-discovered data with hostnames, MAC addresses, IP addresses, and
// device type information from the customer's deployment plan.
package deployment

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

// deploymentFile is the top-level structure of an Azure Local deployment JSON.
type deploymentFile struct {
	ScaleUnits []scaleUnit `json:"ScaleUnits"`
	DevloopData *devloopData `json:"DevloopData"`
}

type scaleUnit struct {
	DeploymentData deploymentData `json:"DeploymentData"`
}

type deploymentData struct {
	PhysicalNodes   []physicalNode   `json:"physicalNodes"`
	PhysicalNodesV2 []physicalNodeV2 `json:"PhysicalNodesV2"`
	NamingPrefix    string           `json:"namingPrefix"`
	DomainFQDN      string           `json:"domainFqdn"`
}

// physicalNode is the simple node entry (legacy format).
type physicalNode struct {
	Name       string `json:"name"`
	IPv4Address string `json:"ipv4Address"`
	BMCIPAddress string `json:"BMCIPAddress"`
	MACAddress  string `json:"MACAddress"`
}

// physicalNodeV2 is the richer node entry with per-NIC details.
type physicalNodeV2 struct {
	Name              string           `json:"Name"`
	IPv4Address       string           `json:"IPv4Address"`
	BMCIPAddress      string           `json:"BMCIPAddress"`
	MACAddressForPXE  string           `json:"MACAddressForPXE"`
	NetworkAdapters   []networkAdapter `json:"NetworkAdapters"`
}

// networkAdapter represents a NIC on a PhysicalNodesV2 entry.
type networkAdapter struct {
	MACAddress string `json:"MACAddress"`
	TargetName string `json:"TargetName"`
}

type devloopData struct {
	HLH   *hlhEntry `json:"HLH"`
	Model string    `json:"Model"`
}

type hlhEntry struct {
	IPv4Address string `json:"IPv4Address"`
	Name        string `json:"Name"`
}

// DeploymentData is the normalized representation used by enrichment logic.
type DeploymentData struct {
	HostNodes []HostNode
}

// HostNode represents a physical server extracted from the deployment JSON.
type HostNode struct {
	Name         string
	IPv4Address  string
	BMCIPAddress string
	NICs         []NIC
}

// NIC represents a network interface on a host node.
type NIC struct {
	Name       string
	MacAddress string
}

// Load reads and parses a deployment design JSON file.
// It supports the Azure Local deployment JSON format with ScaleUnits wrapper
// and normalizes both physicalNodes and PhysicalNodesV2 entries into HostNodes.
func Load(path string) (*DeploymentData, error) {
	log.Printf("[deployment] Loading deployment data from %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading deployment JSON: %w", err)
	}

	var raw deploymentFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing deployment JSON: %w", err)
	}

	if len(raw.ScaleUnits) == 0 {
		return nil, fmt.Errorf("deployment JSON has no ScaleUnits — unsupported schema")
	}

	dd := &DeploymentData{}
	depData := raw.ScaleUnits[0].DeploymentData

	// Prefer PhysicalNodesV2 (has all NIC MACs) over physicalNodes (single MAC)
	if len(depData.PhysicalNodesV2) > 0 {
		for _, pn := range depData.PhysicalNodesV2 {
			host := HostNode{
				Name:         pn.Name,
				IPv4Address:  pn.IPv4Address,
				BMCIPAddress: pn.BMCIPAddress,
			}
			// Add PXE MAC as first NIC if not already in adapters
			pxeMAC := normalizeMAC(pn.MACAddressForPXE)
			pxeFound := false
			for _, na := range pn.NetworkAdapters {
				host.NICs = append(host.NICs, NIC{
					Name:       na.TargetName,
					MacAddress: na.MACAddress,
				})
				if normalizeMAC(na.MACAddress) == pxeMAC {
					pxeFound = true
				}
			}
			if !pxeFound && pn.MACAddressForPXE != "" {
				host.NICs = append([]NIC{{Name: "pxe", MacAddress: pn.MACAddressForPXE}}, host.NICs...)
			}
			dd.HostNodes = append(dd.HostNodes, host)
		}
	} else if len(depData.PhysicalNodes) > 0 {
		// Fallback: physicalNodes only has one MAC per node
		for _, pn := range depData.PhysicalNodes {
			host := HostNode{
				Name:         pn.Name,
				IPv4Address:  pn.IPv4Address,
				BMCIPAddress: pn.BMCIPAddress,
			}
			if pn.MACAddress != "" {
				host.NICs = append(host.NICs, NIC{
					Name:       "ethernet",
					MacAddress: pn.MACAddress,
				})
			}
			dd.HostNodes = append(dd.HostNodes, host)
		}
	}

	if len(dd.HostNodes) == 0 {
		return nil, fmt.Errorf("deployment JSON has no physical nodes — unsupported schema or empty file")
	}

	totalNICs := 0
	for _, h := range dd.HostNodes {
		totalNICs += len(h.NICs)
	}
	log.Printf("[deployment] Loaded: %d host nodes (%d NICs total)",
		len(dd.HostNodes), totalNICs)

	return dd, nil
}

// EnrichTopology enriches a discovered topology with deployment data.
// It matches devices by MAC address (with +2 offset fallback for NX-OS LLDP),
// groups multiple NIC ports belonging to the same host into a single device,
// and synthesizes devices that were expected but not discovered via LLDP.
func EnrichTopology(topo *topology.Topology, dd *DeploymentData) {
	log.Printf("[deployment] Enriching topology with deployment data")

	// Build lookup maps from deployment data
	exactMACToHost := make(map[string]*HostNode)   // exact normalized NIC MAC → host
	offsetMACToHost := make(map[string]*HostNode)   // MAC+2 (LLDP chassis offset) → host
	hostNameToHost := make(map[string]*HostNode)    // lowercase hostname → host

	for i := range dd.HostNodes {
		h := &dd.HostNodes[i]
		hostNameToHost[strings.ToLower(h.Name)] = h

		for j := range h.NICs {
			nic := &h.NICs[j]
			if nic.MacAddress == "" {
				continue
			}
			mac := normalizeMAC(nic.MacAddress)
			exactMACToHost[mac] = h

			// NX-OS LLDP reports chassis-id = port MAC + 2.
			// Precompute the offset so we can match LLDP chassis-ids.
			if offsetMAC, ok := macAddOffset(mac, 2); ok {
				// Only add if it doesn't collide with an exact entry
				if _, collision := exactMACToHost[offsetMAC]; !collision {
					offsetMACToHost[offsetMAC] = h
				}
			}
		}
	}

	// Index existing devices by ID for quick lookup
	deviceByID := make(map[string]*topology.Device)
	for i := range topo.Devices {
		deviceByID[topo.Devices[i].ID] = &topo.Devices[i]
	}

	// Track which deployment hosts were matched (for synthesis later)
	matchedHosts := make(map[string]bool)

	var enriched, reclassified int

	// Pass 1: Enrich existing devices via MAC match (exact first, then +2 offset)
	for i := range topo.Devices {
		dev := &topo.Devices[i]
		chassisMAC := normalizeMAC(dev.ChassisID)

		if chassisMAC != "" {
			// Try exact MAC match first
			if host, ok := exactMACToHost[chassisMAC]; ok {
				enrichDeviceFromHost(dev, host, "mac")
				matchedHosts[host.Name] = true
				enriched++
				if dev.Type == "unknown" {
					dev.Type = "host"
					reclassified++
				}
				continue
			}

			// Try +2 offset match (NX-OS LLDP chassis-id = port MAC + 2)
			if host, ok := offsetMACToHost[chassisMAC]; ok {
				enrichDeviceFromHost(dev, host, "mac_offset_2")
				matchedHosts[host.Name] = true
				enriched++
				if dev.Type == "unknown" {
					dev.Type = "host"
					reclassified++
				}
				continue
			}
		}

		// Fallback: hostname match (weaker signal)
		if dev.SystemName != "" {
			if host, ok := hostNameToHost[strings.ToLower(dev.SystemName)]; ok {
				enrichDeviceFromHost(dev, host, "hostname")
				matchedHosts[host.Name] = true
				enriched++
				if dev.Type == "unknown" {
					dev.Type = "host"
					reclassified++
				}
			}
		}
	}

	// Pass 1.5: Merge devices matched to the same deployment host.
	// Only merge devices matched by MAC (exact or offset) — hostname-only
	// matches are too weak to justify collapsing devices.
	merged := mergeDevicesByHost(topo)

	// Pass 2: Rewrite device IDs to deployment hostnames where matched
	idRenames := make(map[string]string)
	for i := range topo.Devices {
		dev := &topo.Devices[i]
		if dev.Annotations == nil {
			continue
		}
		depName, ok := dev.Annotations["deployment_name"]
		if !ok || depName == "" || depName == dev.ID {
			continue
		}
		// Rebuild deviceByID after merge
		if _, collision := findDeviceByID(topo, depName); collision && depName != dev.ID {
			log.Printf("[deployment] Skipping rename %q → %q (ID collision)", dev.ID, depName)
			continue
		}
		idRenames[dev.ID] = depName
		log.Printf("[deployment] Renaming device %q → %q", dev.ID, depName)
	}

	if len(idRenames) > 0 {
		applyRenames(topo, idRenames)
	}

	// Pass 3: Synthesize missing hosts
	deviceByID = make(map[string]*topology.Device)
	for i := range topo.Devices {
		deviceByID[topo.Devices[i].ID] = &topo.Devices[i]
	}

	var synthesized int
	for _, host := range dd.HostNodes {
		if !matchedHosts[host.Name] {
			if _, exists := deviceByID[host.Name]; !exists {
				dev := synthesizeHost(host)
				topo.Devices = append(topo.Devices, dev)
				deviceByID[dev.ID] = &topo.Devices[len(topo.Devices)-1]
				synthesized++
				log.Printf("[deployment] Synthesized missing host: %s", host.Name)
			}
		}
	}

	log.Printf("[deployment] Enrichment complete: %d enriched, %d reclassified, %d merged, %d synthesized",
		enriched, reclassified, merged, synthesized)
}

// mergeDevicesByHost collapses multiple devices that matched the same deployment
// host (via MAC) into a single device node. Returns the number of devices removed.
func mergeDevicesByHost(topo *topology.Topology) int {
	// Group devices by deployment_name, only considering MAC-based matches
	groups := make(map[string][]int) // deployment_name → device indices
	for i := range topo.Devices {
		dev := &topo.Devices[i]
		if dev.Annotations == nil {
			continue
		}
		matchType := dev.Annotations["deployment_match"]
		if matchType != "mac" && matchType != "mac_offset_2" {
			continue
		}
		depName := dev.Annotations["deployment_name"]
		if depName == "" {
			continue
		}
		groups[depName] = append(groups[depName], i)
	}

	// For each group with >1 device, merge into a primary
	removeSet := make(map[int]bool)
	for depName, indices := range groups {
		if len(indices) <= 1 {
			continue
		}

		// Pick primary: most links first, then lexical ID for stability
		primaryIdx := pickPrimary(topo, indices)
		primary := &topo.Devices[primaryIdx]

		log.Printf("[deployment] Merging %d NIC devices into %q (primary: %s)",
			len(indices), depName, primary.ID)

		// Build set of secondary IDs to rewrite
		for _, idx := range indices {
			if idx == primaryIdx {
				continue
			}
			secondary := &topo.Devices[idx]

			// Rewrite all links referencing the secondary
			for j := range topo.Links {
				if topo.Links[j].RemoteDevice == secondary.ID {
					topo.Links[j].RemoteDevice = primary.ID
				}
				if topo.Links[j].LocalDevice == secondary.ID {
					topo.Links[j].LocalDevice = primary.ID
				}
			}

			// Rewrite endpoint references
			for j := range topo.Endpoints {
				if topo.Endpoints[j].HostDevice == secondary.ID {
					topo.Endpoints[j].HostDevice = primary.ID
				}
			}

			removeSet[idx] = true
		}
	}

	if len(removeSet) == 0 {
		return 0
	}

	// Remove merged devices (rebuild slice skipping removed indices)
	newDevices := make([]topology.Device, 0, len(topo.Devices)-len(removeSet))
	for i, dev := range topo.Devices {
		if !removeSet[i] {
			newDevices = append(newDevices, dev)
		}
	}
	topo.Devices = newDevices

	return len(removeSet)
}

// pickPrimary selects the best device to keep when merging a group.
// Priority: most links > lexically smallest ID (for determinism).
func pickPrimary(topo *topology.Topology, indices []int) int {
	type candidate struct {
		idx       int
		linkCount int
		id        string
	}
	var candidates []candidate
	for _, idx := range indices {
		dev := &topo.Devices[idx]
		count := 0
		for _, l := range topo.Links {
			if l.RemoteDevice == dev.ID || l.LocalDevice == dev.ID {
				count++
			}
		}
		candidates = append(candidates, candidate{idx, count, dev.ID})
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.linkCount > best.linkCount {
			best = c
		} else if c.linkCount == best.linkCount && c.id < best.id {
			best = c
		}
	}
	return best.idx
}

// findDeviceByID checks if a device with the given ID exists in the topology.
func findDeviceByID(topo *topology.Topology, id string) (*topology.Device, bool) {
	for i := range topo.Devices {
		if topo.Devices[i].ID == id {
			return &topo.Devices[i], true
		}
	}
	return nil, false
}

// macAddOffset adds an integer offset to a normalized MAC address (colon-separated).
// Returns the offset MAC and true on success.
func macAddOffset(mac string, offset int) (string, bool) {
	hex := strings.ReplaceAll(mac, ":", "")
	if len(hex) != 12 {
		return "", false
	}
	val, err := strconv.ParseUint(hex, 16, 48)
	if err != nil {
		return "", false
	}
	val = uint64(int64(val) + int64(offset))
	hex = fmt.Sprintf("%012x", val)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		hex[0:2], hex[2:4], hex[4:6], hex[6:8], hex[8:10], hex[10:12]), true
}

// enrichDeviceFromHost fills device metadata from a deployment host entry.
func enrichDeviceFromHost(dev *topology.Device, host *HostNode, matchType string) {
	if dev.Annotations == nil {
		dev.Annotations = make(map[string]string)
	}
	dev.Annotations["deployment_source"] = "true"
	dev.Annotations["deployment_match"] = matchType
	dev.Annotations["deployment_name"] = host.Name

	// Fill gaps (don't overwrite LLDP-discovered data)
	if dev.SystemName == "" {
		dev.SystemName = host.Name
	}
	if dev.ManagementAddress == "" && host.IPv4Address != "" {
		dev.ManagementAddress = host.IPv4Address
	}
	if dev.ManagementAddress == "" && host.BMCIPAddress != "" {
		dev.ManagementAddress = host.BMCIPAddress
	}
}

// synthesizeHost creates a Device from deployment data when LLDP didn't find it.
func synthesizeHost(host HostNode) topology.Device {
	return topology.Device{
		ID:                host.Name,
		Type:              "host",
		SystemName:        host.Name,
		ManagementAddress: host.IPv4Address,
		Annotations: map[string]string{
			"deployment_source":      "true",
			"deployment_synthesized": "true",
			"deployment_match":       "none",
			"deployment_name":        host.Name,
		},
	}
}

// applyRenames rewrites device IDs across the entire topology (devices, links, endpoints).
func applyRenames(topo *topology.Topology, renames map[string]string) {
	// Rename devices
	for i := range topo.Devices {
		if newID, ok := renames[topo.Devices[i].ID]; ok {
			topo.Devices[i].ID = newID
			if topo.Devices[i].SystemName == "" {
				topo.Devices[i].SystemName = newID
			}
		}
	}

	// Rename link references
	for i := range topo.Links {
		if newID, ok := renames[topo.Links[i].LocalDevice]; ok {
			topo.Links[i].LocalDevice = newID
		}
		if newID, ok := renames[topo.Links[i].RemoteDevice]; ok {
			topo.Links[i].RemoteDevice = newID
		}
	}

	// Rename source switches
	for i := range topo.SourceSwitches {
		if newID, ok := renames[topo.SourceSwitches[i]]; ok {
			topo.SourceSwitches[i] = newID
		}
	}
}

// normalizeMAC converts a MAC address to lowercase colon-separated format.
func normalizeMAC(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	// Replace common separators with colons
	mac = strings.ReplaceAll(mac, "-", ":")
	mac = strings.ReplaceAll(mac, ".", "")

	// Handle dot-notation (aaaa.bbbb.cccc → aa:aa:bb:bb:cc:cc)
	if len(mac) == 12 && !strings.Contains(mac, ":") {
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
	}

	return mac
}
