package platform

import (
	"context"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// NXOSPlatform implements collection for Cisco NX-OS switches.
// Uses native /System/ paths for maximum data fidelity, with OpenConfig fallback
// for system info when native paths fail.
type NXOSPlatform struct{}

func (p *NXOSPlatform) Name() string     { return "nxos" }
func (p *NXOSPlatform) Encoding() string  { return "JSON" }
func (p *NXOSPlatform) EnrichInterfacesFromVLANs() bool { return false }

func (p *NXOSPlatform) CollectSystem(ctx context.Context, client *gnmi.Client) (transform.SystemInfo, error) {
	// Try the richer native system path first
	notifs, err := client.Get(ctx, transform.SystemPathNXOS)
	if err == nil {
		info := transform.ParseSystemNXOS(notifs)
		if info.Hostname != "" || info.SoftwareVersion != "" {
			return info, nil
		}
	}
	// Fall through to OpenConfig if NX-OS path fails
	notifs, err = client.Get(ctx, transform.SystemPathOpenConfig)
	if err != nil {
		return transform.SystemInfo{}, err
	}
	return transform.ParseSystemOpenConfig(notifs), nil
}

func (p *NXOSPlatform) CollectLLDP(ctx context.Context, client *gnmi.Client) ([]transform.LLDPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.LLDPPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseLLDPNXOS(notifs), nil
}

func (p *NXOSPlatform) CollectInterfaces(ctx context.Context, client *gnmi.Client) ([]topology.Interface, error) {
	notifs, err := client.GetWithFallback(ctx, transform.InterfacesPathOpenConfig)
	if err != nil {
		return nil, err
	}
	ifaces := transform.ParseInterfacesOpenConfig(notifs)

	// Collect counters separately
	counterNotifs, counterErr := client.GetWithFallback(ctx, transform.InterfacesCountersPathOpenConfig)
	if counterErr == nil {
		transform.MergeInterfaceCounters(ifaces, counterNotifs)
	}

	// Collect per-port VLAN configuration (NX-OS native path)
	vlanNotifs, vlanErr := client.GetWithFallback(ctx, transform.InterfaceVLANPathNXOS)
	if vlanErr == nil {
		vlanConfigs := transform.ParseInterfaceVLANsNXOS(vlanNotifs)
		transform.MergeInterfaceVLANConfigs(ifaces, vlanConfigs)
	}

	return ifaces, nil
}

func (p *NXOSPlatform) CollectResources(ctx context.Context, client *gnmi.Client) (transform.ResourceStats, error) {
	cpuNotifs, cpuErr := client.GetWithFallback(ctx, transform.CPUPathNXOS)
	memNotifs, memErr := client.GetWithFallback(ctx, transform.MemoryPathNXOS)
	if cpuErr != nil && memErr != nil {
		return transform.ResourceStats{}, cpuErr
	}
	return transform.ParseResourceStatsNXOS(cpuNotifs, memNotifs), nil
}

func (p *NXOSPlatform) CollectMACTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.MACEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.MACTablePathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseMACTableNXOS(notifs, switchName), nil
}

func (p *NXOSPlatform) CollectARPTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.ARPEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.ARPPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseARPTableNXOS(notifs, switchName), nil
}

func (p *NXOSPlatform) CollectVLANs(ctx context.Context, client *gnmi.Client, switchName string) ([]topology.VLAN, error) {
	notifs, err := client.GetWithFallback(ctx, transform.VLANPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseVLANsNXOS(notifs, switchName), nil
}

func (p *NXOSPlatform) CollectBGP(ctx context.Context, client *gnmi.Client) ([]transform.BGPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.BGPNeighborsPathNXOS)
	if err != nil {
		return nil, err
	}
	return transform.ParseBGPNXOS(notifs), nil
}
