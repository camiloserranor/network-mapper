package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseBGPOpenConfig(t *testing.T) {
	tests := []struct {
		name     string
		notifs   []gnmi.Notification
		expected []BGPNeighbor
	}{
		{
			name: "single neighbor with state container",
			notifs: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "/openconfig-network-instance:network-instances/network-instance[name=default]/protocols/protocol/bgp/neighbors/neighbor[neighbor-address=10.0.0.1]",
					Value: map[string]interface{}{
						"state": map[string]interface{}{
							"neighbor-address":        "10.0.0.1",
							"peer-as":                 float64(65001),
							"local-as":                float64(65000),
							"peer-type":               "EXTERNAL",
							"session-state":           "ESTABLISHED",
							"enabled":                 true,
							"established-transitions": float64(3),
							"last-established":        "1625000000",
							"description":             "spine-1",
							"messages": map[string]interface{}{
								"received": map[string]interface{}{
									"UPDATE":       float64(150),
									"NOTIFICATION": float64(2),
								},
								"sent": map[string]interface{}{
									"UPDATE":       float64(100),
									"NOTIFICATION": float64(1),
								},
							},
						},
					},
				}},
			}},
			expected: []BGPNeighbor{{
				NeighborAddress:        "10.0.0.1",
				PeerAS:                 65001,
				LocalAS:                65000,
				PeerType:               "EXTERNAL",
				SessionState:           "ESTABLISHED",
				Enabled:                true,
				EstablishedTransitions: 3,
				LastEstablished:        "1625000000",
				Description:            "spine-1",
				VRFName:                "default",
				MessagesReceived:       152,
				MessagesSent:           101,
			}},
		},
		{
			name: "bulk neighbor list",
			notifs: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "/openconfig-network-instance:network-instances/network-instance[name=default]/protocols/protocol/bgp/neighbors",
					Value: map[string]interface{}{
						"neighbor": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"neighbor-address": "10.0.0.1",
									"peer-as":          float64(65001),
									"session-state":    "established",
									"enabled":          true,
								},
							},
							map[string]interface{}{
								"state": map[string]interface{}{
									"neighbor-address": "10.0.0.2",
									"peer-as":          float64(65002),
									"session-state":    "idle",
									"enabled":          false,
								},
							},
						},
					},
				}},
			}},
			expected: []BGPNeighbor{
				{
					NeighborAddress: "10.0.0.1",
					PeerAS:          65001,
					SessionState:    "ESTABLISHED",
					Enabled:         true,
					VRFName:         "default",
				},
				{
					NeighborAddress: "10.0.0.2",
					PeerAS:          65002,
					SessionState:    "IDLE",
					Enabled:         false,
					VRFName:         "default",
				},
			},
		},
		{
			name: "address with subnet mask stripped",
			notifs: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "/openconfig-network-instance:network-instances/network-instance[name=mgmt]/protocols/protocol/bgp/neighbors",
					Value: map[string]interface{}{
						"neighbor": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"neighbor-address": "100.71.182.128/25",
									"peer-as":          float64(65100),
									"session-state":    "ESTABLISHED",
									"enabled":          true,
								},
							},
						},
					},
				}},
			}},
			expected: []BGPNeighbor{{
				NeighborAddress: "100.71.182.128",
				PeerAS:          65100,
				SessionState:    "ESTABLISHED",
				Enabled:         true,
				VRFName:         "mgmt",
			}},
		},
		{
			name:   "empty notifications",
			notifs: []gnmi.Notification{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBGPOpenConfig(tt.notifs)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d neighbors, want %d", len(got), len(tt.expected))
			}
			for i, g := range got {
				e := tt.expected[i]
				if g.NeighborAddress != e.NeighborAddress {
					t.Errorf("[%d] NeighborAddress = %q, want %q", i, g.NeighborAddress, e.NeighborAddress)
				}
				if g.PeerAS != e.PeerAS {
					t.Errorf("[%d] PeerAS = %d, want %d", i, g.PeerAS, e.PeerAS)
				}
				if g.SessionState != e.SessionState {
					t.Errorf("[%d] SessionState = %q, want %q", i, g.SessionState, e.SessionState)
				}
				if g.Enabled != e.Enabled {
					t.Errorf("[%d] Enabled = %v, want %v", i, g.Enabled, e.Enabled)
				}
				if g.VRFName != e.VRFName {
					t.Errorf("[%d] VRFName = %q, want %q", i, g.VRFName, e.VRFName)
				}
				if e.LocalAS != 0 && g.LocalAS != e.LocalAS {
					t.Errorf("[%d] LocalAS = %d, want %d", i, g.LocalAS, e.LocalAS)
				}
				if e.PeerType != "" && g.PeerType != e.PeerType {
					t.Errorf("[%d] PeerType = %q, want %q", i, g.PeerType, e.PeerType)
				}
				if e.Description != "" && g.Description != e.Description {
					t.Errorf("[%d] Description = %q, want %q", i, g.Description, e.Description)
				}
				if e.MessagesReceived != 0 && g.MessagesReceived != e.MessagesReceived {
					t.Errorf("[%d] MessagesReceived = %d, want %d", i, g.MessagesReceived, e.MessagesReceived)
				}
				if e.MessagesSent != 0 && g.MessagesSent != e.MessagesSent {
					t.Errorf("[%d] MessagesSent = %d, want %d", i, g.MessagesSent, e.MessagesSent)
				}
				if e.EstablishedTransitions != 0 && g.EstablishedTransitions != e.EstablishedTransitions {
					t.Errorf("[%d] EstablishedTransitions = %d, want %d", i, g.EstablishedTransitions, e.EstablishedTransitions)
				}
			}
		})
	}
}

func TestParseBGPNXOS(t *testing.T) {
	tests := []struct {
		name     string
		notifs   []gnmi.Notification
		expected []BGPNeighbor
	}{
		{
			name: "single peer with ent-items",
			notifs: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "/System/bgp-items/inst-items/dom-items/Dom-list[name=default]/peer-items/Peer-list",
					Value: []interface{}{
						map[string]interface{}{
							"addr": "10.0.0.1",
							"asn":  float64(65001),
							"name": "spine-1",
							"ent-items": map[string]interface{}{
								"PeerEntry-list": []interface{}{
									map[string]interface{}{
										"operSt":  "established",
										"operAsn": float64(65001),
										"peerstats-items": map[string]interface{}{
											"msgRcvd":           float64(5000),
											"msgSent":           float64(4800),
											"pfxRcvd":           float64(200),
											"pfxSent":           float64(150),
											"estabTransitions":  float64(2),
										},
									},
								},
							},
						},
					},
				}},
			}},
			expected: []BGPNeighbor{{
				NeighborAddress:        "10.0.0.1",
				PeerAS:                 65001,
				Description:            "spine-1",
				SessionState:           "ESTABLISHED",
				VRFName:                "default",
				Enabled:                true,
				MessagesReceived:       5000,
				MessagesSent:           4800,
				PrefixesReceived:       200,
				PrefixesSent:           150,
				EstablishedTransitions: 2,
			}},
		},
		{
			name: "peer without ent-items uses top-level operSt",
			notifs: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "/System/bgp-items/inst-items/dom-items/Dom-list[name=tenant-vrf]/peer-items/Peer-list",
					Value: []interface{}{
						map[string]interface{}{
							"addr":   "192.168.1.1",
							"asn":    float64(65500),
							"operSt": "idle",
						},
					},
				}},
			}},
			expected: []BGPNeighbor{{
				NeighborAddress: "192.168.1.1",
				PeerAS:          65500,
				SessionState:    "IDLE",
				VRFName:         "tenant-vrf",
				Enabled:         true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBGPNXOS(tt.notifs)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d neighbors, want %d", len(got), len(tt.expected))
			}
			for i, g := range got {
				e := tt.expected[i]
				if g.NeighborAddress != e.NeighborAddress {
					t.Errorf("[%d] NeighborAddress = %q, want %q", i, g.NeighborAddress, e.NeighborAddress)
				}
				if g.PeerAS != e.PeerAS {
					t.Errorf("[%d] PeerAS = %d, want %d", i, g.PeerAS, e.PeerAS)
				}
				if g.SessionState != e.SessionState {
					t.Errorf("[%d] SessionState = %q, want %q", i, g.SessionState, e.SessionState)
				}
				if g.VRFName != e.VRFName {
					t.Errorf("[%d] VRFName = %q, want %q", i, g.VRFName, e.VRFName)
				}
				if e.Description != "" && g.Description != e.Description {
					t.Errorf("[%d] Description = %q, want %q", i, g.Description, e.Description)
				}
				if e.MessagesReceived != 0 && g.MessagesReceived != e.MessagesReceived {
					t.Errorf("[%d] MessagesReceived = %d, want %d", i, g.MessagesReceived, e.MessagesReceived)
				}
				if e.MessagesSent != 0 && g.MessagesSent != e.MessagesSent {
					t.Errorf("[%d] MessagesSent = %d, want %d", i, g.MessagesSent, e.MessagesSent)
				}
				if e.PrefixesReceived != 0 && g.PrefixesReceived != e.PrefixesReceived {
					t.Errorf("[%d] PrefixesReceived = %d, want %d", i, g.PrefixesReceived, e.PrefixesReceived)
				}
				if e.EstablishedTransitions != 0 && g.EstablishedTransitions != e.EstablishedTransitions {
					t.Errorf("[%d] EstablishedTransitions = %d, want %d", i, g.EstablishedTransitions, e.EstablishedTransitions)
				}
			}
		})
	}
}

func TestMapNXOSBGPState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"established", "ESTABLISHED"},
		{"idle", "IDLE"},
		{"active", "ACTIVE"},
		{"connect", "CONNECT"},
		{"opensent", "OPENSENT"},
		{"openconfirm", "OPENCONFIRM"},
		{"Established", "ESTABLISHED"},
		{"unknown-state", "UNKNOWN-STATE"},
		{"", ""},
	}

	for _, tt := range tests {
		got := mapNXOSBGPState(tt.input)
		if got != tt.expected {
			t.Errorf("mapNXOSBGPState(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
