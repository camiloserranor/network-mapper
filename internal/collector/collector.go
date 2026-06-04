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
	"github.com/camiloserranor/network-mapper/internal/platform"
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
	NVEPeers     []transform.NVEPeer
	L2RIBMacs    []transform.L2RIBEntry
	QoSStats     []transform.QoSStats
	PFCConfig    []transform.PFCConfig
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
			NVEPeers:     r.NVEPeers,
			L2RIBMacs:    r.L2RIBMacs,
			QoSStats:     r.QoSStats,
			PFCConfig:    r.PFCConfig,
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

	// Resolve the platform strategy
	p := platform.ForPlatform(sw.Platform)

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
		Encoding: p.Encoding(),
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
	sysInfo, err := p.CollectSystem(ctx, client)
	if err != nil {
		log.Printf("WARN: system info for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "system", Message: err.Error(),
		})
	}

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
	neighbors, err := p.CollectLLDP(ctx, client)
	if err != nil {
		log.Printf("WARN: LLDP for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "lldp", Message: err.Error(),
		})
	} else {
		result.Neighbors = neighbors
		log.Printf("  %s: %d LLDP neighbors", sw.Name, len(neighbors))
	}

	// 3. Collect interface state
	ifaces, err := p.CollectInterfaces(ctx, client)
	if err != nil {
		log.Printf("WARN: interfaces for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "interfaces", Message: err.Error(),
		})
	} else {
		result.Interfaces = ifaces
		log.Printf("  %s: %d interfaces", sw.Name, len(ifaces))
	}

	// 4. Collect switch resource utilization (CPU/memory)
	stats, err := p.CollectResources(ctx, client)
	if err != nil {
		log.Printf("  %s: resource stats unavailable: %v", sw.Name, err)
	} else {
		result.Device.CPUUtilization = stats.CPUUtilization
		result.Device.MemoryUsed = stats.MemoryUsed
		result.Device.MemoryTotal = stats.MemoryTotal
		if stats.MemoryTotal > 0 {
			memPct := float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100
			log.Printf("  %s: CPU %.1f%%, Memory %.1f%% (%d/%d bytes)", sw.Name, stats.CPUUtilization, memPct, stats.MemoryUsed, stats.MemoryTotal)
		}
	}

	// 5. Collect MAC table (for VM endpoint discovery)
	macEntries, err := p.CollectMACTable(ctx, client, sw.Name)
	if err != nil {
		log.Printf("  %s: MAC table unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "mac-table", Message: err.Error(),
		})
	} else {
		result.MACEntries = macEntries
		log.Printf("  %s: %d MAC table entries", sw.Name, len(macEntries))
	}

	// 6. Collect ARP table (for IP-to-MAC mapping)
	arpEntries, err := p.CollectARPTable(ctx, client, sw.Name)
	if err != nil {
		log.Printf("  %s: ARP table unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "arp-table", Message: err.Error(),
		})
	} else {
		result.ARPEntries = arpEntries
		log.Printf("  %s: %d ARP entries", sw.Name, len(arpEntries))
	}

	// 7. Collect VLAN configuration
	vlans, err := p.CollectVLANs(ctx, client, sw.Name)
	if err != nil {
		log.Printf("  %s: VLAN config unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "vlan-config", Message: err.Error(),
		})
	} else {
		result.VLANs = vlans
		log.Printf("  %s: %d VLANs", sw.Name, len(vlans))
	}

	// 8. Enrich interfaces with VLAN data (for platforms without per-port VLAN paths)
	if p.EnrichInterfacesFromVLANs() && len(result.VLANs) > 0 {
		transform.EnrichInterfaceVLANsFromVLANConfig(result.Interfaces, result.VLANs)
	}

	// 9. Collect BGP neighbor state
	bgpNeighbors, err := p.CollectBGP(ctx, client)
	if err != nil {
		log.Printf("  %s: BGP data unavailable: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "bgp", Message: err.Error(),
		})
	} else {
		result.BGPNeighbors = bgpNeighbors
		log.Printf("  %s: %d BGP neighbors", sw.Name, len(bgpNeighbors))
	}

	// 10. Collect NVE peers (VXLAN/EVPN — optional capability)
	if vc, ok := p.(platform.VXLANCollector); ok {
		nvePeers, nveErr := vc.CollectNVEPeers(ctx, client)
		if nveErr != nil {
			log.Printf("  %s: NVE peers unavailable: %v", sw.Name, nveErr)
			result.Errors = append(result.Errors, topology.PartialError{
				Switch: sw.Name, Phase: "nve-peers", Message: nveErr.Error(),
			})
		} else if len(nvePeers) > 0 {
			result.NVEPeers = nvePeers
			log.Printf("  %s: %d NVE peers", sw.Name, len(nvePeers))

			// 11. Only collect L2RIB if we have NVE peers AND MACs on NVE ports
			hasNVEMacs := false
			for _, mac := range result.MACEntries {
				if transform.IsNVEInterface(mac.Port) {
					hasNVEMacs = true
					break
				}
			}
			if hasNVEMacs {
				l2ribMacs, l2ribErr := vc.CollectL2RIB(ctx, client)
				if l2ribErr != nil {
					log.Printf("  %s: L2RIB unavailable: %v", sw.Name, l2ribErr)
					result.Errors = append(result.Errors, topology.PartialError{
						Switch: sw.Name, Phase: "l2rib", Message: l2ribErr.Error(),
					})
				} else if len(l2ribMacs) > 0 {
					result.L2RIBMacs = l2ribMacs
					log.Printf("  %s: %d L2RIB MAC entries", sw.Name, len(l2ribMacs))
				}
			}
		}
	}

	// 12. Collect QoS stats (RDMA monitoring — optional capability)
	if qc, ok := p.(platform.QoSCollector); ok {
		qosStats, qosErr := qc.CollectQoSStats(ctx, client)
		if qosErr != nil {
			log.Printf("  %s: QoS stats unavailable: %v", sw.Name, qosErr)
			result.Errors = append(result.Errors, topology.PartialError{
				Switch: sw.Name, Phase: "qos-stats", Message: qosErr.Error(),
			})
		} else if len(qosStats) > 0 {
			result.QoSStats = qosStats
			log.Printf("  %s: %d interfaces with QoS stats", sw.Name, len(qosStats))
		}

		// 13. Collect PFC config
		pfcConfig, pfcErr := qc.CollectPFCConfig(ctx, client)
		if pfcErr != nil {
			log.Printf("  %s: PFC config unavailable: %v", sw.Name, pfcErr)
			result.Errors = append(result.Errors, topology.PartialError{
				Switch: sw.Name, Phase: "pfc-config", Message: pfcErr.Error(),
			})
		} else if len(pfcConfig) > 0 {
			result.PFCConfig = pfcConfig
			log.Printf("  %s: %d interfaces with PFC config", sw.Name, len(pfcConfig))
		}
	}

	log.Printf("  %s: collection completed in %s", sw.Name, time.Since(start))

	return result
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
