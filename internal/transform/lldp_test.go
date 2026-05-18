package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseLLDPFlatLeaf(t *testing.T) {
	// Simulates SONiC flat-leaf Subscribe ONCE format for LLDP.
	// Each notification has a prefix with interface[name=X] and updates with
	// nested neighbor paths like /neighbors/neighbor[id=Y]/state/field.
	notifs := []gnmi.Notification{
		{
			Prefix: "/openconfig-lldp:lldp/interfaces/interface[name=Ethernet48]/neighbors",
			Updates: []gnmi.Update{
				{Path: "/neighbors/neighbor[id=74:86:e2:6e:70:a5]/state/chassis-id", Value: "74:86:e2:6e:70:a5"},
				{Path: "/neighbors/neighbor[id=74:86:e2:6e:70:a5]/state/port-id", Value: "Ethernet49"},
				{Path: "/neighbors/neighbor[id=74:86:e2:6e:70:a5]/state/system-name", Value: "tor-switch-2"},
				{Path: "/neighbors/neighbor[id=74:86:e2:6e:70:a5]/state/system-description", Value: "Dell Enterprise SONiC"},
			},
		},
		{
			Prefix: "/openconfig-lldp:lldp/interfaces/interface[name=Ethernet1]/neighbors",
			Updates: []gnmi.Update{
				{Path: "/neighbors/neighbor[id=aa:bb:cc:dd:ee:ff]/state/chassis-id", Value: "aa:bb:cc:dd:ee:ff"},
				{Path: "/neighbors/neighbor[id=aa:bb:cc:dd:ee:ff]/state/port-id", Value: "eth0"},
				{Path: "/neighbors/neighbor[id=aa:bb:cc:dd:ee:ff]/state/system-name", Value: "server-01"},
			},
		},
	}

	neighbors := ParseLLDPOpenConfig(notifs)

	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(neighbors))
	}

	// First neighbor (Ethernet48 → normalized to Eth48)
	n0 := neighbors[0]
	if n0.LocalPort != "Eth48" {
		t.Errorf("neighbor 0: expected LocalPort 'Eth48', got %q", n0.LocalPort)
	}
	if n0.ChassisID != "74:86:e2:6e:70:a5" {
		t.Errorf("neighbor 0: expected ChassisID '74:86:e2:6e:70:a5', got %q", n0.ChassisID)
	}
	if n0.SystemName != "tor-switch-2" {
		t.Errorf("neighbor 0: expected SystemName 'tor-switch-2', got %q", n0.SystemName)
	}

	// Second neighbor (Ethernet1 → normalized to Eth1)
	n1 := neighbors[1]
	if n1.LocalPort != "Eth1" {
		t.Errorf("neighbor 1: expected LocalPort 'Eth1', got %q", n1.LocalPort)
	}
	if n1.ChassisID != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("neighbor 1: expected ChassisID 'aa:bb:cc:dd:ee:ff', got %q", n1.ChassisID)
	}
	if n1.SystemName != "server-01" {
		t.Errorf("neighbor 1: expected SystemName 'server-01', got %q", n1.SystemName)
	}
	if n1.PortID != "eth0" {
		t.Errorf("neighbor 1: expected PortID 'eth0', got %q", n1.PortID)
	}
}

func TestParseLLDPFlatLeaf_SkipsEmptyNotifications(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			// This notification has chassis-id but no port-id and no system info
			// It should still be included since chassis-id alone is enough
			Prefix: "/openconfig-lldp:lldp/interfaces/interface[name=Ethernet10]/neighbors",
			Updates: []gnmi.Update{
				{Path: "/neighbors/neighbor[id=X]/config/enabled", Value: "true"},
				{Path: "/neighbors/neighbor[id=X]/state/chassis-id", Value: ""},
			},
		},
		{
			Prefix: "/openconfig-lldp:lldp/interfaces/interface[name=Ethernet20]/neighbors",
			Updates: []gnmi.Update{
				{Path: "/neighbors/neighbor[id=Y]/state/chassis-id", Value: "11:22:33:44:55:66"},
				{Path: "/neighbors/neighbor[id=Y]/state/port-id", Value: "ge-0/0/1"},
			},
		},
	}

	neighbors := ParseLLDPOpenConfig(notifs)

	if len(neighbors) != 1 {
		t.Fatalf("expected 1 neighbor (empty chassis+port should be skipped), got %d", len(neighbors))
	}
	if neighbors[0].LocalPort != "Eth20" {
		t.Errorf("expected LocalPort 'Eth20', got %q", neighbors[0].LocalPort)
	}
}
