// Package topology defines the data model for network topology output.
package topology

import "time"

// Topology is the top-level output produced by a collection run.
type Topology struct {
	SchemaVersion   string         `json:"schema_version"`
	CollectedAt     time.Time      `json:"collected_at"`
	SourceSwitches  []string       `json:"source_switches"`
	PartialFailures []PartialError `json:"partial_failures"`
	Devices         []Device       `json:"devices"`
	Links           []Link         `json:"links"`
	VLANs           []VLAN         `json:"vlans,omitempty"`
	Endpoints       []Endpoint     `json:"endpoints,omitempty"`
}

// Device represents a discovered network device (switch, host, BMC, etc).
type Device struct {
	ID                string            `json:"id"`
	Type              string            `json:"type"` // switch, host, bmc, unknown
	ChassisID         string            `json:"chassis_id,omitempty"`
	SystemName        string            `json:"system_name,omitempty"`
	SystemDescription string            `json:"system_description,omitempty"`
	ManagementAddress string            `json:"management_address,omitempty"`
	SoftwareVersion   string            `json:"software_version,omitempty"`
	Uptime            string            `json:"uptime,omitempty"`
	CPUUtilization    float64           `json:"cpu_utilization,omitempty"`    // percentage (0-100)
	MemoryUsed        uint64            `json:"memory_used,omitempty"`       // bytes
	MemoryTotal       uint64            `json:"memory_total,omitempty"`      // bytes
	VLANs             []int             `json:"vlans,omitempty"`             // VLAN IDs this device participates in
	Interfaces        []Interface       `json:"interfaces,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

// Interface represents a network interface on a device.
type Interface struct {
	Name       string          `json:"name"`
	OperStatus string          `json:"oper_status,omitempty"` // UP, DOWN
	Speed      string          `json:"speed,omitempty"`       // 1G, 10G, 25G, 100G
	MTU        int             `json:"mtu,omitempty"`
	Counters   *IfaceCounters  `json:"counters,omitempty"`
}

// IfaceCounters holds interface traffic counters.
type IfaceCounters struct {
	InOctets    uint64 `json:"in_octets"`
	OutOctets   uint64 `json:"out_octets"`
	InPkts      uint64 `json:"in_pkts"`
	OutPkts     uint64 `json:"out_pkts"`
	InErrors    uint64 `json:"in_errors"`
	OutErrors   uint64 `json:"out_errors"`
	InDiscards  uint64 `json:"in_discards"`
	OutDiscards uint64 `json:"out_discards"`
}

// Link represents a discovered connection between two devices.
type Link struct {
	LocalDevice    string         `json:"local_device"`
	LocalPort      string         `json:"local_port"`
	RemoteDevice   string         `json:"remote_device"`
	RemotePort     string         `json:"remote_port"`
	RemoteChassisID string        `json:"remote_chassis_id,omitempty"`
	Source         string         `json:"source"`       // lldp
	DiscoveredAt   time.Time      `json:"discovered_at"`
	OperStatus     string         `json:"oper_status,omitempty"`
	Speed          string         `json:"speed,omitempty"`
	MTU            string         `json:"mtu,omitempty"`
	Counters       *IfaceCounters `json:"counters,omitempty"`
}

// PartialError records a non-fatal error during collection.
type PartialError struct {
	Switch  string `json:"switch"`
	Phase   string `json:"phase"`   // connect, lldp, interfaces, system
	Message string `json:"message"`
}

// VLAN represents a discovered VLAN on the network.
type VLAN struct {
	ID          int      `json:"id"`
	Name        string   `json:"name,omitempty"`
	Status      string   `json:"status,omitempty"`
	Gateway     string   `json:"gateway,omitempty"`      // SVI IP address
	MemberPorts []string `json:"member_ports,omitempty"` // switch interfaces in this VLAN
	SourceSwitch string  `json:"source_switch,omitempty"`
}

// Endpoint represents a discovered VM or virtual endpoint behind a physical host.
type Endpoint struct {
	MAC        string   `json:"mac"`
	IPs        []string `json:"ips,omitempty"`
	VLANs      []int    `json:"vlans"`
	HostPort   string   `json:"host_port"`              // switch port where MAC was learned
	HostDevice string   `json:"host_device,omitempty"`  // parent host device ID (from LLDP)
	SwitchID   string   `json:"switch_id"`              // switch that reported this MAC
	Type       string   `json:"type"`                   // vm, container, floating, unknown
}
