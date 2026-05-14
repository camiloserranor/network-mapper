package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseARPTableOpenConfig_BulkFormat(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-if-ip:interfaces/interface[name=Ethernet48]/subinterfaces/subinterface/ipv4/neighbors",
					Value: map[string]interface{}{
						"neighbor": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"ip":                  "10.0.2.1",
									"link-layer-address":  "aa:bb:cc:dd:ee:01",
								},
							},
							map[string]interface{}{
								"state": map[string]interface{}{
									"ip":                  "10.0.2.2",
									"link-layer-address":  "aa:bb:cc:dd:ee:02",
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseARPTableOpenConfig(notifs, "tor-1")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	tests := []struct {
		idx       int
		ip        string
		mac       string
		iface     string
		switchID  string
	}{
		{0, "10.0.2.1", "aa:bb:cc:dd:ee:01", "Ethernet48", "tor-1"},
		{1, "10.0.2.2", "aa:bb:cc:dd:ee:02", "Ethernet48", "tor-1"},
	}
	for _, tt := range tests {
		e := entries[tt.idx]
		if e.IP != tt.ip {
			t.Errorf("[%d] IP = %q, want %q", tt.idx, e.IP, tt.ip)
		}
		if e.MAC != tt.mac {
			t.Errorf("[%d] MAC = %q, want %q", tt.idx, e.MAC, tt.mac)
		}
		if e.Interface != tt.iface {
			t.Errorf("[%d] Interface = %q, want %q", tt.idx, e.Interface, tt.iface)
		}
		if e.SwitchID != tt.switchID {
			t.Errorf("[%d] SwitchID = %q, want %q", tt.idx, e.SwitchID, tt.switchID)
		}
	}
}

func TestParseARPTableOpenConfig_SubscribeONCE(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-if-ip:interfaces/interface[name=Ethernet1]/subinterfaces/subinterface/ipv4/neighbors/neighbor",
					Value: map[string]interface{}{
						"state": map[string]interface{}{
							"ip":                 "192.168.1.10",
							"link-layer-address": "DE:AD:BE:EF:00:01",
						},
					},
				},
			},
		},
	}

	entries := ParseARPTableOpenConfig(notifs, "sw-2")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.IP != "192.168.1.10" {
		t.Errorf("IP = %q, want %q", e.IP, "192.168.1.10")
	}
	if e.MAC != "de:ad:be:ef:00:01" {
		t.Errorf("MAC = %q, want %q", e.MAC, "de:ad:be:ef:00:01")
	}
	if e.Interface != "Ethernet1" {
		t.Errorf("Interface = %q, want %q", e.Interface, "Ethernet1")
	}
}

func TestParseARPTableOpenConfig_EmptyInput(t *testing.T) {
	entries := ParseARPTableOpenConfig(nil, "sw-1")
	if len(entries) != 0 {
		t.Errorf("got %d entries from nil input, want 0", len(entries))
	}

	entries = ParseARPTableOpenConfig([]gnmi.Notification{}, "sw-1")
	if len(entries) != 0 {
		t.Errorf("got %d entries from empty input, want 0", len(entries))
	}
}

func TestParseARPTableOpenConfig_MissingIPSkipped(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/interfaces/interface[name=Ethernet1]/neighbors",
					Value: map[string]interface{}{
						"neighbor": []interface{}{
							// Valid entry
							map[string]interface{}{
								"state": map[string]interface{}{
									"ip":                 "10.0.0.1",
									"link-layer-address": "aa:bb:cc:dd:ee:ff",
								},
							},
							// Missing IP — should be skipped
							map[string]interface{}{
								"state": map[string]interface{}{
									"link-layer-address": "11:22:33:44:55:66",
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseARPTableOpenConfig(notifs, "sw-1")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (entry without IP should be skipped)", len(entries))
	}
	if entries[0].IP != "10.0.0.1" {
		t.Errorf("IP = %q, want %q", entries[0].IP, "10.0.0.1")
	}
}

func TestParseARPTableOpenConfig_MultipleNotifications(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/interfaces/interface[name=Ethernet1]/neighbors",
					Value: map[string]interface{}{
						"neighbor": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"ip":                 "10.0.0.1",
									"link-layer-address": "aa:bb:cc:dd:ee:01",
								},
							},
						},
					},
				},
			},
		},
		{
			Updates: []gnmi.Update{
				{
					Path: "/interfaces/interface[name=Ethernet2]/neighbors",
					Value: map[string]interface{}{
						"neighbor": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"ip":                 "10.0.0.2",
									"link-layer-address": "aa:bb:cc:dd:ee:02",
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseARPTableOpenConfig(notifs, "sw-1")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Interface != "Ethernet1" {
		t.Errorf("[0] Interface = %q, want %q", entries[0].Interface, "Ethernet1")
	}
	if entries[1].Interface != "Ethernet2" {
		t.Errorf("[1] Interface = %q, want %q", entries[1].Interface, "Ethernet2")
	}
}
