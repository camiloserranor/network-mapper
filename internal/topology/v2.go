// Package topology defines the data model for network topology output.
//
// This file contains the v2 hierarchical schema. The v2 schema organizes
// topology data into logical sections (fabric, compute, vlans) rather than
// flat device/link arrays, making it useful for both human inspection and
// programmatic analysis.
package topology

import "time"

// TopologyV2 is the top-level output produced by the v2 collection pipeline.
// It organizes the network into fabric (switches), compute (hosts), and
// network-wide VLAN views.
type TopologyV2 struct {
	SchemaVersion  string            `json:"schema_version"`
	Metadata       Metadata          `json:"metadata"`
	Fabric         Fabric            `json:"fabric"`
	Compute        Compute           `json:"compute"`
	VLANs          VLANMap           `json:"vlans"`
	UnknownDevices *UnknownDeviceSet `json:"unknown_devices,omitempty"`
	Warnings       []PartialError    `json:"warnings,omitempty"`
}

// Metadata contains collection context and summary statistics.
type Metadata struct {
	CollectedAt    time.Time `json:"collected_at"`
	Tool           string    `json:"tool"`
	ToolVersion    string    `json:"tool_version"`
	SourceSwitches []string  `json:"source_switches"`
	Summary        Summary   `json:"summary"`
}

// Summary provides at-a-glance counts for the topology.
type Summary struct {
	SwitchCount              int `json:"switch_count"`
	HostCount                int `json:"host_count"`
	EndpointCount            int `json:"endpoint_count"`
	UnknownDeviceCount       int `json:"unknown_device_count"`
	TotalLinks               int `json:"total_links"`
	InterSwitchLinks         int `json:"inter_switch_links"`
	HostLinks                int `json:"host_links"`
	VLANCount                int `json:"vlan_count"`
	PartialFailures          int `json:"partial_failures"`
	AttributedEndpoints      int `json:"attributed_endpoints"`
	UnattributedEndpoints    int `json:"unattributed_endpoints"`
}

// Fabric represents the network fabric layer: TOR switches, their interfaces,
// BGP sessions, and inter-switch links.
type Fabric struct {
	Switches []FabricSwitch `json:"switches"`
}

// FabricSwitch is a fully described switch in the fabric.
type FabricSwitch struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	ChassisID         string            `json:"chassis_id,omitempty"`
	ManagementAddress string            `json:"management_address,omitempty"`
	SoftwareVersion   string            `json:"software_version,omitempty"`
	SystemDescription string            `json:"system_description,omitempty"`
	Uptime            string            `json:"uptime,omitempty"`
	Health            *SwitchHealth     `json:"health,omitempty"`
	Interfaces        []Interface       `json:"interfaces"`
	BGPSessions       []BGPSession      `json:"bgp_sessions,omitempty"`
	PeerLinks         []PeerLink        `json:"peer_links,omitempty"`
	ConnectedHosts    []ConnectedHost   `json:"connected_hosts,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

// SwitchHealth holds resource utilization metrics for a switch.
type SwitchHealth struct {
	CPUUtilizationPct float64 `json:"cpu_utilization_pct,omitempty"`
	MemoryUsedBytes   uint64  `json:"memory_used_bytes,omitempty"`
	MemoryTotalBytes  uint64  `json:"memory_total_bytes,omitempty"`
}

// PeerLink represents an inter-switch connection.
type PeerLink struct {
	LocalPort    string `json:"local_port"`
	RemoteSwitch string `json:"remote_switch"`
	RemotePort   string `json:"remote_port"`
	OperStatus   string `json:"oper_status,omitempty"`
	Speed        string `json:"speed,omitempty"`
	MTU          string `json:"mtu,omitempty"`
}

// ConnectedHost describes a host attached to a switch port.
type ConnectedHost struct {
	Port        string `json:"port"`
	HostID      string `json:"host_id"`
	HostMgmtIP  string `json:"host_mgmt_ip,omitempty"`
	OperStatus  string `json:"oper_status,omitempty"`
	MTU         string `json:"mtu,omitempty"`
}

// Compute represents the compute layer: physical hosts and their virtual
// endpoints (VMs).
type Compute struct {
	Hosts                    []ComputeHost           `json:"hosts"`
	UnattributedEndpoints    *UnattributedEndpointSet `json:"unattributed_endpoints,omitempty"`
}

// ComputeHost is a physical host with its switch connections and endpoints.
type ComputeHost struct {
	ID                   string            `json:"id"`
	ChassisID            string            `json:"chassis_id,omitempty"`
	Name                 string            `json:"name,omitempty"`
	ManagementAddress    string            `json:"management_address,omitempty"`
	ClassificationSource string            `json:"classification_source,omitempty"`
	Connections          []HostConnection  `json:"connections,omitempty"`
	Endpoints            []HostEndpoint    `json:"endpoints,omitempty"`
	Annotations          map[string]string `json:"annotations,omitempty"`
}

// HostConnection describes how a host connects to a switch.
type HostConnection struct {
	SwitchName string `json:"switch_name"`
	SwitchID   string `json:"switch_id"`
	SwitchPort string `json:"switch_port"`
	OperStatus string `json:"oper_status,omitempty"`
	Speed      string `json:"speed,omitempty"`
	MTU        string `json:"mtu,omitempty"`
	VLANMode   string `json:"vlan_mode,omitempty"`
	AccessVLAN int    `json:"access_vlan,omitempty"`
	NativeVLAN int    `json:"native_vlan,omitempty"`
	TrunkVLANs []int  `json:"trunk_vlans,omitempty"`
}

// HostEndpoint is a VM or virtual endpoint attributed to a specific host.
type HostEndpoint struct {
	MAC             string   `json:"mac"`
	IPs             []string `json:"ips,omitempty"`
	VLANs           []int    `json:"vlans"`
	Type            string   `json:"type"`
	LearnedOnSwitch string   `json:"learned_on_switch"`
	LearnedOnPort   string   `json:"learned_on_port"`
}

// UnattributedEndpointSet holds endpoints that could not be mapped to a
// specific host (e.g., NVE-learned VMs).
type UnattributedEndpointSet struct {
	Count int            `json:"count"`
	Items []HostEndpoint `json:"items"`
}

// VLANMap provides a network-wide view of VLANs: which switches carry
// each VLAN, on which ports, and which hosts are members.
type VLANMap struct {
	Items []VLANEntry `json:"items"`
}

// VLANEntry describes a single VLAN across the fabric.
type VLANEntry struct {
	ID       int             `json:"id"`
	Switches []VLANSwitch    `json:"switches"`
	Hosts    []VLANHost      `json:"hosts,omitempty"`
}

// VLANSwitch shows which ports on a switch carry a given VLAN.
type VLANSwitch struct {
	SwitchName  string   `json:"switch_name"`
	AccessPorts []string `json:"access_ports,omitempty"`
	TrunkPorts  []string `json:"trunk_ports,omitempty"`
}

// VLANHost identifies a host that is a member of a VLAN.
type VLANHost struct {
	ChassisID    string `json:"chassis_id"`
	ManagementIP string `json:"management_ip,omitempty"`
	SwitchPort   string `json:"switch_port"`
}

// UnknownDeviceSet holds devices discovered via LLDP or MAC tables that
// could not be classified as switch or host.
type UnknownDeviceSet struct {
	Items []UnknownDevice `json:"items"`
}

// UnknownDevice is an unclassified network device.
type UnknownDevice struct {
	ID                string            `json:"id"`
	ChassisID         string            `json:"chassis_id,omitempty"`
	ManagementAddress string            `json:"management_address,omitempty"`
	SystemDescription string            `json:"system_description,omitempty"`
	ConnectedTo       []DeviceAttachment `json:"connected_to,omitempty"`
}

// DeviceAttachment identifies which switch port an unknown device is attached to.
type DeviceAttachment struct {
	Switch     string `json:"switch"`
	Port       string `json:"port"`
	OperStatus string `json:"oper_status,omitempty"`
	MTU        string `json:"mtu,omitempty"`
}
