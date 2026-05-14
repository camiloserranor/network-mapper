package builder

// adapter_test.go validates that the v2 TopologyV2 JSON structure produced by
// the builder contains all the fields the web UI's adaptV2() function expects.
// The JS adapter converts v2 → flat format for rendering; if any field is
// missing or mis-named the UI silently breaks. These tests catch that.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/camiloserranor/network-mapper/internal/collector"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// buildTestTopology creates a realistic v2 topology with all sections populated.
func buildTestTopology() *topology.TopologyV2 {
	cr := &collector.CollectionResult{
		CollectedAt:    time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device: topology.Device{
					ID:                "TOR-1",
					Type:              "switch",
					SystemName:        "tor-1.example.com",
					ChassisID:         "aa:bb:cc:dd:ee:01",
					ManagementAddress: "10.0.0.1",
					SoftwareVersion:   "NX-OS 10.4(5)",
					SystemDescription: "Cisco Nexus",
				},
				Neighbors: []transform.LLDPNeighbor{
					{
						LocalPort:         "Eth1/53",
						SystemName:        "tor-2.example.com",
						ChassisID:         "aa:bb:cc:dd:ee:02",
						PortID:            "Eth1/53",
						SystemDescription: "Cisco Nexus",
						Capabilities:      "Bridge, Router",
					},
					{
						LocalPort:    "Eth1/1",
						SystemName:   "host-01",
						ChassisID:    "11:22:33:44:55:01",
						PortID:       "ens1f0np0",
						Capabilities: "Station",
					},
					{
						LocalPort:    "Eth1/2",
						SystemName:   "",
						ChassisID:    "ff:ee:dd:cc:bb:01",
						PortID:       "eth0",
						Capabilities: "",
					},
				},
				Interfaces: []topology.Interface{
					{Name: "Eth1/53", OperStatus: "UP", Speed: "100G", MTU: 9216, Mode: "trunk", NativeVLAN: 1, TrunkVLANs: []int{100, 200}},
					{Name: "Eth1/1", OperStatus: "UP", Speed: "25G", MTU: 9216, Mode: "trunk", NativeVLAN: 100, TrunkVLANs: []int{100, 200, 300}},
					{Name: "Eth1/2", OperStatus: "UP", Speed: "25G", MTU: 9216, Mode: "access", AccessVLAN: 100},
				},
				VLANs: []topology.VLAN{
					{ID: 100, Name: "mgmt", MemberPorts: []string{"Eth1/1", "Eth1/2"}},
					{ID: 200, Name: "storage", MemberPorts: []string{"Eth1/1"}},
					{ID: 300, Name: "compute", MemberPorts: []string{"Eth1/1"}},
				},
				BGPNeighbors: []transform.BGPNeighbor{
					{NeighborAddress: "10.0.0.2", PeerAS: 65001, SessionState: "ESTABLISHED"},
				},
				MACEntries: []transform.MACEntry{
					{MAC: "aa:11:22:33:44:01", VLAN: 100, Port: "Eth1/1", Type: "dynamic"},
					{MAC: "aa:11:22:33:44:02", VLAN: 200, Port: "Eth1/1", Type: "dynamic"},
				},
				ARPEntries: []transform.ARPEntry{
					{IP: "10.100.0.10", MAC: "aa:11:22:33:44:01"},
					{IP: "10.200.0.10", MAC: "aa:11:22:33:44:02"},
				},
			},
			{
				SwitchName: "TOR-2",
				SwitchID:   "TOR-2",
				Device: topology.Device{
					ID:          "TOR-2",
					Type:        "switch",
					SystemName:  "tor-2.example.com",
					ChassisID:   "aa:bb:cc:dd:ee:02",
					SoftwareVersion: "NX-OS 10.4(5)",
				},
				Neighbors: []transform.LLDPNeighbor{
					{
						LocalPort:         "Eth1/53",
						SystemName:        "tor-1.example.com",
						ChassisID:         "aa:bb:cc:dd:ee:01",
						PortID:            "Eth1/53",
						SystemDescription: "Cisco Nexus",
						Capabilities:      "Bridge, Router",
					},
				},
				Interfaces: []topology.Interface{
					{Name: "Eth1/53", OperStatus: "UP", Speed: "100G", MTU: 9216},
				},
			},
		},
	}
	ToolVersion = "test"
	return Build(cr)
}

// TestAdaptV2_SwitchFields verifies that every field the JS adapter reads from
// fabric.switches is present in the JSON output.
func TestAdaptV2_SwitchFields(t *testing.T) {
	v2 := buildTestTopology()

	data, err := json.Marshal(v2)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Top-level fields the adapter expects
	for _, key := range []string{"schema_version", "metadata", "fabric", "compute", "vlans"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing top-level field %q", key)
		}
	}

	if raw["schema_version"] != "2.0" {
		t.Errorf("schema_version = %v, want %q", raw["schema_version"], "2.0")
	}

	// Metadata fields
	meta := raw["metadata"].(map[string]interface{})
	for _, key := range []string{"collected_at", "source_switches", "summary"} {
		if _, ok := meta[key]; !ok {
			t.Errorf("missing metadata field %q", key)
		}
	}

	// Fabric switches
	fabric := raw["fabric"].(map[string]interface{})
	switches := fabric["switches"].([]interface{})
	if len(switches) < 2 {
		t.Fatalf("expected at least 2 switches, got %d", len(switches))
	}

	// Find TOR-1 (the one with all the data)
	var sw map[string]interface{}
	for _, s := range switches {
		m := s.(map[string]interface{})
		if m["id"] == "TOR-1" {
			sw = m
			break
		}
	}
	if sw == nil {
		t.Fatal("TOR-1 not found in switches")
	}
	switchFields := []string{"id", "name", "chassis_id", "management_address", "software_version",
		"system_description", "interfaces", "peer_links", "connected_hosts", "bgp_sessions"}
	for _, key := range switchFields {
		if _, ok := sw[key]; !ok {
			t.Errorf("switch missing field %q", key)
		}
	}

	// Peer links fields
	peerLinks := sw["peer_links"].([]interface{})
	if len(peerLinks) == 0 {
		t.Fatal("expected peer_links to be non-empty")
	}
	pl := peerLinks[0].(map[string]interface{})
	for _, key := range []string{"local_port", "remote_switch", "remote_port", "oper_status"} {
		if _, ok := pl[key]; !ok {
			t.Errorf("peer_link missing field %q", key)
		}
	}

	// Connected hosts fields
	connHosts := sw["connected_hosts"].([]interface{})
	if len(connHosts) == 0 {
		t.Fatal("expected connected_hosts to be non-empty")
	}
	ch := connHosts[0].(map[string]interface{})
	for _, key := range []string{"port", "host_id", "oper_status"} {
		if _, ok := ch[key]; !ok {
			t.Errorf("connected_host missing field %q", key)
		}
	}

	// Interface fields
	ifaces := sw["interfaces"].([]interface{})
	if len(ifaces) == 0 {
		t.Fatal("expected interfaces to be non-empty")
	}
	iface := ifaces[0].(map[string]interface{})
	for _, key := range []string{"name", "oper_status", "speed", "mtu"} {
		if _, ok := iface[key]; !ok {
			t.Errorf("interface missing field %q", key)
		}
	}

	// BGP session fields
	bgp := sw["bgp_sessions"].([]interface{})
	if len(bgp) == 0 {
		t.Fatal("expected bgp_sessions to be non-empty")
	}
	session := bgp[0].(map[string]interface{})
	for _, key := range []string{"neighbor_address", "peer_as", "session_state"} {
		if _, ok := session[key]; !ok {
			t.Errorf("bgp_session missing field %q", key)
		}
	}
}

// TestAdaptV2_HostFields verifies that compute.hosts entries have all fields
// the JS adapter reads.
func TestAdaptV2_HostFields(t *testing.T) {
	v2 := buildTestTopology()

	data, err := json.Marshal(v2)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	compute := raw["compute"].(map[string]interface{})
	hosts := compute["hosts"].([]interface{})
	if len(hosts) == 0 {
		t.Fatal("expected at least 1 host")
	}

	host := hosts[0].(map[string]interface{})
	for _, key := range []string{"id", "chassis_id", "connections"} {
		if _, ok := host[key]; !ok {
			t.Errorf("host missing field %q", key)
		}
	}

	// Connection fields (critical for VLAN display in UI)
	conns := host["connections"].([]interface{})
	if len(conns) == 0 {
		t.Fatal("expected at least 1 host connection")
	}
	conn := conns[0].(map[string]interface{})
	for _, key := range []string{"switch_id", "switch_port", "oper_status"} {
		if _, ok := conn[key]; !ok {
			t.Errorf("host connection missing field %q", key)
		}
	}
}

// TestAdaptV2_HostVLANsFromTrunk verifies that hosts on trunk ports have
// VLAN data accessible via their connections (access_vlan, native_vlan, trunk_vlans).
func TestAdaptV2_HostVLANsFromTrunk(t *testing.T) {
	v2 := buildTestTopology()

	data, _ := json.Marshal(v2)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	compute := raw["compute"].(map[string]interface{})
	hosts := compute["hosts"].([]interface{})

	// Find host-01 which is on a trunk port
	var host01 map[string]interface{}
	for _, h := range hosts {
		hm := h.(map[string]interface{})
		if hm["id"] == "host-01" {
			host01 = hm
			break
		}
	}
	if host01 == nil {
		t.Fatal("host-01 not found in compute.hosts")
	}

	conns := host01["connections"].([]interface{})
	if len(conns) == 0 {
		t.Fatal("host-01 has no connections")
	}
	conn := conns[0].(map[string]interface{})

	// The JS adapter reads these fields to populate host VLANs
	if conn["vlan_mode"] == nil {
		t.Error("connection missing vlan_mode")
	}

	// Trunk VLANs should be present
	trunkVLANs, ok := conn["trunk_vlans"]
	if !ok {
		t.Fatal("connection missing trunk_vlans")
	}
	tvSlice := trunkVLANs.([]interface{})
	if len(tvSlice) == 0 {
		t.Error("trunk_vlans should not be empty for a trunk port")
	}
}

// TestAdaptV2_VLANFields verifies that vlans.items entries have all fields
// the JS adapter reads.
func TestAdaptV2_VLANFields(t *testing.T) {
	v2 := buildTestTopology()

	data, _ := json.Marshal(v2)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	vlans := raw["vlans"].(map[string]interface{})
	items := vlans["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("expected at least 1 VLAN item")
	}

	vlan := items[0].(map[string]interface{})
	if _, ok := vlan["id"]; !ok {
		t.Error("VLAN missing field 'id'")
	}
	if _, ok := vlan["switches"]; !ok {
		t.Error("VLAN missing field 'switches'")
	}

	// VLAN switch entry fields
	vlanSwitches := vlan["switches"].([]interface{})
	if len(vlanSwitches) == 0 {
		t.Fatal("VLAN should have at least 1 switch entry")
	}
	vs := vlanSwitches[0].(map[string]interface{})
	if _, ok := vs["switch_name"]; !ok {
		t.Error("VLAN switch entry missing 'switch_name'")
	}

	// Hosts within VLANs (used by adapter to assign VLANs to host devices)
	if hosts, ok := vlan["hosts"]; ok {
		hostSlice := hosts.([]interface{})
		if len(hostSlice) > 0 {
			vh := hostSlice[0].(map[string]interface{})
			if _, ok := vh["chassis_id"]; !ok {
				t.Error("VLAN host entry missing 'chassis_id'")
			}
		}
	}
}

// TestAdaptV2_UnknownDevices verifies unknown_devices structure.
func TestAdaptV2_UnknownDevices(t *testing.T) {
	v2 := buildTestTopology()

	data, _ := json.Marshal(v2)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	// unknown_devices may be nil if none exist — that's fine
	ud, ok := raw["unknown_devices"]
	if !ok {
		return // no unknown devices in this test data
	}
	udMap := ud.(map[string]interface{})
	items := udMap["items"].([]interface{})
	if len(items) == 0 {
		return
	}

	unk := items[0].(map[string]interface{})
	for _, key := range []string{"id", "chassis_id", "connected_to"} {
		if _, ok := unk[key]; !ok {
			t.Errorf("unknown device missing field %q", key)
		}
	}
}

// TestAdaptV2_UnattributedEndpoints verifies the unattributed_endpoints structure.
func TestAdaptV2_UnattributedEndpoints(t *testing.T) {
	v2 := buildTestTopology()

	data, _ := json.Marshal(v2)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	compute := raw["compute"].(map[string]interface{})
	uep, ok := compute["unattributed_endpoints"]
	if !ok {
		return // optional field
	}
	uepMap := uep.(map[string]interface{})
	items := uepMap["items"].([]interface{})
	if len(items) == 0 {
		return
	}
	ep := items[0].(map[string]interface{})
	for _, key := range []string{"mac", "learned_on_switch", "learned_on_port"} {
		if _, ok := ep[key]; !ok {
			t.Errorf("unattributed endpoint missing field %q", key)
		}
	}
}

// TestAdaptV2_Warnings verifies warnings are present in the output.
func TestAdaptV2_Warnings(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Now(),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device:     topology.Device{ID: "TOR-1", Type: "switch"},
				Errors:     []topology.PartialError{{Switch: "TOR-1", Phase: "bgp", Message: "timeout"}},
			},
		},
	}

	ToolVersion = "test"
	v2 := Build(cr)

	data, _ := json.Marshal(v2)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	warnings := raw["warnings"].([]interface{})
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	w := warnings[0].(map[string]interface{})
	for _, key := range []string{"switch", "phase", "message"} {
		if _, ok := w[key]; !ok {
			t.Errorf("warning missing field %q", key)
		}
	}
}

// TestAdaptV2_EndToEnd_JSONRoundTrip verifies the full v2 JSON can be
// marshalled and unmarshalled back into TopologyV2 without data loss.
func TestAdaptV2_EndToEnd_JSONRoundTrip(t *testing.T) {
	v2 := buildTestTopology()

	data, err := json.MarshalIndent(v2, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var v2rt topology.TopologyV2
	if err := json.Unmarshal(data, &v2rt); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if v2rt.SchemaVersion != v2.SchemaVersion {
		t.Errorf("schema_version roundtrip: %q != %q", v2rt.SchemaVersion, v2.SchemaVersion)
	}
	if v2rt.Metadata.Summary.SwitchCount != v2.Metadata.Summary.SwitchCount {
		t.Errorf("switch_count roundtrip: %d != %d", v2rt.Metadata.Summary.SwitchCount, v2.Metadata.Summary.SwitchCount)
	}
	if len(v2rt.Fabric.Switches) != len(v2.Fabric.Switches) {
		t.Errorf("fabric.switches length roundtrip: %d != %d", len(v2rt.Fabric.Switches), len(v2.Fabric.Switches))
	}
	if len(v2rt.Compute.Hosts) != len(v2.Compute.Hosts) {
		t.Errorf("compute.hosts length roundtrip: %d != %d", len(v2rt.Compute.Hosts), len(v2.Compute.Hosts))
	}
	if len(v2rt.VLANs.Items) != len(v2.VLANs.Items) {
		t.Errorf("vlans.items length roundtrip: %d != %d", len(v2rt.VLANs.Items), len(v2.VLANs.Items))
	}
}

// TestAdaptV2_SummaryCountsMatchData verifies the summary counts in metadata
// actually match the data arrays.
func TestAdaptV2_SummaryCountsMatchData(t *testing.T) {
	v2 := buildTestTopology()

	s := v2.Metadata.Summary
	switchCount := len(v2.Fabric.Switches)
	hostCount := len(v2.Compute.Hosts)
	vlanCount := len(v2.VLANs.Items)

	if s.SwitchCount != switchCount {
		t.Errorf("summary.switch_count=%d but fabric.switches has %d", s.SwitchCount, switchCount)
	}
	if s.HostCount != hostCount {
		t.Errorf("summary.host_count=%d but compute.hosts has %d", s.HostCount, hostCount)
	}
	if s.VLANCount != vlanCount {
		t.Errorf("summary.vlan_count=%d but vlans.items has %d", s.VLANCount, vlanCount)
	}

	// Count total links from switches
	peerLinkCount := 0
	hostLinkCount := 0
	for _, sw := range v2.Fabric.Switches {
		peerLinkCount += len(sw.PeerLinks)
		hostLinkCount += len(sw.ConnectedHosts)
	}
	if s.InterSwitchLinks != peerLinkCount {
		t.Errorf("summary.inter_switch_links=%d but counted %d peer_links", s.InterSwitchLinks, peerLinkCount)
	}
	if s.HostLinks != hostLinkCount {
		t.Errorf("summary.host_links=%d but counted %d connected_hosts", s.HostLinks, hostLinkCount)
	}
}
