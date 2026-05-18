package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseMACTableOpenConfig_BulkFormat(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-network-instance:network-instances/network-instance[name=default]/fdb/mac-table",
					Value: map[string]interface{}{
						"entries": map[string]interface{}{
							"entry": []interface{}{
								map[string]interface{}{
									"state": map[string]interface{}{
										"mac-address": "AA:BB:CC:DD:EE:01",
										"vlan":        "100",
										"entry-type":  "DYNAMIC",
									},
									"interface": map[string]interface{}{
										"interface-ref": map[string]interface{}{
											"state": map[string]interface{}{
												"interface": "Eth48",
											},
										},
									},
								},
								map[string]interface{}{
									"state": map[string]interface{}{
										"mac-address": "AA:BB:CC:DD:EE:02",
										"vlan":        "vlan-200",
										"entry-type":  "STATIC",
									},
									"interface": map[string]interface{}{
										"interface-ref": map[string]interface{}{
											"state": map[string]interface{}{
												"interface": "Eth1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseMACTableOpenConfig(notifs, "switch-1")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	tests := []struct {
		idx      int
		mac      string
		vlan     int
		port     string
		typ      string
		switchID string
	}{
		{0, "aa:bb:cc:dd:ee:01", 100, "Eth48", "dynamic", "switch-1"},
		{1, "aa:bb:cc:dd:ee:02", 200, "Eth1", "static", "switch-1"},
	}
	for _, tt := range tests {
		e := entries[tt.idx]
		if e.MAC != tt.mac {
			t.Errorf("[%d] MAC = %q, want %q", tt.idx, e.MAC, tt.mac)
		}
		if e.VLAN != tt.vlan {
			t.Errorf("[%d] VLAN = %d, want %d", tt.idx, e.VLAN, tt.vlan)
		}
		if e.Port != tt.port {
			t.Errorf("[%d] Port = %q, want %q", tt.idx, e.Port, tt.port)
		}
		if e.Type != tt.typ {
			t.Errorf("[%d] Type = %q, want %q", tt.idx, e.Type, tt.typ)
		}
		if e.SwitchID != tt.switchID {
			t.Errorf("[%d] SwitchID = %q, want %q", tt.idx, e.SwitchID, tt.switchID)
		}
	}
}

func TestParseMACTableOpenConfig_SubscribeONCE(t *testing.T) {
	// Subscribe ONCE delivers each entry as separate notification with state at top level
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-network-instance:network-instances/network-instance[name=default]/fdb/mac-table/entries/entry",
					Value: map[string]interface{}{
						"state": map[string]interface{}{
							"mac-address": "11:22:33:44:55:66",
							"vlan":        "50",
							"entry-type":  "DYNAMIC",
						},
						"interface": map[string]interface{}{
							"interface-ref": map[string]interface{}{
								"state": map[string]interface{}{
									"interface": "Eth10",
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseMACTableOpenConfig(notifs, "tor-2")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.MAC != "11:22:33:44:55:66" {
		t.Errorf("MAC = %q, want %q", e.MAC, "11:22:33:44:55:66")
	}
	if e.VLAN != 50 {
		t.Errorf("VLAN = %d, want 50", e.VLAN)
	}
	if e.Port != "Eth10" {
		t.Errorf("Port = %q, want %q", e.Port, "Eth10")
	}
}

func TestParseMACTableOpenConfig_EmptyInput(t *testing.T) {
	entries := ParseMACTableOpenConfig(nil, "sw-1")
	if len(entries) != 0 {
		t.Errorf("got %d entries from nil input, want 0", len(entries))
	}

	entries = ParseMACTableOpenConfig([]gnmi.Notification{}, "sw-1")
	if len(entries) != 0 {
		t.Errorf("got %d entries from empty input, want 0", len(entries))
	}
}

func TestParseMACTableOpenConfig_MissingMACSkipped(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/fdb/mac-table",
					Value: map[string]interface{}{
						"entries": map[string]interface{}{
							"entry": []interface{}{
								// Entry with MAC — should be included
								map[string]interface{}{
									"state": map[string]interface{}{
										"mac-address": "aa:bb:cc:dd:ee:ff",
										"vlan":        "10",
									},
								},
								// Entry without MAC — should be skipped
								map[string]interface{}{
									"state": map[string]interface{}{
										"vlan": "20",
									},
								},
								// Entry with empty state — should be skipped
								map[string]interface{}{
									"state": map[string]interface{}{},
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseMACTableOpenConfig(notifs, "sw-1")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (entries without MAC should be skipped)", len(entries))
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC = %q, want %q", entries[0].MAC, "aa:bb:cc:dd:ee:ff")
	}
}

func TestParseMACTableOpenConfig_MACNormalization(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/fdb/mac-table",
					Value: map[string]interface{}{
						"entries": map[string]interface{}{
							"entry": []interface{}{
								map[string]interface{}{
									"state": map[string]interface{}{
										"mac-address": "AA-BB-CC-DD-EE-FF",
										"vlan":        "10",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	entries := ParseMACTableOpenConfig(notifs, "sw-1")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC = %q, want %q (dash-separated should be normalized)", entries[0].MAC, "aa:bb:cc:dd:ee:ff")
	}
}
