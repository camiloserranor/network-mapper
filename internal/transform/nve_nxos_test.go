package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestIsNVEInterface(t *testing.T) {
	tests := []struct {
		port string
		want bool
	}{
		{"Nve", true},
		{"nve", true},
		{"NVE", true},
		{"Nve1", true},
		{"nve1", true},
		{"NVE1", true},
		{"nve 1", true},
		{"Ethernet1/1", false},
		{"Eth1/1", false},
		{"port-channel1", false},
		{"Vlan100", false},
		{"", false},
		{"loopback0", false},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			got := IsNVEInterface(tt.port)
			if got != tt.want {
				t.Errorf("IsNVEInterface(%q) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func TestParseNVEPeersNXOS(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/eps-items",
					Value: map[string]interface{}{
						"epId-items": map[string]interface{}{
							"Ep-list": []interface{}{
								map[string]interface{}{
									"epId":      float64(1),
									"primaryIp": "100.71.36.18",
									"peers-items": map[string]interface{}{
										"DyPeer-list": []interface{}{
											map[string]interface{}{
												"ip":    "100.71.93.158",
												"mac":   "B0:8B:CF:BF:A0:E9",
												"state": "Up",
											},
											map[string]interface{}{
												"ip":    "100.71.94.26",
												"mac":   "48:74:10:09:05:8B",
												"state": "Up",
											},
											map[string]interface{}{
												"ip":    "100.71.93.155",
												"mac":   "9C:09:8B:FB:BD:8F",
												"state": "Down",
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

	peers := ParseNVEPeersNXOS(notifs)
	if len(peers) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(peers))
	}

	// Verify first peer
	if peers[0].PeerIP != "100.71.93.158" {
		t.Errorf("peer[0].PeerIP = %q, want %q", peers[0].PeerIP, "100.71.93.158")
	}
	if peers[0].PeerMAC != "b0:8b:cf:bf:a0:e9" {
		t.Errorf("peer[0].PeerMAC = %q, want %q", peers[0].PeerMAC, "b0:8b:cf:bf:a0:e9")
	}
	if peers[0].State != "Up" {
		t.Errorf("peer[0].State = %q, want %q", peers[0].State, "Up")
	}

	// Verify Down state
	if peers[2].State != "Down" {
		t.Errorf("peer[2].State = %q, want %q", peers[2].State, "Down")
	}
}

func TestParseNVEPeersNXOS_Empty(t *testing.T) {
	peers := ParseNVEPeersNXOS(nil)
	if len(peers) != 0 {
		t.Errorf("expected 0 peers from nil input, got %d", len(peers))
	}

	peers = ParseNVEPeersNXOS([]gnmi.Notification{})
	if len(peers) != 0 {
		t.Errorf("expected 0 peers from empty input, got %d", len(peers))
	}
}

// TestParseNVEPeersNXOS_RealStructure tests with the actual NX-OS structure
// where peers are under peers-items → dy_peer-items → DyPeer-list
func TestParseNVEPeersNXOS_RealStructure(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/eps-items",
					Value: map[string]interface{}{
						"epId-items": map[string]interface{}{
							"Ep-list": []interface{}{
								map[string]interface{}{
									"epId":      float64(1),
									"primaryIp": "100.71.36.18",
									"mac":       "F8:39:18:E8:DC:1F",
									"peers-items": map[string]interface{}{
										"dy_peer-items": map[string]interface{}{
											"DyPeer-list": []interface{}{
												map[string]interface{}{
													"ip":    "100.71.93.158",
													"mac":   "B0:8B:CF:BF:A0:E9",
													"state": "Up",
												},
												map[string]interface{}{
													"ip":    "100.71.94.26",
													"mac":   "48:74:10:09:05:8B",
													"state": "Up",
												},
											},
										},
										"dyn_ir_peer-items": map[string]interface{}{},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	peers := ParseNVEPeersNXOS(notifs)
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}
	if peers[0].PeerIP != "100.71.93.158" {
		t.Errorf("peer[0].PeerIP = %q, want %q", peers[0].PeerIP, "100.71.93.158")
	}
	if peers[0].PeerMAC != "b0:8b:cf:bf:a0:e9" {
		t.Errorf("peer[0].PeerMAC = %q, want %q", peers[0].PeerMAC, "b0:8b:cf:bf:a0:e9")
	}
	if peers[1].PeerIP != "100.71.94.26" {
		t.Errorf("peer[1].PeerIP = %q, want %q", peers[1].PeerIP, "100.71.94.26")
	}
}

func TestParseL2RIBNXOS(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/l2rib-items",
					Value: map[string]interface{}{
						"inst-items": map[string]interface{}{
							"topology-items": map[string]interface{}{
								"topo-items": map[string]interface{}{
									"Topo-list": []interface{}{
										map[string]interface{}{
											"topoId": float64(21712),
											"mac-items": map[string]interface{}{
												"MacEntry-list": []interface{}{
													map[string]interface{}{
														"macAddr": "00:15:5d:01:02:03",
														"producer-items": map[string]interface{}{
															"MacRt-list": []interface{}{
																map[string]interface{}{
																	"nexthops-items": map[string]interface{}{
																		"nh-items": map[string]interface{}{
																			"MacNexthop-list": []interface{}{
																				map[string]interface{}{
																					"nh":    "100.71.93.158",
																					"type":  "regular",
																					"label": "21712",
																				},
																			},
																		},
																	},
																},
															},
														},
													},
													map[string]interface{}{
														"macAddr": "00:15:5d:04:05:06",
														"producer-items": map[string]interface{}{
															"MacRt-list": []interface{}{
																map[string]interface{}{
																	"nexthops-items": map[string]interface{}{
																		"nh-items": map[string]interface{}{
																			"MacNexthop-list": []interface{}{
																				map[string]interface{}{
																					"nh":   "100.71.94.26",
																					"type": "regular",
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

	entries := ParseL2RIBNXOS(notifs)
	if len(entries) != 2 {
		t.Fatalf("expected 2 L2RIB entries, got %d", len(entries))
	}

	if entries[0].MAC != "00:15:5d:01:02:03" {
		t.Errorf("entry[0].MAC = %q, want %q", entries[0].MAC, "00:15:5d:01:02:03")
	}
	if entries[0].NextHopIP != "100.71.93.158" {
		t.Errorf("entry[0].NextHopIP = %q, want %q", entries[0].NextHopIP, "100.71.93.158")
	}
	if entries[0].VNI != 21712 {
		t.Errorf("entry[0].VNI = %d, want %d", entries[0].VNI, 21712)
	}

	if entries[1].MAC != "00:15:5d:04:05:06" {
		t.Errorf("entry[1].MAC = %q, want %q", entries[1].MAC, "00:15:5d:04:05:06")
	}
	if entries[1].NextHopIP != "100.71.94.26" {
		t.Errorf("entry[1].NextHopIP = %q, want %q", entries[1].NextHopIP, "100.71.94.26")
	}
}

func TestParseL2RIBNXOS_Empty(t *testing.T) {
	entries := ParseL2RIBNXOS(nil)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries from nil input, got %d", len(entries))
	}
}

func TestParseL2RIBNXOS_MacIpEntries(t *testing.T) {
	// Test the macip-items path which is what real NX-OS 10.4 data uses
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/l2rib-items",
					Value: map[string]interface{}{
						"inst-items": map[string]interface{}{
							"topology-items": map[string]interface{}{
								"topo-items": map[string]interface{}{
									"Topo-list": []interface{}{
										map[string]interface{}{
											"topoId": float64(1201),
											"macip-items": map[string]interface{}{
												"mac-items": map[string]interface{}{
													"MacIpEntry-list": []interface{}{
														map[string]interface{}{
															"macAddr": "02:EC:F8:0A:01:80",
															"ip":      "100.78.73.137",
															"producer-items": map[string]interface{}{
																"MacIpRt-list": []interface{}{
																	map[string]interface{}{
																		"producer": "bgp",
																		"nexthops-items": map[string]interface{}{
																			"nh-items": map[string]interface{}{
																				"MacIpNexthop-list": []interface{}{
																					map[string]interface{}{
																						"nh":    "100.71.94.26",
																						"label": "21201",
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
														map[string]interface{}{
															"macAddr": "02:EC:F8:0A:02:40",
															"ip":      "100.78.73.200",
															"producer-items": map[string]interface{}{
																"MacIpRt-list": []interface{}{
																	map[string]interface{}{
																		"producer": "bgp",
																		"nexthops-items": map[string]interface{}{
																			"nh-items": map[string]interface{}{
																				"MacIpNexthop-list": []interface{}{
																					map[string]interface{}{
																						"nh":    "100.71.93.154",
																						"label": "21201",
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
														// Local entry should be skipped
														map[string]interface{}{
															"macAddr": "02:EC:F8:0A:03:00",
															"ip":      "100.78.73.50",
															"producer-items": map[string]interface{}{
																"MacIpRt-list": []interface{}{
																	map[string]interface{}{
																		"producer": "hmm",
																		"nexthops-items": map[string]interface{}{
																			"nh-items": map[string]interface{}{
																				"MacIpNexthop-list": []interface{}{
																					map[string]interface{}{
																						"nh": "Local",
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

	entries := ParseL2RIBNXOS(notifs)
	if len(entries) != 2 {
		t.Fatalf("expected 2 L2RIB entries (skipping Local), got %d", len(entries))
	}

	if entries[0].MAC != "02:ec:f8:0a:01:80" {
		t.Errorf("entry[0].MAC = %q, want %q", entries[0].MAC, "02:ec:f8:0a:01:80")
	}
	if entries[0].NextHopIP != "100.71.94.26" {
		t.Errorf("entry[0].NextHopIP = %q, want %q", entries[0].NextHopIP, "100.71.94.26")
	}
	if entries[0].VNI != 1201 {
		t.Errorf("entry[0].VNI = %d, want %d", entries[0].VNI, 1201)
	}

	if entries[1].MAC != "02:ec:f8:0a:02:40" {
		t.Errorf("entry[1].MAC = %q, want %q", entries[1].MAC, "02:ec:f8:0a:02:40")
	}
	if entries[1].NextHopIP != "100.71.93.154" {
		t.Errorf("entry[1].NextHopIP = %q, want %q", entries[1].NextHopIP, "100.71.93.154")
	}
	if entries[1].VNI != 1201 {
		t.Errorf("entry[1].VNI = %d, want %d", entries[1].VNI, 1201)
	}
}

func TestCorrelateEndpoints_VTEPFallback(t *testing.T) {
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Ethernet1/1", ChassisID: "d8:94:24:00:00:01", SystemName: "host-1"},
			},
			MACEntries: []MACEntry{
				// VM on physical port → should be attributed to host-1
				{MAC: "00:15:5d:aa:bb:cc", Port: "Ethernet1/1", VLAN: 100},
				// VM on NVE port → should get VTEP attribution from L2RIB
				{MAC: "00:15:5d:11:22:33", Port: "Nve1", VLAN: 100},
				// VM on NVE port, no L2RIB entry → unattributed
				{MAC: "00:15:5d:99:99:99", Port: "Nve1", VLAN: 200},
			},
			ARPEntries: []ARPEntry{
				{MAC: "00:15:5d:11:22:33", IP: "10.0.0.50"},
			},
			NVEPeers: []NVEPeer{
				{PeerIP: "100.71.93.158", PeerMAC: "b0:8b:cf:bf:a0:e9", State: "Up"},
			},
			L2RIBMacs: []L2RIBEntry{
				{MAC: "00:15:5d:11:22:33", NextHopIP: "100.71.93.158", VNI: 21712},
			},
		},
	}

	endpoints := CorrelateEndpoints(inputs)

	// Should have 3 endpoints total
	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(endpoints))
	}

	// Find each endpoint by MAC
	epMap := make(map[string]*endpoint_topology_Endpoint)
	for i := range endpoints {
		epMap[endpoints[i].MAC] = &endpoints[i]
	}

	// Port-attributed VM
	portVM := epMap["00:15:5d:aa:bb:cc"]
	if portVM == nil {
		t.Fatal("missing port-attributed VM 00:15:5d:aa:bb:cc")
	}
	if portVM.HostDevice != "host-1" {
		t.Errorf("port VM HostDevice = %q, want %q", portVM.HostDevice, "host-1")
	}
	if portVM.VTEPIP != "" {
		t.Errorf("port VM should not have VTEPIP, got %q", portVM.VTEPIP)
	}

	// VTEP-attributed VM
	vtepVM := epMap["00:15:5d:11:22:33"]
	if vtepVM == nil {
		t.Fatal("missing VTEP-attributed VM 00:15:5d:11:22:33")
	}
	if vtepVM.HostDevice != "" {
		t.Errorf("VTEP VM should not have HostDevice, got %q", vtepVM.HostDevice)
	}
	if vtepVM.VTEPIP != "100.71.93.158" {
		t.Errorf("VTEP VM VTEPIP = %q, want %q", vtepVM.VTEPIP, "100.71.93.158")
	}
	if vtepVM.VNI != 21712 {
		t.Errorf("VTEP VM VNI = %d, want %d", vtepVM.VNI, 21712)
	}
	if len(vtepVM.IPs) != 1 || vtepVM.IPs[0] != "10.0.0.50" {
		t.Errorf("VTEP VM IPs = %v, want [10.0.0.50]", vtepVM.IPs)
	}

	// Unattributed VM (NVE port but no L2RIB match)
	unattVM := epMap["00:15:5d:99:99:99"]
	if unattVM == nil {
		t.Fatal("missing unattributed VM 00:15:5d:99:99:99")
	}
	if unattVM.HostDevice != "" {
		t.Errorf("unattributed VM should not have HostDevice, got %q", unattVM.HostDevice)
	}
	if unattVM.VTEPIP != "" {
		t.Errorf("unattributed VM should not have VTEPIP, got %q", unattVM.VTEPIP)
	}
}

// Type alias to avoid confusion with topology.Endpoint in test code
type endpoint_topology_Endpoint = topology.Endpoint

func TestCorrelateEndpoints_NonVXLAN_Unchanged(t *testing.T) {
	// Verify that without L2RIB data, the existing behavior is unchanged
	inputs := []CorrelationInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Ethernet1/1", ChassisID: "aa:bb:cc:00:00:01", SystemName: "host-1"},
				{LocalPort: "Ethernet1/2", ChassisID: "aa:bb:cc:00:00:02", SystemName: "host-2"},
			},
			MACEntries: []MACEntry{
				{MAC: "00:15:5d:01:01:01", Port: "Ethernet1/1", VLAN: 100},
				{MAC: "00:15:5d:02:02:02", Port: "Ethernet1/2", VLAN: 200},
			},
			ARPEntries: nil,
			NVEPeers:   nil, // No VXLAN
			L2RIBMacs:  nil, // No L2RIB
		},
	}

	endpoints := CorrelateEndpoints(inputs)
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	// Both should be port-attributed
	for _, ep := range endpoints {
		if ep.HostDevice == "" {
			t.Errorf("endpoint %s should be attributed to a host", ep.MAC)
		}
		if ep.VTEPIP != "" {
			t.Errorf("endpoint %s should not have VTEPIP in non-VXLAN env", ep.MAC)
		}
	}
}

func TestParseQoSStatsNXOS(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/ipqos-items/queuing-items/policy-items",
					Value: map[string]interface{}{
						"out-items": map[string]interface{}{
							"intf-items": map[string]interface{}{
								"If-list": []interface{}{
									map[string]interface{}{
										"name": "Ethernet1/1",
										"queCmap-items": map[string]interface{}{
											"QueuingStats-list": []interface{}{
												map[string]interface{}{
													"cmapName":             "c-out-8q-q3",
													"pfcTxPpp":             float64(1500),
													"pfcRxPpp":             float64(200),
													"pfcwdFlushedPackets":  float64(0),
													"randEcnMarkedPackets": float64(42000),
													"dropPackets":          float64(0),
													"currQueueDepth":       float64(4096),
													"maxQueueDepth":        float64(307200),
												},
												map[string]interface{}{
													"cmapName":    "c-out-8q-q-default",
													"dropPackets": float64(1234),
													"pfcTxPpp":    float64(0),
													"pfcRxPpp":    float64(0),
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
		},
	}

	stats := ParseQoSStatsNXOS(notifs)
	if len(stats) != 1 {
		t.Fatalf("expected 1 interface with stats, got %d", len(stats))
	}

	if stats[0].InterfaceName != "Eth1/1" {
		t.Errorf("InterfaceName = %q, want Eth1/1", stats[0].InterfaceName)
	}
	if len(stats[0].Queues) != 2 {
		t.Fatalf("expected 2 queues with non-zero stats, got %d", len(stats[0].Queues))
	}

	// Find RDMA queue (q3)
	var rdmaQ *QueueStat
	for i := range stats[0].Queues {
		if stats[0].Queues[i].QueueName == "c-out-8q-q3" {
			rdmaQ = &stats[0].Queues[i]
			break
		}
	}
	if rdmaQ == nil {
		t.Fatal("RDMA queue c-out-8q-q3 not found")
	}
	if rdmaQ.Direction != "egress" {
		t.Errorf("Direction = %q, want egress", rdmaQ.Direction)
	}
	if rdmaQ.PFCPauseFramesTx != 1500 {
		t.Errorf("PFCPauseFramesTx = %d, want 1500", rdmaQ.PFCPauseFramesTx)
	}
	if rdmaQ.ECNMarkedPackets != 42000 {
		t.Errorf("ECNMarkedPackets = %d, want 42000", rdmaQ.ECNMarkedPackets)
	}
	if rdmaQ.CurrentQueueDepth != 4096 {
		t.Errorf("CurrentQueueDepth = %d, want 4096", rdmaQ.CurrentQueueDepth)
	}
}

func TestParsePFCConfigNXOS(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/intf-items/phys-items/PhysIf-list",
					Value: map[string]interface{}{
						"PhysIf-list": []interface{}{
							map[string]interface{}{
								"id": "eth1/1",
								"priorflowctrl-items": map[string]interface{}{
									"mode":     "on",
									"sendTlv":  "true",
									"pfcCos0":  "false",
									"pfcCos1":  "false",
									"pfcCos2":  "false",
									"pfcCos3":  "true",
									"pfcCos4":  "false",
									"pfcCos5":  "false",
									"pfcCos6":  "false",
									"pfcCos7":  "false",
								},
							},
							map[string]interface{}{
								"id": "eth1/2",
								"priorflowctrl-items": map[string]interface{}{
									"mode":    "off",
									"sendTlv": "false",
								},
							},
						},
					},
				},
			},
		},
	}

	configs := ParsePFCConfigNXOS(notifs)
	if len(configs) != 2 {
		t.Fatalf("expected 2 PFC configs, got %d", len(configs))
	}

	// First interface: PFC on, CoS 3 lossless
	if configs[0].Mode != "on" {
		t.Errorf("configs[0].Mode = %q, want on", configs[0].Mode)
	}
	if !configs[0].SendTLV {
		t.Error("configs[0].SendTLV should be true")
	}
	if len(configs[0].LosslessCos) != 1 || configs[0].LosslessCos[0] != 3 {
		t.Errorf("configs[0].LosslessCos = %v, want [3]", configs[0].LosslessCos)
	}

	// Second interface: PFC off
	if configs[1].Mode != "off" {
		t.Errorf("configs[1].Mode = %q, want off", configs[1].Mode)
	}
	if configs[1].SendTLV {
		t.Error("configs[1].SendTLV should be false")
	}
	if len(configs[1].LosslessCos) != 0 {
		t.Errorf("configs[1].LosslessCos = %v, want empty", configs[1].LosslessCos)
	}
}
