package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseVLANsOpenConfig_MultipleVLANsWithMembers(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-network-instance:network-instances/network-instance/vlans/vlan",
					Value: map[string]interface{}{
						"vlan": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"vlan-id": float64(100),
									"name":    "mgmt",
									"status":  "ACTIVE",
								},
								"members": map[string]interface{}{
									"member": []interface{}{
										map[string]interface{}{
											"state": map[string]interface{}{
												"interface": "Ethernet1",
											},
										},
										map[string]interface{}{
											"state": map[string]interface{}{
												"interface": "Ethernet2",
											},
										},
									},
								},
							},
							map[string]interface{}{
								"state": map[string]interface{}{
									"vlan-id": float64(200),
									"name":    "storage",
									"status":  "ACTIVE",
								},
								"members": map[string]interface{}{
									"member": []interface{}{
										map[string]interface{}{
											"state": map[string]interface{}{
												"interface": "Ethernet3",
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

	vlans := ParseVLANsOpenConfig(notifs, "tor-1")
	if len(vlans) != 2 {
		t.Fatalf("got %d VLANs, want 2", len(vlans))
	}

	// VLAN 100
	if vlans[0].ID != 100 {
		t.Errorf("[0] ID = %d, want 100", vlans[0].ID)
	}
	if vlans[0].Name != "mgmt" {
		t.Errorf("[0] Name = %q, want %q", vlans[0].Name, "mgmt")
	}
	if len(vlans[0].MemberPorts) != 2 {
		t.Errorf("[0] MemberPorts count = %d, want 2", len(vlans[0].MemberPorts))
	}
	if vlans[0].SourceSwitch != "tor-1" {
		t.Errorf("[0] SourceSwitch = %q, want %q", vlans[0].SourceSwitch, "tor-1")
	}

	// VLAN 200
	if vlans[1].ID != 200 {
		t.Errorf("[1] ID = %d, want 200", vlans[1].ID)
	}
	if len(vlans[1].MemberPorts) != 1 {
		t.Errorf("[1] MemberPorts count = %d, want 1", len(vlans[1].MemberPorts))
	}
}

func TestParseVLANsOpenConfig_VLANWithoutMembers(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/vlans/vlan",
					Value: map[string]interface{}{
						"vlan": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"vlan-id": float64(300),
									"name":    "isolated",
								},
							},
						},
					},
				},
			},
		},
	}

	vlans := ParseVLANsOpenConfig(notifs, "sw-1")
	if len(vlans) != 1 {
		t.Fatalf("got %d VLANs, want 1", len(vlans))
	}
	if vlans[0].ID != 300 {
		t.Errorf("ID = %d, want 300", vlans[0].ID)
	}
	if len(vlans[0].MemberPorts) != 0 {
		t.Errorf("MemberPorts = %v, want empty", vlans[0].MemberPorts)
	}
}

func TestParseVLANsOpenConfig_VLANIDFromPathKey(t *testing.T) {
	// Subscribe ONCE: VLAN ID comes from path key, state may not have vlan-id
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/vlans/vlan[vlan-id=42]",
					Value: map[string]interface{}{
						"state": map[string]interface{}{
							"name": "special",
						},
					},
				},
			},
		},
	}

	vlans := ParseVLANsOpenConfig(notifs, "sw-1")
	if len(vlans) != 1 {
		t.Fatalf("got %d VLANs, want 1", len(vlans))
	}
	if vlans[0].ID != 42 {
		t.Errorf("ID = %d, want 42 (from path key)", vlans[0].ID)
	}
}

func TestParseVLANsOpenConfig_VLANIDAsString(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/vlans/vlan",
					Value: map[string]interface{}{
						"vlan": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"vlan-id": "500",
									"name":    "string-id",
								},
							},
						},
					},
				},
			},
		},
	}

	vlans := ParseVLANsOpenConfig(notifs, "sw-1")
	if len(vlans) != 1 {
		t.Fatalf("got %d VLANs, want 1", len(vlans))
	}
	if vlans[0].ID != 500 {
		t.Errorf("ID = %d, want 500 (from string vlan-id)", vlans[0].ID)
	}
}

func TestParseVLANsOpenConfig_EmptyInput(t *testing.T) {
	vlans := ParseVLANsOpenConfig(nil, "sw-1")
	if len(vlans) != 0 {
		t.Errorf("got %d VLANs from nil input, want 0", len(vlans))
	}

	vlans = ParseVLANsOpenConfig([]gnmi.Notification{}, "sw-1")
	if len(vlans) != 0 {
		t.Errorf("got %d VLANs from empty input, want 0", len(vlans))
	}
}

func TestParseVLANsOpenConfig_SubscribeONCEFormat(t *testing.T) {
	// Single VLAN with state at top level
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/vlans/vlan[vlan-id=10]",
					Value: map[string]interface{}{
						"state": map[string]interface{}{
							"vlan-id": float64(10),
							"name":    "native",
							"status":  "ACTIVE",
						},
					},
				},
			},
		},
	}

	vlans := ParseVLANsOpenConfig(notifs, "sw-1")
	if len(vlans) != 1 {
		t.Fatalf("got %d VLANs, want 1", len(vlans))
	}
	if vlans[0].ID != 10 {
		t.Errorf("ID = %d, want 10", vlans[0].ID)
	}
	if vlans[0].Name != "native" {
		t.Errorf("Name = %q, want %q", vlans[0].Name, "native")
	}
}

func TestParseVLANsOpenConfig_MembersViaInterfaceRef(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/vlans/vlan",
					Value: map[string]interface{}{
						"vlan": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"vlan-id": float64(77),
									"name":    "ref-test",
								},
								"members": map[string]interface{}{
									"member": []interface{}{
										map[string]interface{}{
											"interface-ref": map[string]interface{}{
												"state": map[string]interface{}{
													"interface": "Ethernet5",
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

	vlans := ParseVLANsOpenConfig(notifs, "sw-1")
	if len(vlans) != 1 {
		t.Fatalf("got %d VLANs, want 1", len(vlans))
	}
	if len(vlans[0].MemberPorts) != 1 {
		t.Fatalf("MemberPorts count = %d, want 1", len(vlans[0].MemberPorts))
	}
	if vlans[0].MemberPorts[0] != "Eth5" {
		t.Errorf("MemberPorts[0] = %q, want %q", vlans[0].MemberPorts[0], "Eth5")
	}
}
