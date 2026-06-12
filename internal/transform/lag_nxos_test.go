package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseLAGMembershipNXOS_RsMbrIfs(t *testing.T) {
	// Simulate NX-OS response with rsMbrIfs-list format
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Value: []interface{}{
						map[string]interface{}{
							"id":       "po501",
							"pcId":     "501",
							"adminSt":  "up",
							"operSt":   "up",
							"rsMbrIfs-list": []interface{}{
								map[string]interface{}{"tDn": "sys/intf/phys-[eth1/49]"},
								map[string]interface{}{"tDn": "sys/intf/phys-[eth1/50]"},
							},
						},
						map[string]interface{}{
							"id":       "po502",
							"pcId":     "502",
							"adminSt":  "up",
							"operSt":   "up",
							"rsMbrIfs-list": []interface{}{
								map[string]interface{}{"tDn": "sys/intf/phys-[eth1/51]"},
								map[string]interface{}{"tDn": "sys/intf/phys-[eth1/52]"},
							},
						},
					},
				},
			},
		},
	}

	membership := ParseLAGMembershipNXOS(notifs)

	if len(membership) != 2 {
		t.Fatalf("expected 2 port-channels, got %d", len(membership))
	}

	if members := membership["po501"]; len(members) != 2 {
		t.Errorf("po501: expected 2 members, got %d: %v", len(members), members)
	} else {
		if members[0] != "Eth1/49" || members[1] != "Eth1/50" {
			t.Errorf("po501 members = %v, want [Eth1/49, Eth1/50]", members)
		}
	}

	if members := membership["po502"]; len(members) != 2 {
		t.Errorf("po502: expected 2 members, got %d: %v", len(members), members)
	} else {
		if members[0] != "Eth1/51" || members[1] != "Eth1/52" {
			t.Errorf("po502 members = %v, want [Eth1/51, Eth1/52]", members)
		}
	}
}

func TestParseLAGMembershipNXOS_MbrItems(t *testing.T) {
	// Simulate NX-OS response with mbr-items format
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Value: []interface{}{
						map[string]interface{}{
							"id":   "po101",
							"pcId": "101",
							"mbr-items": map[string]interface{}{
								"AggrMbrIf-list": []interface{}{
									map[string]interface{}{"id": "eth1/1"},
									map[string]interface{}{"id": "eth1/2"},
								},
							},
						},
					},
				},
			},
		},
	}

	membership := ParseLAGMembershipNXOS(notifs)

	if len(membership) != 1 {
		t.Fatalf("expected 1 port-channel, got %d", len(membership))
	}

	members := membership["po101"]
	if len(members) != 2 {
		t.Fatalf("po101: expected 2 members, got %d: %v", len(members), members)
	}
	if members[0] != "Eth1/1" || members[1] != "Eth1/2" {
		t.Errorf("po101 members = %v, want [Eth1/1, Eth1/2]", members)
	}
}

func TestParseLAGMembershipNXOS_WrappedInAggrItems(t *testing.T) {
	// Simulate response wrapped in aggr-items container
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Value: map[string]interface{}{
						"aggr-items": map[string]interface{}{
							"AggrIf-list": []interface{}{
								map[string]interface{}{
									"id":   "po200",
									"pcId": "200",
									"rsMbrIfs-list": []interface{}{
										map[string]interface{}{"tDn": "sys/intf/phys-[eth1/10]"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	membership := ParseLAGMembershipNXOS(notifs)

	if len(membership) != 1 {
		t.Fatalf("expected 1 port-channel, got %d", len(membership))
	}
	members := membership["po200"]
	if len(members) != 1 || members[0] != "Eth1/10" {
		t.Errorf("po200 members = %v, want [Eth1/10]", members)
	}
}

func TestNormalizePortChannelName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"po501", "po501"},
		{"Po501", "po501"},
		{"PO501", "po501"},
		{"port-channel501", "po501"},
		{"Port-Channel501", "po501"},
		{"PortChannel501", "po501"},
		{"portchannel501", "po501"},
		{"eth1/1", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := NormalizePortChannelName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizePortChannelName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsPortChannel(t *testing.T) {
	tests := []struct {
		port string
		want bool
	}{
		{"po501", true},
		{"Po501", true},
		{"port-channel501", true},
		{"Port-Channel501", true},
		{"PortChannel501", true},
		{"Eth1/49", false},
		{"nve1", false},
		{"mgmt0", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsPortChannel(tt.port)
		if got != tt.want {
			t.Errorf("IsPortChannel(%q) = %v, want %v", tt.port, got, tt.want)
		}
	}
}

func TestResolveLAGPort(t *testing.T) {
	membership := LAGMembership{
		"po501": {"Eth1/49", "Eth1/50"},
		"po502": {"Eth1/51", "Eth1/52"},
	}

	tests := []struct {
		port    string
		wantLen int
	}{
		{"po501", 2},
		{"Po501", 2},
		{"port-channel501", 2},
		{"po502", 2},
		{"po999", 0},  // not in membership
		{"Eth1/1", 0}, // not a port-channel
	}

	for _, tt := range tests {
		got := ResolveLAGPort(tt.port, membership)
		if len(got) != tt.wantLen {
			t.Errorf("ResolveLAGPort(%q) returned %d members, want %d", tt.port, len(got), tt.wantLen)
		}
	}
}

func TestExtractPortFromDn(t *testing.T) {
	tests := []struct {
		dn   string
		want string
	}{
		{"sys/intf/phys-[eth1/49]", "eth1/49"},
		{"sys/intf/phys-[Eth1/49]", "Eth1/49"},
		{"topology/pod-1/node-1/sys/phys-[eth1/49]", "eth1/49"},
		{"no-brackets", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractPortFromDn(tt.dn)
		if got != tt.want {
			t.Errorf("extractPortFromDn(%q) = %q, want %q", tt.dn, got, tt.want)
		}
	}
}

func TestParseLAGMembershipFromPhysIf(t *testing.T) {
	// Simulate NX-OS PhysIf data where pcId indicates port-channel membership
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Value: []interface{}{
						map[string]interface{}{
							"id":     "eth1/49",
							"pcId":   float64(102),
							"adminSt": "up",
						},
						map[string]interface{}{
							"id":     "eth1/50",
							"pcId":   float64(102),
							"adminSt": "up",
						},
						map[string]interface{}{
							"id":     "eth1/51",
							"pcId":   float64(101),
							"adminSt": "up",
						},
						map[string]interface{}{
							"id":     "eth1/1",
							"pcId":   float64(0), // not in any port-channel
							"adminSt": "up",
						},
						map[string]interface{}{
							"id":       "eth1/2",
							"adminSt": "up",
							// no pcId field at all
						},
					},
				},
			},
		},
	}

	membership := ParseLAGMembershipFromPhysIf(notifs)

	if len(membership) != 2 {
		t.Fatalf("expected 2 port-channels, got %d: %v", len(membership), membership)
	}

	if members := membership["po102"]; len(members) != 2 {
		t.Errorf("po102: expected 2 members, got %d: %v", len(members), members)
	} else {
		if members[0] != "Eth1/49" || members[1] != "Eth1/50" {
			t.Errorf("po102 members = %v, want [Eth1/49, Eth1/50]", members)
		}
	}

	if members := membership["po101"]; len(members) != 1 {
		t.Errorf("po101: expected 1 member, got %d: %v", len(members), members)
	} else {
		if members[0] != "Eth1/51" {
			t.Errorf("po101 members = %v, want [Eth1/51]", members)
		}
	}
}

func TestParseLAGMembershipFromPhysIf_NestedPhysItems(t *testing.T) {
	// Some NX-OS versions nest operational data under "phys-items"
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Value: []interface{}{
						map[string]interface{}{
							"id": "eth1/49",
							"phys-items": map[string]interface{}{
								"pcId": float64(501),
							},
						},
						map[string]interface{}{
							"id": "eth1/50",
							"phys-items": map[string]interface{}{
								"pcId": float64(501),
							},
						},
					},
				},
			},
		},
	}

	membership := ParseLAGMembershipFromPhysIf(notifs)

	if len(membership) != 1 {
		t.Fatalf("expected 1 port-channel, got %d: %v", len(membership), membership)
	}
	members := membership["po501"]
	if len(members) != 2 {
		t.Fatalf("po501: expected 2 members, got %d: %v", len(members), members)
	}
}
