package collector

import (
	"time"

	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// CollectionResult holds the raw per-switch data gathered by the gNMI
// collector. It is the output of CollectRaw() and the input to the builder
// package, which transforms it into a TopologyV2.
//
// This struct can be serialized to JSON for test fixtures or offline
// analysis, enabling the build step to be tested without a live switch.
type CollectionResult struct {
	CollectedAt time.Time    `json:"collected_at"`
	Switches    []SwitchData `json:"switches"`
}

// SwitchData contains all gNMI data collected from a single TOR switch.
type SwitchData struct {
	// SwitchName is the config-level name (e.g., "TOR-1").
	SwitchName string `json:"switch_name"`

	// SwitchID is the unique identifier used to reference this switch in
	// links and device maps. Usually the same as SwitchName.
	SwitchID string `json:"switch_id"`

	// Device is the switch's own identity (system name, description, etc).
	Device topology.Device `json:"device"`

	// Neighbors are LLDP neighbors discovered on this switch.
	Neighbors []transform.LLDPNeighbor `json:"neighbors,omitempty"`

	// Interfaces are the parsed interface states (name, status, speed, VLAN config).
	Interfaces []topology.Interface `json:"interfaces,omitempty"`

	// MACEntries are the MAC address table entries (for endpoint discovery).
	MACEntries []transform.MACEntry `json:"mac_entries,omitempty"`

	// ARPEntries are the ARP table entries (for IP-to-MAC mapping).
	ARPEntries []transform.ARPEntry `json:"arp_entries,omitempty"`

	// VLANs are the VLAN configurations on this switch.
	VLANs []topology.VLAN `json:"vlans,omitempty"`

	// BGPNeighbors are the BGP peering sessions on this switch.
	BGPNeighbors []transform.BGPNeighbor `json:"bgp_neighbors,omitempty"`

	// Errors are non-fatal errors encountered during collection.
	Errors []topology.PartialError `json:"errors,omitempty"`
}
