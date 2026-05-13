package transform

import (
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

// ParseInterfacesNXOS extracts interface state from NX-OS native gNMI responses
// (path: /System/intf-items/phys-items/PhysIf-list).
func ParseInterfacesNXOS(notifs []gnmi.Notification) []topology.Interface {
	var ifaces []topology.Interface

	for _, n := range notifs {
		for _, u := range n.Updates {
			// Extract interface name from path key if present
			nameHint := ExtractPathKey(u.Path, "id")

			// Value could be a map, a slice, or wrapped in a path key
			var ifaceList []map[string]interface{}

			switch val := u.Value.(type) {
			case []interface{}:
				ifaceList = AsMapSlice(val)
			case map[string]interface{}:
				// Try extracting from known wrapper keys
				for _, key := range []string{
					"System/intf-items/phys-items/PhysIf-list",
					"PhysIf-list",
				} {
					if inner := GetSlice(val, key); inner != nil {
						ifaceList = AsMapSlice(inner)
						break
					}
				}
				if ifaceList == nil {
					// The map itself might be a single interface
					ifaceList = []map[string]interface{}{val}
				}
			}

			for _, ifaceMap := range ifaceList {
				iface := parseNXOSInterface(ifaceMap)
				if iface.Name == "" && nameHint != "" {
					iface.Name = NormalizeInterfaceName(nameHint)
				}
				if iface.Name != "" {
					ifaces = append(ifaces, iface)
				}
			}
		}
	}

	return ifaces
}

func parseNXOSInterface(m map[string]interface{}) topology.Interface {
	id := GetString(m, "id")
	adminSt := GetString(m, "adminSt")

	iface := topology.Interface{
		Name:       NormalizeInterfaceName(id),
		OperStatus: normalizeNXOSOperStatus(adminSt),
		Speed:      normalizeSpeed(GetString(m, "speed")),
		MTU:        GetInt(m, "mtu"),
	}

	// Mode: "trunk", "access", or "fex-fabric" etc.
	mode := strings.ToLower(GetString(m, "mode"))
	if mode == "trunk" || mode == "access" {
		iface.Mode = mode
	}

	// Native VLAN (format: "vlan-7" → 7)
	nativeVlan := GetString(m, "nativeVlan")
	if nativeVlan != "" {
		iface.NativeVLAN = parseVlanNumber(nativeVlan)
	}

	// Access VLAN
	accessVlan := GetString(m, "accessVlan")
	if accessVlan != "" {
		iface.AccessVLAN = parseVlanNumber(accessVlan)
	}

	// Trunk VLANs — only relevant for trunk mode ports.
	// Skip default "1-4094" range as it's not informative.
	if mode == "trunk" {
		trunkVlans := GetString(m, "trunkVlans")
		if trunkVlans != "" && trunkVlans != "1-4094" {
			iface.TrunkVLANs = parseVlanRange(trunkVlans)
		}
	}

	return iface
}

// normalizeNXOSOperStatus converts NX-OS adminSt to standard UP/DOWN.
func normalizeNXOSOperStatus(s string) string {
	switch strings.ToLower(s) {
	case "up":
		return "UP"
	case "down":
		return "DOWN"
	default:
		if s != "" {
			return s
		}
		return ""
	}
}

// parseVlanNumber extracts the VLAN number from "vlan-X" format.
func parseVlanNumber(s string) int {
	s = strings.TrimPrefix(s, "vlan-")
	n, _ := strconv.Atoi(s)
	return n
}

// parseVlanRange parses a VLAN range string like "1-5,7,100-102" into a slice of IDs.
func parseVlanRange(s string) []int {
	var vlans []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx := strings.Index(part, "-"); idx > 0 {
			start, _ := strconv.Atoi(part[:idx])
			end, _ := strconv.Atoi(part[idx+1:])
			if start > 0 && end >= start && (end-start) < 4096 {
				for v := start; v <= end; v++ {
					vlans = append(vlans, v)
				}
			}
		} else {
			if n, err := strconv.Atoi(part); err == nil && n > 0 {
				vlans = append(vlans, n)
			}
		}
	}
	return vlans
}
