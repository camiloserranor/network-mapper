// Package collector orchestrates gNMI data collection from TOR switches
// and assembles the topology model.
package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// switchResult holds all data collected from a single switch.
type switchResult struct {
	SwitchName string
	SwitchID   string
	Device     topology.Device
	Neighbors  []transform.LLDPNeighbor
	Interfaces []topology.Interface
	Errors     []topology.PartialError
}

// Collect connects to all configured switches, queries gNMI for LLDP, interface,
// and system data, and returns a complete Topology.
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

	return buildTopology(results, now), nil
}

func collectSwitch(ctx context.Context, sw config.SwitchConfig, cfg *config.Config, now time.Time) switchResult {
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
	if cfg.Collect.SkipCounters {
		return
	}

	notifs, err := client.GetWithFallback(ctx, transform.InterfacesPathOpenConfig)
	if err != nil {
		log.Printf("WARN: interfaces for %s: %v", sw.Name, err)
		result.Errors = append(result.Errors, topology.PartialError{
			Switch: sw.Name, Phase: "interfaces", Message: err.Error(),
		})
		return
	}

	result.Interfaces = transform.ParseInterfacesOpenConfig(notifs)
	log.Printf("  %s: %d interfaces", sw.Name, len(result.Interfaces))
}

func buildTopology(results []switchResult, now time.Time) *topology.Topology {
	topo := &topology.Topology{
		SchemaVersion: "1.0",
		CollectedAt:   now,
	}

	// Track all devices by ID to deduplicate
	deviceMap := make(map[string]*topology.Device)

	for _, r := range results {
		topo.SourceSwitches = append(topo.SourceSwitches, r.SwitchID)
		topo.PartialFailures = append(topo.PartialFailures, r.Errors...)

		// Add the switch itself
		switchDev := r.Device
		switchDev.Interfaces = r.Interfaces
		deviceMap[switchDev.ID] = &switchDev

		// Convert LLDP neighbors to links and remote devices
		for _, nbr := range r.Neighbors {
			remoteID := nbr.SystemName
			if remoteID == "" {
				remoteID = nbr.ChassisID
			}
			if remoteID == "" {
				continue
			}

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

	if topo.PartialFailures == nil {
		topo.PartialFailures = []topology.PartialError{}
	}

	return topo
}

func classifyDevice(nbr transform.LLDPNeighbor) string {
	return transform.ClassifyDevice(nbr.SystemDescription, nbr.SystemName)
}

func extractHost(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return address[:i]
		}
	}
	return address
}
