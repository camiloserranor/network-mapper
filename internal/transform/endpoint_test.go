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



