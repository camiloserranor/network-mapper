package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestEnrichInterfaceVLANsFromVLANConfig(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []topology.Interface
		vlans      []topology.VLAN
		wantMode   map[string]string // interface name → expected VLANMode
		wantAccess map[string]int    // interface name → expected AccessVLAN
		wantTrunk  map[string][]int  // interface name → expected TrunkVLANs
	}{
		{
			name: "single VLAN sets access mode",
			interfaces: []topology.Interface{
				{Name: "Ethernet1/1"},
				{Name: "Ethernet1/2"},
			},
			vlans: []topology.VLAN{
				{ID: 100, MemberPorts: []string{"Ethernet1/1"}},
			},
			wantMode:   map[string]string{"Ethernet1/1": "access", "Ethernet1/2": ""},
			wantAccess: map[string]int{"Ethernet1/1": 100},
		},
		{
			name: "multiple VLANs set trunk mode",
			interfaces: []topology.Interface{
				{Name: "Ethernet1/1"},
			},
			vlans: []topology.VLAN{
				{ID: 100, MemberPorts: []string{"Ethernet1/1"}},
				{ID: 200, MemberPorts: []string{"Ethernet1/1"}},
			},
			wantMode:  map[string]string{"Ethernet1/1": "trunk"},
			wantTrunk: map[string][]int{"Ethernet1/1": {100, 200}},
		},
		{
			name: "normalized name matching (eth→Eth)",
			interfaces: []topology.Interface{
				{Name: "eth1/1"},
			},
			vlans: []topology.VLAN{
				{ID: 10, MemberPorts: []string{"eth1/1"}},
			},
			wantMode:   map[string]string{"eth1/1": "access"},
			wantAccess: map[string]int{"eth1/1": 10},
		},
		{
			name: "skip interfaces with existing VLAN mode",
			interfaces: []topology.Interface{
				{Name: "Ethernet1/1", VLANMode: "trunk", TrunkVLANs: []int{50}},
			},
			vlans: []topology.VLAN{
				{ID: 100, MemberPorts: []string{"Ethernet1/1"}},
			},
			wantMode:  map[string]string{"Ethernet1/1": "trunk"},
			wantTrunk: map[string][]int{"Ethernet1/1": {50}},
		},
		{
			name:       "no VLANs leaves interfaces unchanged",
			interfaces: []topology.Interface{{Name: "Ethernet1/1"}},
			vlans:      nil,
			wantMode:   map[string]string{"Ethernet1/1": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			EnrichInterfaceVLANsFromVLANConfig(tt.interfaces, tt.vlans)

			for _, iface := range tt.interfaces {
				if want, ok := tt.wantMode[iface.Name]; ok {
					if iface.VLANMode != want {
						t.Errorf("interface %s: VLANMode = %q, want %q", iface.Name, iface.VLANMode, want)
					}
				}
				if want, ok := tt.wantAccess[iface.Name]; ok {
					if iface.AccessVLAN != want {
						t.Errorf("interface %s: AccessVLAN = %d, want %d", iface.Name, iface.AccessVLAN, want)
					}
				}
				if want, ok := tt.wantTrunk[iface.Name]; ok {
					if len(iface.TrunkVLANs) != len(want) {
						t.Errorf("interface %s: TrunkVLANs = %v, want %v", iface.Name, iface.TrunkVLANs, want)
					} else {
						for i, v := range want {
							if iface.TrunkVLANs[i] != v {
								t.Errorf("interface %s: TrunkVLANs[%d] = %d, want %d", iface.Name, i, iface.TrunkVLANs[i], v)
							}
						}
					}
				}
			}
		})
	}
}
