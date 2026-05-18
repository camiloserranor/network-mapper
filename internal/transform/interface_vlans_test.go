package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestParseInterfaceVLANsNXOS(t *testing.T) {
	tests := []struct {
		name  string
		input []gnmi.Notification
		want  []InterfaceVLANConfig
	}{
		{
			name: "single interface with access VLAN",
			input: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "PhysIf-list[id=eth1/1]",
					Value: map[string]interface{}{
						"id":         "eth1/1",
						"mode":       "access",
						"accessVlan": "vlan-100",
					},
				}},
			}},
			want: []InterfaceVLANConfig{
				{Name: "Eth1/1", Mode: "access", AccessVLAN: 100},
			},
		},
		{
			name: "trunk interface with native and allowed VLANs",
			input: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "PhysIf-list[id=eth1/2]",
					Value: map[string]interface{}{
						"id":         "eth1/2",
						"mode":       "trunk",
						"nativeVlan": "vlan-1",
						"trunkVlans": "1-10,100,200-205",
					},
				}},
			}},
			want: []InterfaceVLANConfig{
				{
					Name:       "Eth1/2",
					Mode:       "trunk",
					NativeVLAN: 1,
					TrunkVLANs: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 100, 200, 201, 202, 203, 204, 205},
				},
			},
		},
		{
			name: "list of interfaces in single update",
			input: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "/System/intf-items/phys-items/PhysIf-list",
					Value: []interface{}{
						map[string]interface{}{
							"id":         "eth1/3",
							"mode":       "access",
							"accessVlan": "vlan-200",
						},
						map[string]interface{}{
							"id":         "eth1/4",
							"mode":       "trunk",
							"nativeVlan": "1",
							"trunkVlans": "100,200",
						},
					},
				}},
			}},
			want: []InterfaceVLANConfig{
				{Name: "Eth1/3", Mode: "access", AccessVLAN: 200},
				{Name: "Eth1/4", Mode: "trunk", NativeVLAN: 1, TrunkVLANs: []int{100, 200}},
			},
		},
		{
			name: "NX-OS nested phys-items structure",
			input: []gnmi.Notification{{
				Updates: []gnmi.Update{{
					Path: "PhysIf-list[id=eth1/1]",
					Value: map[string]interface{}{
						"id":          "eth1/1",
						"switchingSt": "disabled",
						"phys-items": map[string]interface{}{
							"operMode":     "trunk",
							"accessVlan":   "vlan-1",
							"nativeVlan":   "vlan-1007",
							"allowedVlans": "1006-1007,1201-1203",
						},
					},
				}},
			}},
			want: []InterfaceVLANConfig{
				{
					Name:       "Eth1/1",
					Mode:       "trunk",
					AccessVLAN: 1,
					NativeVLAN: 1007,
					TrunkVLANs: []int{1006, 1007, 1201, 1202, 1203},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInterfaceVLANsNXOS(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d configs, want %d", len(got), len(tt.want))
			}
			for i, w := range tt.want {
				g := got[i]
				if g.Name != w.Name {
					t.Errorf("[%d] Name: got %q, want %q", i, g.Name, w.Name)
				}
				if g.Mode != w.Mode {
					t.Errorf("[%d] Mode: got %q, want %q", i, g.Mode, w.Mode)
				}
				if g.AccessVLAN != w.AccessVLAN {
					t.Errorf("[%d] AccessVLAN: got %d, want %d", i, g.AccessVLAN, w.AccessVLAN)
				}
				if g.NativeVLAN != w.NativeVLAN {
					t.Errorf("[%d] NativeVLAN: got %d, want %d", i, g.NativeVLAN, w.NativeVLAN)
				}
				if !intSliceEqual(g.TrunkVLANs, w.TrunkVLANs) {
					t.Errorf("[%d] TrunkVLANs: got %v, want %v", i, g.TrunkVLANs, w.TrunkVLANs)
				}
			}
		})
	}
}

func TestParseTrunkVLANRange(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"100", []int{100}},
		{"1-5", []int{1, 2, 3, 4, 5}},
		{"1-3,10,20-22", []int{1, 2, 3, 10, 20, 21, 22}},
		{"vlan-100", []int{100}},
		{"", nil},
	}
	for _, tt := range tests {
		got := parseTrunkVLANRange(tt.input)
		if !intSliceEqual(got, tt.want) {
			t.Errorf("parseTrunkVLANRange(%q): got %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMergeInterfaceVLANConfigs(t *testing.T) {
	ifaces := []topology.Interface{
		{Name: "Eth1/1", OperStatus: "UP"},
		{Name: "Eth1/2", OperStatus: "UP"},
		{Name: "Eth1/3", OperStatus: "DOWN"},
	}

	configs := []InterfaceVLANConfig{
		{Name: "Eth1/1", Mode: "access", AccessVLAN: 100},
		{Name: "Eth1/2", Mode: "trunk", NativeVLAN: 1, TrunkVLANs: []int{100, 200}},
		// Eth1/3 not in config list
	}

	MergeInterfaceVLANConfigs(ifaces, configs)

	if ifaces[0].Mode != "access" || ifaces[0].AccessVLAN != 100 {
		t.Errorf("Eth1/1: got mode=%q access=%d, want access/100", ifaces[0].Mode, ifaces[0].AccessVLAN)
	}
	if ifaces[1].Mode != "trunk" || ifaces[1].NativeVLAN != 1 {
		t.Errorf("Eth1/2: got mode=%q native=%d, want trunk/1", ifaces[1].Mode, ifaces[1].NativeVLAN)
	}
	if !intSliceEqual(ifaces[1].TrunkVLANs, []int{100, 200}) {
		t.Errorf("Eth1/2 TrunkVLANs: got %v, want [100 200]", ifaces[1].TrunkVLANs)
	}
	if ifaces[2].Mode != "" {
		t.Errorf("Eth1/3: got mode=%q, want empty", ifaces[2].Mode)
	}
}

func intSliceEqual(a, b []int) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
