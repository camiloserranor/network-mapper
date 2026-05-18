package platform

import (
	"context"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// SONiCPlatform implements collection for Dell Enterprise SONiC switches.
// Uses OpenConfig paths with GetWithFallback (Subscribe ONCE) for all categories.
// Handles SONiC-specific quirks:
//   - Flat-leaf response format (parsers auto-detect)
//   - No separate counters query (counters included in flat-leaf interface response)
//   - Explicit BGP path (SONiC rejects wildcard paths in protocol subtree)
type SONiCPlatform struct{}

func (p *SONiCPlatform) Name() string     { return "sonic" }
func (p *SONiCPlatform) Encoding() string  { return "JSON_IETF" }
func (p *SONiCPlatform) EnrichInterfacesFromVLANs() bool { return true }

func (p *SONiCPlatform) CollectSystem(ctx context.Context, client *gnmi.Client) (transform.SystemInfo, error) {
	notifs, err := client.Get(ctx, transform.SystemPathOpenConfig)
	if err != nil {
		return transform.SystemInfo{}, err
	}
	return transform.ParseSystemOpenConfig(notifs), nil
}

func (p *SONiCPlatform) CollectLLDP(ctx context.Context, client *gnmi.Client) ([]transform.LLDPNeighbor, error) {
	notifs, err := client.GetWithFallback(ctx, transform.LLDPPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseLLDPOpenConfig(notifs), nil
}

func (p *SONiCPlatform) CollectInterfaces(ctx context.Context, client *gnmi.Client) ([]topology.Interface, error) {
	notifs, err := client.GetWithFallback(ctx, transform.InterfacesPathOpenConfig)
	if err != nil {
		return nil, err
	}
	// SONiC flat-leaf response already includes counters — no separate query needed.
	return transform.ParseInterfacesOpenConfig(notifs), nil
}

func (p *SONiCPlatform) CollectResources(ctx context.Context, client *gnmi.Client) (transform.ResourceStats, error) {
	cpuNotifs, cpuErr := client.GetWithFallback(ctx, transform.CPUPathOpenConfig)
	memNotifs, memErr := client.GetWithFallback(ctx, transform.MemoryPathOpenConfig)
	if cpuErr != nil && memErr != nil {
		return transform.ResourceStats{}, cpuErr
	}
	return transform.ParseResourceStatsOpenConfig(cpuNotifs, memNotifs), nil
}

func (p *SONiCPlatform) CollectMACTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.MACEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.MACTablePathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseMACTableOpenConfig(notifs, switchName), nil
}

func (p *SONiCPlatform) CollectARPTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.ARPEntry, error) {
	notifs, err := client.GetWithFallback(ctx, transform.ARPPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseARPTableOpenConfig(notifs, switchName), nil
}

func (p *SONiCPlatform) CollectVLANs(ctx context.Context, client *gnmi.Client, switchName string) ([]topology.VLAN, error) {
	notifs, err := client.GetWithFallback(ctx, transform.VLANPathOpenConfig)
	if err != nil {
		return nil, err
	}
	return transform.ParseVLANsOpenConfig(notifs, switchName), nil
}

func (p *SONiCPlatform) CollectBGP(ctx context.Context, client *gnmi.Client) ([]transform.BGPNeighbor, error) {
	// SONiC doesn't support wildcard paths in the protocol subtree.
	// Use the SONiC-specific path that avoids wildcards.
	notifs, err := client.GetWithFallback(ctx, transform.BGPNeighborsPathSONiC)
	if err != nil {
		return nil, err
	}
	return transform.ParseBGPOpenConfig(notifs), nil
}
