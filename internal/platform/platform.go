// Package platform defines the vendor-specific collection strategy interface.
// Each platform implementation encapsulates gNMI path selection, fetch strategy,
// and parser dispatch for a specific switch vendor (NX-OS, SONiC, etc.).
package platform

import (
	"context"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// Platform defines vendor-specific gNMI collection and parsing behavior.
// Each method handles path selection, data fetching, and parsing for one data category.
// Implementations are stateless — all switch-specific context is passed as parameters.
type Platform interface {
	// Name returns the platform identifier (e.g., "nxos", "sonic").
	Name() string

	// Encoding returns the gNMI encoding to use for this platform ("JSON" or "JSON_IETF").
	Encoding() string

	// CollectSystem fetches and parses system info (hostname, version, uptime).
	CollectSystem(ctx context.Context, client *gnmi.Client) (transform.SystemInfo, error)

	// CollectLLDP fetches and parses LLDP neighbor data.
	CollectLLDP(ctx context.Context, client *gnmi.Client) ([]transform.LLDPNeighbor, error)

	// CollectInterfaces fetches and parses interface state and counters.
	CollectInterfaces(ctx context.Context, client *gnmi.Client) ([]topology.Interface, error)

	// CollectResources fetches and parses CPU/memory utilization.
	CollectResources(ctx context.Context, client *gnmi.Client) (transform.ResourceStats, error)

	// CollectMACTable fetches and parses the MAC/FDB table.
	CollectMACTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.MACEntry, error)

	// CollectARPTable fetches and parses the ARP/neighbor table.
	CollectARPTable(ctx context.Context, client *gnmi.Client, switchName string) ([]transform.ARPEntry, error)

	// CollectVLANs fetches and parses VLAN configuration.
	CollectVLANs(ctx context.Context, client *gnmi.Client, switchName string) ([]topology.VLAN, error)

	// CollectBGP fetches and parses BGP neighbor state.
	CollectBGP(ctx context.Context, client *gnmi.Client) ([]transform.BGPNeighbor, error)

	// EnrichInterfaces returns true if VLAN data should be cross-referenced
	// into interface VLAN fields (used by OpenConfig platforms where per-port
	// VLAN config isn't available from a separate path).
	EnrichInterfacesFromVLANs() bool
}

// ForPlatform returns the Platform implementation for the given platform name.
// Unknown platforms fall back to the generic OpenConfig implementation.
func ForPlatform(name string) Platform {
	switch name {
	case "nxos":
		return &NXOSPlatform{}
	case "sonic":
		return &SONiCPlatform{}
	default:
		return &OpenConfigPlatform{}
	}
}
