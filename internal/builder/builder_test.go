package builder

import (
	"testing"
	"time"

	"github.com/camiloserranor/network-mapper/internal/collector"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

func TestBuild_EmptyInput(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	v2 := Build(cr)

	if v2.SchemaVersion != "2.0" {
		t.Errorf("schema_version = %q, want %q", v2.SchemaVersion, "2.0")
	}
	if !v2.Metadata.CollectedAt.Equal(cr.CollectedAt) {
		t.Errorf("collected_at = %v, want %v", v2.Metadata.CollectedAt, cr.CollectedAt)
	}
	if v2.Metadata.Summary.SwitchCount != 0 {
		t.Errorf("switch_count = %d, want 0", v2.Metadata.Summary.SwitchCount)
	}
}

func TestBuild_SingleSwitch(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Now(),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device: topology.Device{
					ID:          "TOR-1",
					Type:        "switch",
					SystemName:  "tor-1.example.com",
					ChassisID:   "aa:bb:cc:dd:ee:01",
					SoftwareVersion: "10.4(5)",
				},
				Interfaces: []topology.Interface{
					{Name: "Ethernet1/1", OperStatus: "UP", Speed: "25G", MTU: 9216},
					{Name: "Ethernet1/2", OperStatus: "DOWN", Speed: "10G"},
				},
				BGPNeighbors: []transform.BGPNeighbor{
					{NeighborAddress: "10.0.0.2", PeerAS: 65001, SessionState: "ESTABLISHED"},
				},
			},
		},
	}

	v2 := Build(cr)

	if v2.Metadata.Summary.SwitchCount != 1 {
		t.Errorf("switch_count = %d, want 1", v2.Metadata.Summary.SwitchCount)
	}
	if len(v2.Fabric.Switches) != 1 {
		t.Fatalf("fabric.switches length = %d, want 1", len(v2.Fabric.Switches))
	}

	sw := v2.Fabric.Switches[0]
	if sw.ID != "TOR-1" {
		t.Errorf("switch id = %q, want %q", sw.ID, "TOR-1")
	}
	if sw.ChassisID != "aa:bb:cc:dd:ee:01" {
		t.Errorf("chassis_id = %q, want %q", sw.ChassisID, "aa:bb:cc:dd:ee:01")
	}
	if len(sw.Interfaces) != 2 {
		t.Errorf("interfaces length = %d, want 2", len(sw.Interfaces))
	}
	if len(sw.BGPSessions) != 1 {
		t.Errorf("bgp_sessions length = %d, want 1", len(sw.BGPSessions))
	}
}

func TestBuild_PeerLinks(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Now(),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device:     topology.Device{ID: "TOR-1", Type: "switch", SystemName: "tor-1"},
				Neighbors: []transform.LLDPNeighbor{
					{
						LocalPort:         "Ethernet1/53",
						SystemName:        "tor-2",
						ChassisID:         "aa:bb:cc:dd:ee:02",
						PortID:            "Ethernet1/53",
						SystemDescription: "Cisco Nexus",
						Capabilities:      "Bridge, Router",
					},
				},
				Interfaces: []topology.Interface{
					{Name: "Ethernet1/53", OperStatus: "UP", Speed: "100G", MTU: 9216},
				},
			},
			{
				SwitchName: "TOR-2",
				SwitchID:   "TOR-2",
				Device:     topology.Device{ID: "TOR-2", Type: "switch", SystemName: "tor-2"},
			},
		},
	}

	v2 := Build(cr)

	if v2.Metadata.Summary.InterSwitchLinks != 1 {
		t.Errorf("inter_switch_links = %d, want 1", v2.Metadata.Summary.InterSwitchLinks)
	}

	// Find TOR-1 and check peer_links
	var tor1 *topology.FabricSwitch
	for i := range v2.Fabric.Switches {
		if v2.Fabric.Switches[i].ID == "TOR-1" {
			tor1 = &v2.Fabric.Switches[i]
			break
		}
	}
	if tor1 == nil {
		t.Fatal("TOR-1 not found in fabric switches")
	}
	if len(tor1.PeerLinks) != 1 {
		t.Fatalf("peer_links length = %d, want 1", len(tor1.PeerLinks))
	}
	pl := tor1.PeerLinks[0]
	if pl.RemoteSwitch != "TOR-2" {
		t.Errorf("peer_link remote_switch = %q, want %q", pl.RemoteSwitch, "TOR-2")
	}
	if pl.OperStatus != "UP" {
		t.Errorf("peer_link oper_status = %q, want %q", pl.OperStatus, "UP")
	}
}

func TestBuild_HostClassification(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Now(),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device:     topology.Device{ID: "TOR-1", Type: "switch"},
				Neighbors: []transform.LLDPNeighbor{
					{
						LocalPort:    "Ethernet1/1",
						SystemName:   "host-1",
						ChassisID:    "11:22:33:44:55:66",
						PortID:       "ens1",
						Capabilities: "Station", // Station capability → classified as host
					},
				},
				Interfaces: []topology.Interface{
					{Name: "Ethernet1/1", OperStatus: "UP", Speed: "25G", MTU: 9216, Mode: "access", AccessVLAN: 100},
				},
			},
		},
	}

	v2 := Build(cr)

	if v2.Metadata.Summary.HostCount != 1 {
		t.Errorf("host_count = %d, want 1", v2.Metadata.Summary.HostCount)
	}
	if len(v2.Compute.Hosts) != 1 {
		t.Fatalf("compute.hosts length = %d, want 1", len(v2.Compute.Hosts))
	}

	host := v2.Compute.Hosts[0]
	if host.ID != "host-1" {
		t.Errorf("host id = %q, want %q", host.ID, "host-1")
	}
	if len(host.Connections) != 1 {
		t.Fatalf("host connections = %d, want 1", len(host.Connections))
	}
	if host.Connections[0].SwitchID != "TOR-1" {
		t.Errorf("connection switch_id = %q, want %q", host.Connections[0].SwitchID, "TOR-1")
	}
	if host.Connections[0].VLANMode != "access" {
		t.Errorf("connection vlan_mode = %q, want %q", host.Connections[0].VLANMode, "access")
	}
	if host.Connections[0].AccessVLAN != 100 {
		t.Errorf("connection access_vlan = %d, want 100", host.Connections[0].AccessVLAN)
	}
}

func TestBuild_VLANs(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Now(),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device:     topology.Device{ID: "TOR-1", Type: "switch"},
				VLANs: []topology.VLAN{
					{ID: 100, Name: "mgmt", MemberPorts: []string{"Ethernet1/1", "Ethernet1/2"}},
					{ID: 200, Name: "storage", MemberPorts: []string{"Ethernet1/3"}},
				},
				Interfaces: []topology.Interface{
					{Name: "Ethernet1/1", Mode: "access"},
					{Name: "Ethernet1/2", Mode: "trunk"},
					{Name: "Ethernet1/3", Mode: "access"},
				},
			},
		},
	}

	v2 := Build(cr)

	if v2.Metadata.Summary.VLANCount != 2 {
		t.Errorf("vlan_count = %d, want 2", v2.Metadata.Summary.VLANCount)
	}

	// Find VLAN 100
	var vlan100 *topology.VLANEntry
	for i := range v2.VLANs.Items {
		if v2.VLANs.Items[i].ID == 100 {
			vlan100 = &v2.VLANs.Items[i]
			break
		}
	}
	if vlan100 == nil {
		t.Fatal("VLAN 100 not found")
	}
	if len(vlan100.Switches) != 1 {
		t.Fatalf("VLAN 100 switches = %d, want 1", len(vlan100.Switches))
	}
	vs := vlan100.Switches[0]
	if vs.SwitchName != "TOR-1" {
		t.Errorf("VLAN switch_name = %q, want %q", vs.SwitchName, "TOR-1")
	}
	if len(vs.AccessPorts) != 1 {
		t.Errorf("access_ports = %d, want 1", len(vs.AccessPorts))
	}
	if len(vs.TrunkPorts) != 1 {
		t.Errorf("trunk_ports = %d, want 1", len(vs.TrunkPorts))
	}
}

func TestBuild_Summary(t *testing.T) {
	cr := &collector.CollectionResult{
		CollectedAt: time.Now(),
		Switches: []collector.SwitchData{
			{
				SwitchName: "TOR-1",
				SwitchID:   "TOR-1",
				Device:     topology.Device{ID: "TOR-1", Type: "switch"},
				Errors: []topology.PartialError{
					{Switch: "TOR-1", Phase: "mac-table", Message: "timeout"},
				},
			},
		},
	}

	v2 := Build(cr)

	s := v2.Metadata.Summary
	if s.SwitchCount != 1 {
		t.Errorf("switch_count = %d, want 1", s.SwitchCount)
	}
	if s.PartialFailures != 1 {
		t.Errorf("partial_failures = %d, want 1", s.PartialFailures)
	}
	if len(v2.Warnings) != 1 {
		t.Errorf("warnings length = %d, want 1", len(v2.Warnings))
	}
}

func TestBuildFromV1(t *testing.T) {
	v1 := &topology.Topology{
		SchemaVersion: "1.0",
		CollectedAt:   time.Now(),
		SourceSwitches: []string{"TOR-1"},
		Devices: []topology.Device{
			{ID: "TOR-1", Type: "switch", SystemName: "tor-1", ChassisID: "aa:bb:cc:00:00:01"},
			{ID: "host-1", Type: "host", SystemName: "host-1", ChassisID: "aa:bb:cc:00:00:02"},
		},
		Links: []topology.Link{
			{LocalDevice: "TOR-1", LocalPort: "Eth1/1", RemoteDevice: "host-1", RemotePort: "ens1", OperStatus: "UP"},
		},
		PartialFailures: []topology.PartialError{
			{Switch: "TOR-1", Phase: "bgp", Message: "not supported"},
		},
	}

	v2 := BuildFromV1(v1)

	if v2.SchemaVersion != "2.0" {
		t.Errorf("schema_version = %q, want %q", v2.SchemaVersion, "2.0")
	}
	if v2.Metadata.Summary.SwitchCount != 1 {
		t.Errorf("switch_count = %d, want 1", v2.Metadata.Summary.SwitchCount)
	}
	if v2.Metadata.Summary.HostCount != 1 {
		t.Errorf("host_count = %d, want 1", v2.Metadata.Summary.HostCount)
	}
	if v2.Metadata.Summary.HostLinks != 1 {
		t.Errorf("host_links = %d, want 1", v2.Metadata.Summary.HostLinks)
	}
	if len(v2.Warnings) != 1 {
		t.Errorf("warnings = %d, want 1", len(v2.Warnings))
	}
}
