package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestCorrelateEndpoints_Basic(t *testing.T) {
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/1", ChassisID: "aa:bb:cc:00:00:01"},
			},
			MACEntries: []MACEntry{
				{MAC: "de:ad:be:ef:00:01", VLAN: 100, Port: "Eth1/1", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "10.0.1.100", MAC: "de:ad:be:ef:00:01", SwitchID: "TOR-1"},
			},
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	if len(endpoints) != 1 {
		t.Fatalf("got %d endpoints, want 1", len(endpoints))
	}
	ep := endpoints[0]
	if ep.HostDevice != "aa:bb:cc:00:00:01" {
		t.Errorf("HostDevice = %q, want 'aa:bb:cc:00:00:01'", ep.HostDevice)
	}
	if ep.SwitchID != "TOR-1" {
		t.Errorf("SwitchID = %q, want 'TOR-1'", ep.SwitchID)
	}
	if len(ep.IPs) != 1 || ep.IPs[0] != "10.0.1.100" {
		t.Errorf("IPs = %v, want [10.0.1.100]", ep.IPs)
	}
}

func TestCorrelateEndpoints_PeerLinkUpgrade(t *testing.T) {
	// Simulates Azure Local dual-homing:
	// - TOR-1 learns VM MAC on "po1" (peer-link to TOR-4), no LLDP neighbor on po1
	// - TOR-4 learns the SAME VM MAC on "Eth1/48" (physical port with LLDP to host)
	// The correlator should upgrade the endpoint to use TOR-4's host association.
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/1", ChassisID: "aa:bb:cc:00:00:01", SystemName: "HOST-01"},
				// No LLDP neighbor on po1 (it's the peer-link)
			},
			MACEntries: []MACEntry{
				// VM learned on peer-link port (po1) - no LLDP host here
				{MAC: "00:15:5d:24:7e:99", VLAN: 1007, Port: "po1", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "100.68.248.200", MAC: "00:15:5d:24:7e:99", SwitchID: "TOR-1"},
			},
		},
		{
			SwitchID: "TOR-4",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/48", ChassisID: "94:6d:ae:ef:bf:c1", SystemName: "HOST-48"},
			},
			MACEntries: []MACEntry{
				// Same VM learned on physical port with LLDP neighbor
				{MAC: "00:15:5d:24:7e:99", VLAN: 1007, Port: "Eth1/48", SwitchID: "TOR-4"},
			},
			ARPEntries: []ARPEntry{},
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	// Find our VM
	var vm *topology.Endpoint
	for i := range endpoints {
		if endpoints[i].MAC == "00:15:5d:24:7e:99" {
			vm = &endpoints[i]
			break
		}
	}
	if vm == nil {
		t.Fatal("VM 00:15:5d:24:7e:99 not found in endpoints")
	}

	// The endpoint should be upgraded to use TOR-4's host association
	if vm.HostDevice != "HOST-48" {
		t.Errorf("HostDevice = %q, want 'HOST-48' (should upgrade from peer-link to physical port)", vm.HostDevice)
	}
	if vm.SwitchID != "TOR-4" {
		t.Errorf("SwitchID = %q, want 'TOR-4'", vm.SwitchID)
	}
	if vm.HostPort != "Eth1/48" {
		t.Errorf("HostPort = %q, want 'Eth1/48'", vm.HostPort)
	}
	if len(vm.IPs) != 1 || vm.IPs[0] != "100.68.248.200" {
		t.Errorf("IPs = %v, want [100.68.248.200]", vm.IPs)
	}
}

func TestCorrelateEndpoints_PeerLinkUpgradeReverseOrder(t *testing.T) {
	// Same as above but TOR-4 (with host) is processed FIRST.
	// The endpoint should already have correct host_device from the start.
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-4",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/48", ChassisID: "94:6d:ae:ef:bf:c1", SystemName: "HOST-48"},
			},
			MACEntries: []MACEntry{
				{MAC: "00:15:5d:24:7e:99", VLAN: 1007, Port: "Eth1/48", SwitchID: "TOR-4"},
			},
			ARPEntries: []ARPEntry{
				{IP: "100.68.248.200", MAC: "00:15:5d:24:7e:99", SwitchID: "TOR-4"},
			},
		},
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/1", ChassisID: "aa:bb:cc:00:00:01", SystemName: "HOST-01"},
			},
			MACEntries: []MACEntry{
				{MAC: "00:15:5d:24:7e:99", VLAN: 1007, Port: "po1", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{},
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	var vm *topology.Endpoint
	for i := range endpoints {
		if endpoints[i].MAC == "00:15:5d:24:7e:99" {
			vm = &endpoints[i]
			break
		}
	}
	if vm == nil {
		t.Fatal("VM not found")
	}

	// Should retain original correct association (not downgrade to po1)
	if vm.HostDevice != "HOST-48" {
		t.Errorf("HostDevice = %q, want 'HOST-48'", vm.HostDevice)
	}
	if vm.SwitchID != "TOR-4" {
		t.Errorf("SwitchID = %q, want 'TOR-4'", vm.SwitchID)
	}
}

func TestCorrelateEndpoints_PortChannelResolution(t *testing.T) {
	// W-001 fix: MAC learned on port-channel should be resolved via LAG membership
	// to the physical member port's LLDP neighbor.
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/49", ChassisID: "94:6d:ae:aa:bb:01", SystemName: "HOST-01"},
				{LocalPort: "Eth1/50", ChassisID: "94:6d:ae:aa:bb:01", SystemName: "HOST-01"},
			},
			MACEntries: []MACEntry{
				// VM MAC learned on port-channel (the aggregate interface)
				{MAC: "00:15:5d:11:22:33", VLAN: 100, Port: "po501", SwitchID: "TOR-1"},
				// Host chassis MAC also on port-channel — should be filtered
				{MAC: "94:6d:ae:aa:bb:01", VLAN: 100, Port: "po501", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "10.0.1.50", MAC: "00:15:5d:11:22:33", SwitchID: "TOR-1"},
			},
			LAGMembership: LAGMembership{
				"po501": {"Eth1/49", "Eth1/50"},
			},
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	// The host MAC should NOT appear as an endpoint
	for _, ep := range endpoints {
		if ep.MAC == "94:6d:ae:aa:bb:01" {
			t.Error("Host chassis MAC should have been filtered by LAG resolution")
		}
	}

	// The VM MAC should be attributed to HOST-01 via port-channel → member port resolution
	var vm *topology.Endpoint
	for i := range endpoints {
		if endpoints[i].MAC == "00:15:5d:11:22:33" {
			vm = &endpoints[i]
			break
		}
	}
	if vm == nil {
		t.Fatal("VM endpoint not found")
	}
	if vm.HostDevice != "HOST-01" {
		t.Errorf("HostDevice = %q, want 'HOST-01'", vm.HostDevice)
	}
	if vm.HostPort != "Eth1/49" {
		t.Errorf("HostPort = %q, want 'Eth1/49' (resolved from po501)", vm.HostPort)
	}
	if len(vm.IPs) != 1 || vm.IPs[0] != "10.0.1.50" {
		t.Errorf("IPs = %v, want [10.0.1.50]", vm.IPs)
	}
}

func TestCorrelateEndpoints_PortChannelWithoutLAGData(t *testing.T) {
	// Without LAG membership data, port-channel MACs should still appear
	// as endpoints (just without host attribution — same as before the fix).
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/49", ChassisID: "94:6d:ae:aa:bb:01", SystemName: "HOST-01"},
			},
			MACEntries: []MACEntry{
				{MAC: "00:15:5d:44:55:66", VLAN: 200, Port: "po501", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{},
			// No LAGMembership — simulates switches that don't support the path
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	var vm *topology.Endpoint
	for i := range endpoints {
		if endpoints[i].MAC == "00:15:5d:44:55:66" {
			vm = &endpoints[i]
			break
		}
	}
	if vm == nil {
		t.Fatal("VM endpoint not found")
	}
	// Without LAG data, the endpoint should exist but have no host attribution
	if vm.HostDevice != "" {
		t.Errorf("HostDevice = %q, want '' (no LAG data to resolve)", vm.HostDevice)
	}
}

func TestCorrelateEndpoints_InfraPortFiltering(t *testing.T) {
	// W-002 fix: MACs learned on infrastructure ports should be excluded.
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/1", ChassisID: "94:6d:ae:aa:bb:01", SystemName: "HOST-01"},
			},
			MACEntries: []MACEntry{
				// Valid VM on a physical port
				{MAC: "00:15:5d:11:22:33", VLAN: 100, Port: "Eth1/1", SwitchID: "TOR-1"},
				// Switch self-traffic on supervisor port (should be filtered)
				{MAC: "aa:bb:cc:dd:ee:f0", VLAN: 100, Port: "sup-eth1", SwitchID: "TOR-1"},
				// Same on mgmt port (should be filtered)
				{MAC: "aa:bb:cc:dd:ee:f1", VLAN: 1, Port: "mgmt0", SwitchID: "TOR-1"},
				// Same on loopback (should be filtered)
				{MAC: "aa:bb:cc:dd:ee:f2", VLAN: 100, Port: "lo0", SwitchID: "TOR-1"},
				// Same on SVI (should be filtered)
				{MAC: "aa:bb:cc:dd:ee:f3", VLAN: 100, Port: "Vlan100", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{},
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	// Only the valid VM MAC on Eth1/1 should appear
	if len(endpoints) != 1 {
		var macs []string
		for _, ep := range endpoints {
			macs = append(macs, ep.MAC+"@"+ep.HostPort)
		}
		t.Fatalf("expected 1 endpoint, got %d: %v", len(endpoints), macs)
	}
	if endpoints[0].MAC != "00:15:5d:11:22:33" {
		t.Errorf("unexpected endpoint MAC = %q", endpoints[0].MAC)
	}
}

func TestIsInfraPort(t *testing.T) {
	tests := []struct {
		port string
		want bool
	}{
		{"sup-eth1", true},
		{"supeth1", true},
		{"Sup-Eth1", true},
		{"mgmt0", true},
		{"Mgmt0", true},
		{"lo0", true},
		{"Loopback0", true},  // "loopback" starts with "lo"
		{"Vlan100", true},
		{"vlan1", true},
		{"Eth1/1", false},
		{"po501", false},
		{"nve1", false},
	}

	for _, tt := range tests {
		got := isInfraPort(tt.port)
		if got != tt.want {
			t.Errorf("isInfraPort(%q) = %v, want %v", tt.port, got, tt.want)
		}
	}
}
