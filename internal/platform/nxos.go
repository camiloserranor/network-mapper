package platform

import (
	"context"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// NXOSPlatform implements collection for Cisco NX-OS switches.
// Prefers OpenConfig paths for operational data (oper-status, counters, system info)
// and supplements with NX-OS native /System/ paths for platform-specific data
// (VLAN config, LLDP, MAC table, ARP, BGP, VXLAN, QoS).
type NXOSPlatform struct{}

func (p *NXOSPlatform) Name() string     { return "nxos" }
func (p *NXOSPlatform) Encoding() string  { return "JSON" }
func (p *NXOSPlatform) EnrichInterfacesFromVLANs() bool { return false }

func (p *NXOSPlatform) CollectSystem(ctx context.Context, client gnmi.GNMIClient) (transform.SystemInfo, error) {
	// Prefer OpenConfig — well-tested, standardized fields (hostname, version).
	notifs, err := client.Get(ctx, transform.SystemPathOpenConfig)
	if err == nil {
		info := transform.ParseSystemOpenConfig(notifs)
		if info.Hostname != "" || info.SoftwareVersion != "" {
			return info, nil
		}
	}
	// Fall back to NX-OS native path
	notifs, err = client.Get(ctx, transform.SystemPathNXOS)
	if err != nil {
		return transform.SystemInfo{}, err
	}
	return transform.ParseSystemNXOS(notifs), nil
}

func (p *NXOSPlatform) CollectLLDP(ctx context.Context, client gnmi.GNMIClient) ([]transform.LLDPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.LLDPPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseLLDPNXOS(notifs), nil
}

func (p *NXOSPlatform) CollectInterfaces(ctx context.Context, client gnmi.GNMIClient) ([]topology.Interface, error) {
	// Prefer OpenConfig — provides oper-status, counters, and is well-tested.
	// NX-OS native path is used as supplement for VLAN config that OpenConfig lacks.
	var ifaces []topology.Interface

	ocNotifs, ocErr := client.GetWithFallback(ctx, transform.InterfacesPathOpenConfig)
	if ocErr == nil {
		ifaces = transform.ParseInterfacesOpenConfig(ocNotifs)
	}

	if len(ifaces) == 0 {
		// OpenConfig unavailable — fall back to NX-OS native path
		nxNotifs, nxErr := client.GetWithFallback(ctx, transform.InterfacesPathNXOS)
		if nxErr != nil {
			if ocErr != nil {
				return nil, ocErr
			}
			return nil, nxErr
		}
		ifaces = transform.ParseInterfacesNXOS(nxNotifs)

		// Since NX-OS native path lacks oper-status, attempt OpenConfig one more
		// time with the counter path to get at least some operational data.
	}

	// Collect counters separately (OpenConfig counter path)
	counterNotifs, counterErr := client.GetWithFallback(ctx, transform.InterfacesCountersPathOpenConfig)
	if counterErr == nil {
		transform.MergeInterfaceCounters(ifaces, counterNotifs)
	}

	// Supplement with NX-OS native VLAN configuration (mode, trunk VLANs, native VLAN).
	// OpenConfig does not expose per-port VLAN details on NX-OS.
	vlanNotifs, vlanErr := client.GetWithFallback(ctx, transform.InterfaceVLANPathNXOS)
	if vlanErr == nil {
		vlanConfigs := transform.ParseInterfaceVLANsNXOS(vlanNotifs)
		transform.MergeInterfaceVLANConfigs(ifaces, vlanConfigs)
	}

	return ifaces, nil
}

func (p *NXOSPlatform) CollectResources(ctx context.Context, client gnmi.GNMIClient) (transform.ResourceStats, error) {
	cpuNotifs, cpuErr := client.GetWithFallback(ctx, transform.CPUPathNXOS)
	memNotifs, memErr := client.GetWithFallback(ctx, transform.MemoryPathNXOS)
	if cpuErr != nil && memErr != nil {
		return transform.ResourceStats{}, cpuErr
	}
	return transform.ParseResourceStatsNXOS(cpuNotifs, memNotifs), nil
}

func (p *NXOSPlatform) CollectMACTable(ctx context.Context, client gnmi.GNMIClient, switchName string) ([]transform.MACEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.MACTablePathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseMACTableNXOS(notifs, switchName), nil
}

func (p *NXOSPlatform) CollectARPTable(ctx context.Context, client gnmi.GNMIClient, switchName string) ([]transform.ARPEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.ARPPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseARPTableNXOS(notifs, switchName), nil
}

func (p *NXOSPlatform) CollectVLANs(ctx context.Context, client gnmi.GNMIClient, switchName string) ([]topology.VLAN, error) {
	notifs, err := client.GetWithFallback(ctx, transform.VLANPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseVLANsNXOS(notifs, switchName), nil
}

func (p *NXOSPlatform) CollectBGP(ctx context.Context, client gnmi.GNMIClient) ([]transform.BGPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.BGPNeighborsPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseBGPNXOS(notifs), nil
}

// --- VXLANCollector implementation ---

func (p *NXOSPlatform) CollectNVEPeers(ctx context.Context, client gnmi.GNMIClient) ([]transform.NVEPeer, error) {
	notifs, err := client.GetWithFallback(ctx, transform.NVEPeersPathNXOS)
	if err != nil {
		if gnmi.IsPathNotSupported(err) {
			return nil, nil
		}
		return nil, err
	}
	return transform.ParseNVEPeersNXOS(notifs), nil
}

func (p *NXOSPlatform) CollectL2RIB(ctx context.Context, client gnmi.GNMIClient) ([]transform.L2RIBEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.L2RIBPathNXOS)
	if err != nil {
		if gnmi.IsPathNotSupported(err) {
			return nil, nil
		}
		return nil, err
	}
	return transform.ParseL2RIBNXOS(notifs), nil
}

// --- QoSCollector implementation ---

func (p *NXOSPlatform) CollectQoSStats(ctx context.Context, client gnmi.GNMIClient) ([]transform.QoSStats, error) {
	notifs, err := client.GetWithFallback(ctx, transform.QoSStatsPathNXOS)
	if err != nil {
		if gnmi.IsPathNotSupported(err) {
			return nil, nil
		}
		return nil, err
	}
	return transform.ParseQoSStatsNXOS(notifs), nil
}

func (p *NXOSPlatform) CollectPFCConfig(ctx context.Context, client gnmi.GNMIClient) ([]transform.PFCConfig, error) {
	// PFC config is embedded in the interface data; we re-fetch interface list
	// and extract priorflowctrl-items from each PhysIf
	notifs, err := client.GetWithFallback(ctx, transform.InterfacesPathNXOS)
	if err != nil {
		if gnmi.IsPathNotSupported(err) {
			return nil, nil
		}
		return nil, err
	}
	return transform.ParsePFCConfigNXOS(notifs), nil
}
