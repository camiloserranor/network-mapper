// Package collector orchestrates gNMI data collection from TOR switches
// and assembles the topology model.
package collector

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// switchResult holds all data collected from a single switch.
type switchResult struct {
	SwitchName   string
	SwitchID     string
	Device       topology.Device
	Neighbors    []transform.LLDPNeighbor
	Interfaces   []topology.Interface
	MACEntries   []transform.MACEntry
	ARPEntries   []transform.ARPEntry
	VLANs        []topology.VLAN
	BGPNeighbors []transform.BGPNeighbor
	Errors       []topology.PartialError
}

// CollectRaw connects to all configured switches, queries gNMI for LLDP,
// interface, system data, and returns the raw per-switch results. This is
// the stage-1 output of the pipeline — pass the result to builder.Build()
// to produce the v2 topology.
func CollectRaw(ctx context.Context, cfg *config.Config) (*CollectionResult, error) {
	now := time.Now()

	results := make([]switchResult, len(cfg.Switches))
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Collect.Parallel)

	for i, sw := range cfg.Switches {
		wg.Add(1)
		go func(idx int, sw config.SwitchConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			swCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Collect.TimeoutSec)*time.Second)
			defer cancel()

			results[idx] = collectSwitch(swCtx, sw, cfg, now)
		}(i, sw)
	}

	wg.Wait()

	cr := &CollectionResult{
		CollectedAt: now,
		Switches:    make([]SwitchData, len(results)),
	}
	for i, r := range results {
		cr.Switches[i] = SwitchData{
			SwitchName:   r.SwitchName,
			SwitchID:     r.SwitchID,
			Device:       r.Device,
			Neighbors:    r.Neighbors,
			Interfaces:   r.Interfaces,
			MACEntries:   r.MACEntries,
			ARPEntries:   r.ARPEntries,
			VLANs:        r.VLANs,
			BGPNeighbors: r.BGPNeighbors,
			Errors:       r.Errors,
		}
	}

	return cr, nil
}

// Collect connects to all configured switches, queries gNMI for LLDP, interface,
// and system data, and returns a complete Topology.
//
// Deprecated: Use CollectRaw + builder.Build for the v2 pipeline.
func Collect(ctx context.Context, cfg *config.Config) (*topology.Topology, error) {
	now := time.Now()

	results := make([]switchResult, len(cfg.Switches))
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Collect.Parallel)

	for i, sw := range cfg.Switches {
		wg.Add(1)
		go func(idx int, sw config.SwitchConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			swCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Collect.TimeoutSec)*time.Second)
			defer cancel()

			results[idx] = collectSwitch(swCtx, sw, cfg, now)
		}(i, sw)
	}

	wg.Wait()

	topo := buildTopology(results, now, cfg.Collect.ReverseDNS)

	return topo, nil
}

func collectSwitch(ctx context.Context, sw config.SwitchConfig, cfg *config.Config, now time.Time) switchResult {
	start := time.Now()
	result := switchResult{
		SwitchName: sw.Name,
		SwitchID:   sw.Name,
	}

	// Determine encoding based on platform
	encoding := "JSON_IETF"
	if sw.Platform == "nxos" {
		encoding = "JSON"
	}

	// Connect
	client, err := gnmi.NewClient(ctx, gnmi.ClientOptions{
		Address:  sw.Address,
		Username: sw.Auth.Username,
		Password: sw.Auth.Password,
		TLS: gnmi.TLSOptions{
			SkipVerify: cfg.TLS.SkipVerify,
			TOFU:       cfg.TLS.TOFU,
			CertDir:    cfg.TLS.CertDir,
			CACert:     cfg.TLS.CACert,
			ClientCert: cfg.TLS.ClientCert,
			ClientKey:  cfg.TLS.ClientKey,
		},
		Encoding: encoding,
	})
	if err != nil {
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "connect", Message: err.Error(),
		})
		return result
	}
	defer client.Close()

	log.Printf("Connected to %s (%s)", sw.Name, sw.Address)

	// 1. Collect system info
	sysInfo := collectSystemInfo(ctx, client, sw, &result)

	// Build the switch device entry
	result.Device = topology.Device{
		ID:                sw.Name,
		Type:              "switch",
		SystemName:        sysInfo.Hostname,
		SoftwareVersion:   sysInfo.SoftwareVersion,
		Uptime:            sysInfo.Uptime,
		ManagementAddress: extractHost(sw.Address),
	}
	if result.Device.SystemName == "" {
		result.Device.SystemName = sw.Name
	}

	// 2. Collect LLDP neighbors
	collectLLDP(ctx, client, sw, &result)

	// 3. Collect interface state
	collectInterfaces(ctx, client, sw, cfg, &result)

	// 4. Collect switch resource utilization (CPU/memory)
	collectResources(ctx, client, sw, &result)

	// 5. Collect MAC table (for VM endpoint discovery)
	collectMACTable(ctx, client, sw, &result)

	// 6. Collect ARP table (for IP-to-MAC mapping)
	collectARPTable(ctx, client, sw, &result)

	// 7. Collect VLAN configuration
	collectVLANs(ctx, client, sw, &result)

	// 8. Collect BGP neighbor state
	collectBGP(ctx, client, sw, &result)

	log.Printf("  %s: collection completed in %s", sw.Name, time.Since(start))

	return result
}

func collectSystemInfo(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) transform.SystemInfo {
	notifs, err := client.Get(ctx, transform.SystemPathOpenConfig)
	if err != nil {
		log.Printf("WARN: system info for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "system", Message: err.Error(),
		})
		return transform.SystemInfo{}
	}
	return transform.ParseSystemOpenConfig(notifs)
}

func collectLLDP(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) {
	var lldpPath string
	if sw.Platform == "nxos" {
		lldpPath = transform.LLDPPathNXOS
	} else {
		lldpPath = transform.LLDPPathOpenConfig
	}

	notifs, err := client.GetWithFallback(ctx, lldpPath)
	if err != nil {
		log.Printf("WARN: LLDP for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "lldp", Message: err.Error(),
		})
		return
	}

	if sw.Platform == "nxos" {
		result.Neighbors = transform.ParseLLDPNXOS(notifs)
	} else {
		result.Neighbors = transform.ParseLLDPOpenConfig(notifs)
	}

	log.Printf("  %s: %d LLDP neighbors", sw.Name, len(result.Neighbors))
}

func collectInterfaces(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, cfg *config.Config, result *switchResult) {
	// Collect interface state (oper-status, speed, MTU, name)
	notifs, err := client.GetWithFallback(ctx, transform.InterfacesPathOpenConfig)
	if err != nil {
		log.Printf("WARN: interfaces for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "interfaces", Message: err.Error(),
		})
		return
	}

	result.Interfaces = transform.ParseInterfacesOpenConfig(notifs)

	// Collect counters separately (different path, often larger payload)
	if !cfg.Collect.SkipCounters && len(result.Interfaces) > 0 {
		counterNotifs, counterErr := client.GetWithFallback(ctx, transform.InterfacesCountersPathOpenConfig)
		if counterErr != nil {
			log.Printf("  %s: interface counters unavailable: %v", sw.Name, counterErr)
		} else {
			transform.MergeInterfaceCounters(result.Interfaces, counterNotifs)
		}
	}

	// Collect per-port VLAN configuration (NX-OS only)
	if sw.Platform == "nxos" && len(result.Interfaces) > 0 {
		vlanNotifs, vlanErr := client.GetWithFallback(ctx, transform.InterfaceVLANPathNXOS)
		if vlanErr != nil {
			log.Printf("  %s: interface VLAN config unavailable: %v", sw.Name, vlanErr)
		} else {
			vlanConfigs := transform.ParseInterfaceVLANsNXOS(vlanNotifs)
			transform.MergeInterfaceVLANConfigs(result.Interfaces, vlanConfigs)
			log.Printf("  %s: %d interface VLAN configs", sw.Name, len(vlanConfigs))
		}
	}

	log.Printf("  %s: %d interfaces", sw.Name, len(result.Interfaces))
}

func collectResources(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) {
	cpuPath := transform.CPUPathOpenConfig
	memPath := transform.MemoryPathOpenConfig
	if sw.Platform == "nxos" {
		cpuPath = transform.CPUPathNXOS
		memPath = transform.MemoryPathNXOS
	}

	cpuNotifs, cpuErr := client.GetWithFallback(ctx, cpuPath)
	memNotifs, memErr := client.GetWithFallback(ctx, memPath)

	if cpuErr != nil && memErr != nil {
		log.Printf("  %s: resource stats unavailable (CPU: %v, Memory: %v)", sw.Name, cpuErr, memErr)
		return
	}

	stats := transform.ParseResourceStatsNXOS(cpuNotifs, memNotifs)
	result.Device.CPUUtilization = stats.CPUUtilization
	result.Device.MemoryUsed = stats.MemoryUsed
	result.Device.MemoryTotal = stats.MemoryTotal

	if stats.MemoryTotal > 0 {
		memPct := float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100
		log.Printf("  %s: CPU %.1f%%, Memory %.1f%% (%d/%d bytes)", sw.Name, stats.CPUUtilization, memPct, stats.MemoryUsed, stats.MemoryTotal)
	}
}

func collectMACTable(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) {
	if sw.Platform != "nxos" {
		return // MAC table collection only supported on NX-OS for now
	}

	notifs, err := client.GetWithFallback(ctx, transform.MACTablePathNXOS)
	if err != nil {
		log.Printf("  %s: MAC table unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "mac-table", Message: err.Error(),
		})
		return
	}

	result.MACEntries = transform.ParseMACTableNXOS(notifs, sw.Name)
	log.Printf("  %s: %d MAC table entries (from %d notifications)", sw.Name, len(result.MACEntries), len(notifs))
	if len(result.MACEntries) == 0 && len(notifs) > 0 {
		// Log raw notification structure for debugging
		for i, n := range notifs {
			log.Printf("  %s: MAC notif[%d]: %d updates", sw.Name, i, len(n.Updates))
			for j, u := range n.Updates {
				if j < 3 { // only first 3 to avoid flooding
					raw := fmt.Sprintf("%v", u.Value)
					if len(raw) > 300 {
						raw = raw[:300] + "..."
					}
					log.Printf("  %s: MAC update[%d]: path=%q value=%s", sw.Name, j, u.Path, raw)
				}
			}
		}
	}
}

func collectARPTable(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) {
	if sw.Platform != "nxos" {
		return // ARP table collection only supported on NX-OS for now
	}

	notifs, err := client.GetWithFallback(ctx, transform.ARPPathNXOS)
	if err != nil {
		log.Printf("  %s: ARP table unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "arp-table", Message: err.Error(),
		})
		return
	}

	result.ARPEntries = transform.ParseARPTableNXOS(notifs, sw.Name)
	log.Printf("  %s: %d ARP entries (from %d notifications)", sw.Name, len(result.ARPEntries), len(notifs))
	if len(result.ARPEntries) == 0 && len(notifs) > 0 {
		for i, n := range notifs {
			log.Printf("  %s: ARP notif[%d]: %d updates", sw.Name, i, len(n.Updates))
			for j, u := range n.Updates {
				if j < 3 {
					raw := fmt.Sprintf("%v", u.Value)
					if len(raw) > 300 {
						raw = raw[:300] + "..."
					}
					log.Printf("  %s: ARP update[%d]: path=%q value=%s", sw.Name, j, u.Path, raw)
				}
			}
		}
	}
}

func collectVLANs(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) {
	if sw.Platform != "nxos" {
		return // VLAN collection only supported on NX-OS for now
	}

	notifs, err := client.GetWithFallback(ctx, transform.VLANPathNXOS)
	if err != nil {
		log.Printf("  %s: VLAN config unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "vlan-config", Message: err.Error(),
		})
		return
	}

	result.VLANs = transform.ParseVLANsNXOS(notifs, sw.Name)
	log.Printf("  %s: %d VLANs", sw.Name, len(result.VLANs))
}

func collectBGP(ctx context.Context, client *gnmi.Client, sw config.SwitchConfig, result *switchResult) {
	var notifs []gnmi.Notification
	var err error

	switch sw.Platform {
	case "nxos":
		notifs, err = client.GetWithFallback(ctx, transform.BGPNeighborsPathNXOS)
		if err != nil {
			log.Printf("  %s: BGP data unavailable: %v", sw.Name, err)
			result.Errors = append(result.Errors, topology.PartialError{
				Switch: sw.Name, Phase: "bgp", Message: err.Error(),
			})
			return
		}
		result.BGPNeighbors = transform.ParseBGPNXOS(notifs)
	default:
		// OpenConfig / SONiC
		notifs, err = client.GetWithFallback(ctx, transform.BGPNeighborsPathOpenConfig)
		if err != nil {
			log.Printf("  %s: BGP data unavailable: %v", sw.Name, err)
			result.Errors = append(result.Errors, topology.PartialError{
				Switch: sw.Name, Phase: "bgp", Message: err.Error(),
			})
			return
		}
		result.BGPNeighbors = transform.ParseBGPOpenConfig(notifs)
	}

	log.Printf("  %s: %d BGP neighbors", sw.Name, len(result.BGPNeighbors))
}

func buildTopology(results []switchResult, now time.Time, reverseDNS bool) *topology.Topology {
	topo := &topology.Topology{
		SchemaVersion: "1.0",
		CollectedAt:   now,
	}

	// Track all devices by ID to deduplicate
	deviceMap := make(map[string]*topology.Device)

	// Build a mapping from system hostname (FQDN) → config ID so that LLDP
	// neighbors pointing to a switch we already queried are merged rather
	// than creating a duplicate node.
	systemNameToID := buildSystemNameIndex(results)

	for _, r := range results {
		topo.SourceSwitches = append(topo.SourceSwitches, r.SwitchID)
		topo.PartialFailures = append(topo.PartialFailures, r.Errors...)

		// Add the switch itself (skip if connect failed and no device was built)
		switchDev := r.Device
		switchDev.Interfaces = r.Interfaces
		switchDev.BGPSessions = bgpNeighborsToSessions(r.BGPNeighbors)
		if switchDev.ID != "" {
			deviceMap[switchDev.ID] = &switchDev
		}

		// Convert LLDP neighbors to links and remote devices
		for _, nbr := range r.Neighbors {
			remoteID := nbr.SystemName
			if remoteID == "" {
				remoteID = nbr.ChassisID
			}
			if remoteID == "" {
				continue
			}

			// Resolve LLDP system name to a configured switch ID when possible
			remoteID = resolveDeviceID(remoteID, systemNameToID)

			// Add or merge remote device
			if existing, ok := deviceMap[remoteID]; ok {
				// Merge: fill in missing fields
				if existing.ChassisID == "" {
					existing.ChassisID = nbr.ChassisID
				}
				if existing.ManagementAddress == "" {
					existing.ManagementAddress = nbr.ManagementAddress
				}
				if existing.SystemDescription == "" {
					existing.SystemDescription = nbr.SystemDescription
				}
			} else {
				deviceMap[remoteID] = &topology.Device{
					ID:                remoteID,
					Type:              classifyDevice(nbr),
					ChassisID:         nbr.ChassisID,
					SystemName:        nbr.SystemName,
					SystemDescription: nbr.SystemDescription,
					ManagementAddress: nbr.ManagementAddress,
				}
			}

			// Build link with interface enrichment
			link := topology.Link{
				LocalDevice:     r.SwitchID,
				LocalPort:       nbr.LocalPort,
				RemoteDevice:    remoteID,
				RemotePort:      nbr.PortID,
				RemoteChassisID: nbr.ChassisID,
				Source:          "lldp",
				DiscoveredAt:    now,
			}

			// Enrich link with interface data from the local switch
			for _, iface := range r.Interfaces {
				if iface.Name == nbr.LocalPort {
					link.OperStatus = iface.OperStatus
					link.Speed = iface.Speed
					if iface.MTU > 0 {
						link.MTU = fmt.Sprintf("%d", iface.MTU)
					}
					link.Counters = iface.Counters
					break
				}
			}

			topo.Links = append(topo.Links, link)
		}
	}

	// Flatten device map
	for _, d := range deviceMap {
		topo.Devices = append(topo.Devices, *d)
	}

	// Collect VLANs from all switches (deduplicate by ID)
	vlanMap := make(map[int]topology.VLAN)
	for _, r := range results {
		for _, v := range r.VLANs {
			if _, exists := vlanMap[v.ID]; !exists {
				vlanMap[v.ID] = v
			}
		}
	}
	for _, v := range vlanMap {
		topo.VLANs = append(topo.VLANs, v)
	}

	// Enrich unknown devices using ARP-port correlation (L2/L3 switch data only)
	var enrichInputs []transform.HostEnrichmentInput
	for _, r := range results {
		if len(r.MACEntries) > 0 || len(r.ARPEntries) > 0 {
			enrichInputs = append(enrichInputs, transform.HostEnrichmentInput{
				SwitchID:   r.SwitchID,
				Neighbors:  r.Neighbors,
				MACEntries: r.MACEntries,
				ARPEntries: r.ARPEntries,
			})
		}
	}
	if len(enrichInputs) > 0 {
		transform.EnrichDevicesFromSwitchData(topo, enrichInputs, transform.HostEnrichmentConfig{
			ReverseDNS: reverseDNS,
		})
	}

	// Correlate endpoints (VMs) from MAC/ARP/LLDP data
	var correlationInputs []transform.CorrelationInput
	for _, r := range results {
		if len(r.MACEntries) > 0 {
			correlationInputs = append(correlationInputs, transform.CorrelationInput{
				SwitchID:   r.SwitchID,
				Neighbors:  r.Neighbors,
				MACEntries: r.MACEntries,
				ARPEntries: r.ARPEntries,
			})
		}
	}
	if len(correlationInputs) > 0 {
		topo.Endpoints = transform.CorrelateEndpoints(correlationInputs)

		// Assign VLAN memberships to devices based on endpoint data
		deviceVLANs := make(map[string]map[int]bool)
		for _, ep := range topo.Endpoints {
			if ep.HostDevice != "" {
				if deviceVLANs[ep.HostDevice] == nil {
					deviceVLANs[ep.HostDevice] = make(map[int]bool)
				}
				for _, vid := range ep.VLANs {
					deviceVLANs[ep.HostDevice][vid] = true
				}
			}
		}
		for i := range topo.Devices {
			if vlans, ok := deviceVLANs[topo.Devices[i].ID]; ok {
				for vid := range vlans {
					topo.Devices[i].VLANs = append(topo.Devices[i].VLANs, vid)
				}
			}
		}
	}

	if topo.PartialFailures == nil {
		topo.PartialFailures = []topology.PartialError{}
	}

	// Populate ObservedVLANs on interfaces from MAC table data
	enrichInterfaceObservedVLANs(topo, results)

	return topo
}

// buildSystemNameIndex creates a mapping from each queried switch's FQDN
// (SystemName) back to its config-level ID. This allows LLDP-discovered
// neighbors to be merged with the switch device we already created.
func buildSystemNameIndex(results []switchResult) map[string]string {
	idx := make(map[string]string)
	for _, r := range results {
		sysName := r.Device.SystemName
		if sysName != "" && sysName != r.SwitchID {
			idx[sysName] = r.SwitchID
		}
	}
	return idx
}

// resolveDeviceID maps an LLDP-discovered device name to a configured switch
// ID if one exists, preventing duplicate nodes for the same physical switch.
func resolveDeviceID(id string, systemNameToID map[string]string) string {
	if mapped, ok := systemNameToID[id]; ok {
		return mapped
	}
	return id
}

func classifyDevice(nbr transform.LLDPNeighbor) string {
	return transform.ClassifyDevice(nbr.SystemDescription, nbr.SystemName, nbr.Capabilities)
}

// enrichInterfaceObservedVLANs aggregates VLAN IDs from MAC table entries
// onto the corresponding switch interfaces. This shows which VLANs have
// active traffic on each port, derived from observed MAC learning.
func enrichInterfaceObservedVLANs(topo *topology.Topology, results []switchResult) {
	// Build per-switch, per-port VLAN sets from MAC entries
	// Key: switchID → portName → set of VLAN IDs
	type portVLANs map[string]map[int]bool
	switchPortVLANs := make(map[string]portVLANs)

	for _, r := range results {
		for _, mac := range r.MACEntries {
			if mac.Port == "" || mac.VLAN == 0 {
				continue
			}
			port := transform.NormalizeInterfaceName(mac.Port)
			if switchPortVLANs[r.SwitchID] == nil {
				switchPortVLANs[r.SwitchID] = make(portVLANs)
			}
			if switchPortVLANs[r.SwitchID][port] == nil {
				switchPortVLANs[r.SwitchID][port] = make(map[int]bool)
			}
			switchPortVLANs[r.SwitchID][port][mac.VLAN] = true
		}
	}

	// Merge onto device interfaces
	for i := range topo.Devices {
		dev := &topo.Devices[i]
		pvs, ok := switchPortVLANs[dev.ID]
		if !ok {
			continue
		}
		for j := range dev.Interfaces {
			vlanSet, ok := pvs[dev.Interfaces[j].Name]
			if !ok {
				continue
			}
			var vlans []int
			for v := range vlanSet {
				vlans = append(vlans, v)
			}
			sort.Ints(vlans)
			dev.Interfaces[j].ObservedVLANs = vlans
		}
	}
}

func extractHost(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return address[:i]
		}
	}
	return address
}

// bgpNeighborsToSessions converts transform.BGPNeighbor to topology.BGPSession.
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
