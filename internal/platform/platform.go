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
	CollectSystem(ctx context.Context, client gnmi.GNMIClient) (transform.SystemInfo, error)

	// CollectLLDP fetches and parses LLDP neighbor data.
	CollectLLDP(ctx context.Context, client gnmi.GNMIClient) ([]transform.LLDPNeighbor, error)

	// CollectInterfaces fetches and parses interface state and counters.
	CollectInterfaces(ctx context.Context, client gnmi.GNMIClient) ([]topology.Interface, error)

	// CollectResources fetches and parses CPU/memory utilization.
	CollectResources(ctx context.Context, client gnmi.GNMIClient) (transform.ResourceStats, error)

	// CollectMACTable fetches and parses the MAC/FDB table.
	CollectMACTable(ctx context.Context, client gnmi.GNMIClient, switchName string) ([]transform.MACEntry, error)

	// CollectARPTable fetches and parses the ARP/neighbor table.
	CollectARPTable(ctx context.Context, client gnmi.GNMIClient, switchName string) ([]transform.ARPEntry, error)

	// CollectVLANs fetches and parses VLAN configuration.
	CollectVLANs(ctx context.Context, client gnmi.GNMIClient, switchName string) ([]topology.VLAN, error)

	// CollectBGP fetches and parses BGP neighbor state.
	CollectBGP(ctx context.Context, client gnmi.GNMIClient) ([]transform.BGPNeighbor, error)

	// EnrichInterfaces returns true if VLAN data should be cross-referenced
	// into interface VLAN fields (used by OpenConfig platforms where per-port
	// VLAN config isn't available from a separate path).
	EnrichInterfacesFromVLANs() bool
}

// VXLANCollector is an optional capability interface for platforms that support
// VXLAN/EVPN data collection. The collector uses type assertion to check whether
// a Platform supports this, enabling VM-to-VTEP correlation in overlay environments.
type VXLANCollector interface {
	// CollectNVEPeers fetches VTEP peer information.
	// Returns nil, nil if no NVE is configured on the switch.
	CollectNVEPeers(ctx context.Context, client gnmi.GNMIClient) ([]transform.NVEPeer, error)

	// CollectL2RIB fetches L2RIB MAC routes with next-hop VTEP IPs.
	// Returns nil, nil if L2RIB is empty or not available.
	CollectL2RIB(ctx context.Context, client gnmi.GNMIClient) ([]transform.L2RIBEntry, error)
}

// QoSCollector is an optional capability interface for platforms that support
// per-queue QoS and PFC telemetry. This enables RDMA health monitoring by
// exposing per-priority pause frames, ECN marking, and queue drops.
type QoSCollector interface {
	// CollectQoSStats fetches per-queue counters (PFC, ECN, drops, depth).
	// Returns nil, nil if QoS stats are not available.
	CollectQoSStats(ctx context.Context, client gnmi.GNMIClient) ([]transform.QoSStats, error)

	// CollectPFCConfig fetches PFC configuration per interface.
	// Returns nil, nil if PFC config is not available.
	CollectPFCConfig(ctx context.Context, client gnmi.GNMIClient) ([]transform.PFCConfig, error)
}

// FallbackReporter is an optional interface for platforms that track when
// fallback paths were used instead of preferred paths. The collector surfaces
// these notes as informational warnings in the topology output.
type FallbackReporter interface {
	// CollectionNotes returns informational messages about fallback usage.
	CollectionNotes() []string

	// ResetNotes clears accumulated notes for a fresh collection cycle.
	ResetNotes()
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
