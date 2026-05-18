package transform

import (
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

// gNMI paths for VLAN configuration.
const (
	VLANPathNXOS       = "/System/bd-items"
	VLANPathOpenConfig = "/openconfig-network-instance:network-instances/network-instance/vlans/vlan"
	SVIIPPath          = "/openconfig-interfaces:interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address"
)

// ParseVLANsNXOS extracts VLAN definitions from NX-OS gNMI responses.
// NX-OS path: /System/bd-items → bd-items → BD-list
func ParseVLANsNXOS(notifs []gnmi.Notification, switchID string) []topology.VLAN {
	var vlans []topology.VLAN

	for _, n := range notifs {
		for _, u := range n.Updates {
			items := AsMapSlice(u.Value)
			if items == nil {
				continue
			}

			for _, item := range items {
				// BD-list at top level
				bdList := GetSlice(item, "BD-list")

				// Or nested: bd-items → BD-list
				if bdList == nil {
					if bdItems := GetMap(item, "bd-items"); bdItems != nil {
						bdList = GetSlice(bdItems, "BD-list")
					}
				}

				if bdList == nil {
					// Direct BD entry
					if v := extractVLAN(item, switchID); v.ID > 0 {
						vlans = append(vlans, v)
					}
					continue
				}

				for _, bdRaw := range bdList {
					bd, ok := bdRaw.(map[string]interface{})
					if !ok {
						continue
					}
					if v := extractVLAN(bd, switchID); v.ID > 0 {
						vlans = append(vlans, v)
					}
				}
			}
		}
	}

	return vlans
}

func extractVLAN(m map[string]interface{}, switchID string) topology.VLAN {
	idStr := GetFirstString(m, "fabEncap", "accEncap", "id")
	vlanID := parseVLANIDFromEncap(idStr)

	if vlanID == 0 {
		return topology.VLAN{}
	}

	name := GetFirstString(m, "name", "BdOperName", "descr")
	status := GetFirstString(m, "adminSt", "operSt", "status")

	// Collect member ports if available
	var memberPorts []string
	memberList := GetSlice(m, "member-items")
	// Nested: member-items → VlanMemberIf-list
	if memberList == nil {
		if mi := GetMap(m, "member-items"); mi != nil {
			memberList = GetSlice(mi, "VlanMemberIf-list")
		}
	}
	for _, mRaw := range memberList {
		if member, ok := mRaw.(map[string]interface{}); ok {
			port := GetFirstString(member, "if", "id", "port")
			if port != "" {
				memberPorts = append(memberPorts, NormalizeInterfaceName(port))
			}
		}
	}

	return topology.VLAN{
		ID:           vlanID,
		Name:         name,
		Status:       status,
		MemberPorts:  memberPorts,
		SourceSwitch: switchID,
	}
}

// parseVLANIDFromEncap handles "vlan-100", "100", etc.
func parseVLANIDFromEncap(s string) int {
	s = strings.TrimPrefix(s, "vlan-")
	s = strings.TrimPrefix(s, "Vlan")
	s = strings.TrimPrefix(s, "vlan")
	id, _ := strconv.Atoi(s)
	return id
}

// ParseVLANsOpenConfig extracts VLAN definitions from OpenConfig gNMI responses (SONiC/Dell).
// Supports both bulk format (vlan[] array) and Subscribe ONCE per-VLAN format.
func ParseVLANsOpenConfig(notifs []gnmi.Notification, switchID string) []topology.VLAN {
	var vlans []topology.VLAN

	for _, n := range notifs {
		for _, u := range n.Updates {
			m, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Bulk format: top-level "vlan" array
			if vlanList := GetSlice(m, "vlan"); vlanList != nil {
				for _, vRaw := range vlanList {
					entry, ok := vRaw.(map[string]interface{})
					if !ok {
						continue
					}
					if v := extractOpenConfigVLAN(entry, switchID); v.ID > 0 {
						vlans = append(vlans, v)
					}
				}
				continue
			}

			// Subscribe ONCE format: single VLAN with "state" at top level
			if state := GetMap(m, "state"); state != nil {
				if v := extractOpenConfigVLAN(m, switchID); v.ID > 0 {
					vlans = append(vlans, v)
					continue
				}
			}

			// Fallback: try extracting VLAN ID from path key
			pathID := ExtractPathKey(u.Path, "vlan-id")
			if pathID != "" {
				v := extractOpenConfigVLAN(m, switchID)
				if v.ID == 0 {
					v.ID = parseVLANIDFromEncap(pathID)
					v.SourceSwitch = switchID
				}
				if v.ID > 0 {
					vlans = append(vlans, v)
				}
			}
		}
	}

	return vlans
}

func extractOpenConfigVLAN(m map[string]interface{}, switchID string) topology.VLAN {
	state := GetMap(m, "state")
	if state == nil {
		return topology.VLAN{}
	}

	vlanID := GetInt(state, "vlan-id")
	if vlanID == 0 {
		// vlan-id might be a string
		idStr := GetFirstString(state, "vlan-id")
		vlanID = parseVLANIDFromEncap(idStr)
	}
	if vlanID == 0 {
		return topology.VLAN{}
	}

	name := GetFirstString(state, "name")
	status := GetFirstString(state, "status")

	var memberPorts []string
	if members := GetMap(m, "members"); members != nil {
		if memberList := GetSlice(members, "member"); memberList != nil {
			for _, mRaw := range memberList {
				member, ok := mRaw.(map[string]interface{})
				if !ok {
					continue
				}

				var port string
				// Try members.member[].state.interface
				if ms := GetMap(member, "state"); ms != nil {
					port = GetFirstString(ms, "interface")
				}
				// Fallback: members.member[].interface-ref.state.interface
				if port == "" {
					if ifRef := GetMap(member, "interface-ref"); ifRef != nil {
						if irs := GetMap(ifRef, "state"); irs != nil {
							port = GetFirstString(irs, "interface")
						}
					}
				}

				if port != "" {
					memberPorts = append(memberPorts, NormalizeInterfaceName(port))
				}
			}
		}
	}

	return topology.VLAN{
		ID:           vlanID,
		Name:         name,
		Status:       status,
		MemberPorts:  memberPorts,
		SourceSwitch: switchID,
	}
}

// EnrichInterfaceVLANsFromVLANConfig assigns VLAN membership to interfaces based on
// the VLAN configuration's member port lists. This is used for OpenConfig platforms
// where per-port VLAN data is not directly available.
func EnrichInterfaceVLANsFromVLANConfig(interfaces []topology.Interface, vlans []topology.VLAN) {
	// Build reverse map: normalized interface name → list of VLAN IDs
	ifaceVLANs := make(map[string][]int)
	for _, v := range vlans {
		for _, port := range v.MemberPorts {
			norm := NormalizeInterfaceName(port)
			ifaceVLANs[norm] = append(ifaceVLANs[norm], v.ID)
		}
	}

	for i := range interfaces {
		norm := NormalizeInterfaceName(interfaces[i].Name)
		vids, ok := ifaceVLANs[norm]
		if !ok || len(vids) == 0 {
			continue
		}
		// Skip interfaces that already have VLAN data (e.g. from per-port query)
		if interfaces[i].Mode != "" {
			continue
		}
		if len(vids) == 1 {
			interfaces[i].Mode = "access"
			interfaces[i].AccessVLAN = vids[0]
		} else {
			interfaces[i].Mode = "trunk"
			interfaces[i].TrunkVLANs = vids
		}
	}
}

// ParseSVIGateways extracts IP addresses from SVI (VLAN) interfaces to identify gateways.
// It looks for interfaces named "Vlan<N>" and extracts their IP address.
func ParseSVIGateways(notifs []gnmi.Notification) map[int]string {
	gateways := make(map[int]string) // VLAN ID → gateway IP

	for _, n := range notifs {
		for _, u := range n.Updates {
			// Extract interface name from path
			ifName := ExtractPathKey(u.Path, "name")
			if ifName == "" {
				continue
			}

			// Only interested in VLAN SVIs
			if !strings.HasPrefix(strings.ToLower(ifName), "vlan") {
				continue
			}

			vlanID := parseVLANIDFromEncap(ifName)
			if vlanID == 0 {
				continue
			}

			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Look for IP in various formats
			ip := GetFirstString(vals, "ip", "prefix", "address")
			if ip == "" {
				// Nested under state or config
				if state := GetMap(vals, "state"); state != nil {
					ip = GetFirstString(state, "ip", "prefix")
				}
			}

			// Strip prefix length if present (e.g., "10.0.100.1/24" → "10.0.100.1")
			if idx := strings.Index(ip, "/"); idx > 0 {
				ip = ip[:idx]
			}

			if ip != "" {
				gateways[vlanID] = ip
			}
		}
	}

	return gateways
}
