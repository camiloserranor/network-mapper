package collector

import (
	"testing"
	"time"

	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

func TestBuildTopology_DeduplicatesSwitchBySystemName(t *testing.T) {
	now := time.Now()

	// Simulate two queried switches: TOR-1 (FQDN: sw1.example.com) and
	// TOR-2 (FQDN: sw2.example.com). Each sees the other via LLDP plus
	// a host neighbor.
	results := []switchResult{
		{
			SwitchName: "TOR-1",
			SwitchID:   "TOR-1",
			Device: topology.Device{
				ID:         "TOR-1",
				Type:       "switch",
				SystemName: "sw1.example.com",
			},
			Neighbors: []transform.LLDPNeighbor{
				{
					LocalPort:  "Eth1/49",
					SystemName: "sw2.example.com", // FQDN of TOR-2
					ChassisID:  "aaaa.bbbb.cccc",
					PortID:     "Ethernet1/49",
				},
				{
					LocalPort: "Eth1/1",
					ChassisID: "dddd.eeee.ffff",
					PortID:    "dddd.eeee.ffff",
				},
			},
		},
		{
			SwitchName: "TOR-2",
			SwitchID:   "TOR-2",
			Device: topology.Device{
				ID:         "TOR-2",
				Type:       "switch",
				SystemName: "sw2.example.com",
			},
			Neighbors: []transform.LLDPNeighbor{
				{
					LocalPort:  "Eth1/49",
					SystemName: "sw1.example.com", // FQDN of TOR-1
					ChassisID:  "1111.2222.3333",
					PortID:     "Ethernet1/49",
				},
				{
					LocalPort: "Eth1/1",
					ChassisID: "aaaa.1111.2222",
					PortID:    "aaaa.1111.2222",
				},
			},
		},
	}

	topo := buildTopology(results, now, false)

	// Count device types
	devicesByID := map[string]topology.Device{}
	for _, d := range topo.Devices {
		devicesByID[d.ID] = d
	}

	// There should be exactly 4 devices: TOR-1, TOR-2, and 2 host MACs.
	// NOT 6 (which would happen if sw1.example.com and sw2.example.com
	// were created as separate devices).
	if len(topo.Devices) != 4 {
		t.Errorf("expected 4 devices, got %d", len(topo.Devices))
		for _, d := range topo.Devices {
			t.Logf("  device: ID=%s Type=%s SystemName=%s", d.ID, d.Type, d.SystemName)
		}
	}

	// The FQDN names should NOT appear as device IDs
	if _, exists := devicesByID["sw1.example.com"]; exists {
		t.Error("sw1.example.com should not be a separate device — should be merged into TOR-1")
	}
	if _, exists := devicesByID["sw2.example.com"]; exists {
		t.Error("sw2.example.com should not be a separate device — should be merged into TOR-2")
	}

	// TOR-1 should have chassis ID filled in from TOR-2's LLDP discovery
	if tor1, ok := devicesByID["TOR-1"]; ok {
		if tor1.ChassisID != "1111.2222.3333" {
			t.Errorf("TOR-1 chassis_id should be 1111.2222.3333 (from LLDP), got %q", tor1.ChassisID)
		}
	} else {
		t.Error("TOR-1 not found in devices")
	}

	// Links should reference TOR-1/TOR-2, not the FQDNs
	for _, link := range topo.Links {
		if link.RemoteDevice == "sw1.example.com" || link.RemoteDevice == "sw2.example.com" {
			t.Errorf("link remote_device should use config ID, not FQDN: %s → %s",
				link.LocalDevice, link.RemoteDevice)
		}
	}
}

func TestBuildTopology_NonQueriedSwitchKeepsFQDN(t *testing.T) {
	now := time.Now()

	// TOR-1 sees a switch (spine.example.com) that is NOT in our config.
	// It should remain as a separate device with its FQDN as the ID.
	results := []switchResult{
		{
			SwitchName: "TOR-1",
			SwitchID:   "TOR-1",
			Device: topology.Device{
				ID:         "TOR-1",
				Type:       "switch",
				SystemName: "leaf1.example.com",
			},
			Neighbors: []transform.LLDPNeighbor{
				{
					LocalPort:         "Eth1/53",
					SystemName:        "spine.example.com",
					ChassisID:         "aaaa.bbbb.cccc",
					PortID:            "Ethernet1/11",
					SystemDescription: "Cisco NX-OS spine",
				},
			},
		},
	}

	topo := buildTopology(results, now, false)

	devicesByID := map[string]topology.Device{}
	for _, d := range topo.Devices {
		devicesByID[d.ID] = d
	}

	if len(topo.Devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(topo.Devices))
	}
	if _, ok := devicesByID["spine.example.com"]; !ok {
		t.Error("spine.example.com should exist as a separate device")
	}
}

func TestBuildSystemNameIndex(t *testing.T) {
	results := []switchResult{
		{SwitchID: "TOR-1", Device: topology.Device{SystemName: "sw1.example.com"}},
		{SwitchID: "TOR-2", Device: topology.Device{SystemName: "sw2.example.com"}},
		{SwitchID: "TOR-3", Device: topology.Device{SystemName: "TOR-3"}}, // same as ID — should be excluded
		{SwitchID: "TOR-4", Device: topology.Device{SystemName: ""}},       // empty — should be excluded
	}

	idx := buildSystemNameIndex(results)

	if len(idx) != 2 {
		t.Errorf("expected 2 entries, got %d", len(idx))
	}
	if idx["sw1.example.com"] != "TOR-1" {
		t.Errorf("expected sw1.example.com → TOR-1, got %q", idx["sw1.example.com"])
	}
	if idx["sw2.example.com"] != "TOR-2" {
		t.Errorf("expected sw2.example.com → TOR-2, got %q", idx["sw2.example.com"])
	}
}
