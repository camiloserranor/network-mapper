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
	BGPSessions       []BGPSession      `json:"bgp_sessions,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

// BGPSession holds the state of a BGP peering session on a device.
type BGPSession struct {
	NeighborAddress        string `json:"neighbor_address"`
	PeerAS                 uint32 `json:"peer_as"`
	LocalAS                uint32 `json:"local_as,omitempty"`
	PeerType               string `json:"peer_type,omitempty"`    // INTERNAL, EXTERNAL
	Description            string `json:"description,omitempty"`
	SessionState           string `json:"session_state"`          // ESTABLISHED, IDLE, ACTIVE, etc.
	Enabled                bool   `json:"enabled"`
	EstablishedTransitions uint64 `json:"established_transitions,omitempty"`
	LastEstablished        string `json:"last_established,omitempty"`
	VRFName                string `json:"vrf_name,omitempty"`
	MessagesReceived       int64  `json:"messages_received,omitempty"`
	MessagesSent           int64  `json:"messages_sent,omitempty"`
	PrefixesReceived       int64  `json:"prefixes_received,omitempty"`
	PrefixesSent           int64  `json:"prefixes_sent,omitempty"`
}

// Interface represents a network interface on a device.
type Interface struct {
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`   // port description (often identifies connected device)
	OperStatus    string          `json:"oper_status,omitempty"`    // UP, DOWN
	Speed         string          `json:"speed,omitempty"`          // 1G, 10G, 25G, 100G
	MTU           int             `json:"mtu,omitempty"`
	Mode          string          `json:"mode,omitempty"`           // access, trunk, routed
	AccessVLAN    int             `json:"access_vlan,omitempty"`    // configured access VLAN
	NativeVLAN    int             `json:"native_vlan,omitempty"`    // configured native VLAN (trunk)
	TrunkVLANs    []int           `json:"trunk_vlans,omitempty"`    // configured trunk allowed VLANs
	ObservedVLANs []int           `json:"observed_vlans,omitempty"` // VLANs seen in MAC table traffic
	Counters      *IfaceCounters  `json:"counters,omitempty"`
}

// IfaceCounters holds interface traffic counters.
type IfaceCounters struct {
	InOctets         uint64 `json:"in_octets"`
	OutOctets        uint64 `json:"out_octets"`
	InPkts           uint64 `json:"in_pkts"`
	OutPkts          uint64 `json:"out_pkts"`
	InErrors         uint64 `json:"in_errors"`
	OutErrors        uint64 `json:"out_errors"`
	InDiscards       uint64 `json:"in_discards"`
	OutDiscards      uint64 `json:"out_discards"`
	InMulticastPkts  uint64 `json:"in_multicast_pkts,omitempty"`
	OutMulticastPkts uint64 `json:"out_multicast_pkts,omitempty"`
	InBroadcastPkts  uint64 `json:"in_broadcast_pkts,omitempty"`
	OutBroadcastPkts uint64 `json:"out_broadcast_pkts,omitempty"`
	CRCErrors        uint64 `json:"crc_errors,omitempty"`
	PauseFramesIn    uint64 `json:"pause_frames_in,omitempty"`
	PauseFramesOut   uint64 `json:"pause_frames_out,omitempty"`
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
	VTEPIP     string   `json:"vtep_ip,omitempty"`      // VTEP peer IP (from L2RIB, VXLAN only)
	VNI        int      `json:"vni,omitempty"`          // VNI where MAC was learned (VXLAN only)
}
