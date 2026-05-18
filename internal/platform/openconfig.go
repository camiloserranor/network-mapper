package platform

import (
	"context"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// OpenConfigPlatform is the generic fallback for platforms that support standard
// OpenConfig YANG paths. Used for unknown or future vendors.
type OpenConfigPlatform struct{}

func (p *OpenConfigPlatform) Name() string     { return "openconfig" }
func (p *OpenConfigPlatform) Encoding() string  { return "JSON_IETF" }
func (p *OpenConfigPlatform) EnrichInterfacesFromVLANs() bool { return true }

func (p *OpenConfigPlatform) CollectSystem(ctx context.Context, client *gnmi.Client) (transform.SystemInfo, error) {
	notifs, err := client.Get(ctx, transform.SystemPathOpenConfig)
	if err != nil {
		return transform.SystemInfo{}, err
	}
	return transform.ParseSystemOpenConfig(notifs), nil
}

func (p *OpenConfigPlatform) CollectLLDP(ctx context.Context, client *gnmi.Client) ([]transform.LLDPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.LLDPPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseLLDPOpenConfig(notifs), nil
}

func (p *OpenConfigPlatform) CollectInterfaces(ctx context.Context, client *gnmi.Client) ([]topology.Interface, error) {
	notifs, err := client.GetWithFallback(ctx, transform.InterfacesPathOpenConfig)
	if err != nil {
		return nil, err
	}
	ifaces := transform.ParseInterfacesOpenConfig(notifs)

	// Collect counters separately (different path, often larger payload)
	counterNotifs, counterErr := client.GetWithFallback(ctx, transform.InterfacesCountersPathOpenConfig)
	if counterErr == nil {
		transform.MergeInterfaceCounters(ifaces, counterNotifs)
	}

	return ifaces, nil
}

func (p *OpenConfigPlatform) CollectResources(ctx context.Context, client *gnmi.Client) (transform.ResourceStats, error) {
	cpuNotifs, cpuErr := client.GetWithFallback(ctx, transform.CPUPathOpenConfig)
	memNotifs, memErr := client.GetWithFallback(ctx, transform.MemoryPathOpenConfig)
	if cpuErr != nil && memErr != nil {
		return transform.ResourceStats{}, cpuErr
	}
	return transform.ParseResourceStatsOpenConfig(cpuNotifs, memNotifs), nil
}

func (p *OpenConfigPlatform) CollectMACTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.MACEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.MACTablePathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseMACTableOpenConfig(notifs, switchName), nil
}

func (p *OpenConfigPlatform) CollectARPTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.ARPEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.ARPPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseARPTableOpenConfig(notifs, switchName), nil
}

func (p *OpenConfigPlatform) CollectVLANs(ctx context.Context, client *gnmi.Client, switchName string) ([]topology.VLAN, error) {
	notifs, err := client.GetWithFallback(ctx, transform.VLANPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseVLANsOpenConfig(notifs, switchName), nil
}

func (p *OpenConfigPlatform) CollectBGP(ctx context.Context, client *gnmi.Client) ([]transform.BGPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.BGPNeighborsPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseBGPOpenConfig(notifs), nil
}
