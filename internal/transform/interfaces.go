package transform

import (
	"fmt"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

// Interface gNMI paths.
const (
	InterfacesPathOpenConfig = "/openconfig-interfaces:interfaces/interface"
)

// ParseInterfacesOpenConfig extracts interface state from OpenConfig gNMI responses.
func ParseInterfacesOpenConfig(notifs []gnmi.Notification) []topology.Interface {
	var ifaces []topology.Interface

	for _, n := range notifs {
		for _, u := range n.Updates {
			ifaceName := ExtractPathKey(u.Path, "name")

			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Could be a list of interfaces or a single interface
			ifaceList := AsMapSlice(vals)
			if ifaceList == nil {
				// Might be wrapped: {"interface": [...]}
				if inner := GetSlice(vals, "interface"); inner != nil {
					ifaceList = AsMapSlice(inner)
				}
			}

			if ifaceList != nil {
				for _, ifaceMap := range ifaceList {
					iface := parseOneInterface(ifaceMap, "")
					if iface.Name != "" {
						ifaces = append(ifaces, iface)
					}
				}
			} else if ifaceName != "" {
				iface := parseOneInterface(vals, ifaceName)
				if iface.Name != "" {
					ifaces = append(ifaces, iface)
				}
			}
		}
	}

	return ifaces
}

func parseOneInterface(vals map[string]interface{}, nameHint string) topology.Interface {
	state := GetMap(vals, "state")
	if state == nil {
		state = vals
	}

	name := GetFirstString(state, "name", "ifname")
	if name == "" {
		name = GetString(vals, "name")
	}
	if name == "" {
		name = nameHint
	}

	iface := topology.Interface{
		Name:       NormalizeInterfaceName(name),
		OperStatus: normalizeOperStatus(GetFirstString(state, "oper-status", "oper_status", "operStatus")),
		MTU:        GetInt(state, "mtu"),
	}

	// Speed: try "speed" field, may be like "SPEED_25GB" or "25000" or "25G"
	speed := GetFirstString(state, "speed", "portSpeed")
	iface.Speed = normalizeSpeed(speed)

	// Counters
	countersMap := GetMap(state, "counters")
	if countersMap == nil {
		countersMap = GetMap(vals, "counters")
	}
	if countersMap != nil {
		iface.Counters = &topology.IfaceCounters{
			InOctets:    GetNumber(countersMap, "in-octets"),
			OutOctets:   GetNumber(countersMap, "out-octets"),
			InPkts:      GetNumber(countersMap, "in-pkts") + GetNumber(countersMap, "in-unicast-pkts") + GetNumber(countersMap, "in-broadcast-pkts") + GetNumber(countersMap, "in-multicast-pkts"),
			OutPkts:     GetNumber(countersMap, "out-pkts") + GetNumber(countersMap, "out-unicast-pkts") + GetNumber(countersMap, "out-broadcast-pkts") + GetNumber(countersMap, "out-multicast-pkts"),
			InErrors:    GetNumber(countersMap, "in-errors"),
			OutErrors:   GetNumber(countersMap, "out-errors"),
			InDiscards:  GetNumber(countersMap, "in-discards"),
			OutDiscards: GetNumber(countersMap, "out-discards"),
		}
	}

	return iface
}

func normalizeOperStatus(s string) string {
	switch {
	case s == "" :
		return ""
	case containsCI(s, "UP"):
		return "UP"
	case containsCI(s, "DOWN"):
		return "DOWN"
	default:
		return s
	}
}

func normalizeSpeed(s string) string {
	if s == "" {
		return ""
	}
	// Already formatted like "25G" or "100G"
	if len(s) <= 5 && (s[len(s)-1] == 'G' || s[len(s)-1] == 'g') {
		return s
	}
	// OpenConfig: "SPEED_25GB" → "25G"
	if containsCI(s, "SPEED_") {
		s = s[6:] // strip "SPEED_"
		if len(s) > 2 && (s[len(s)-2:] == "GB" || s[len(s)-2:] == "gb") {
			return s[:len(s)-1] // "25GB" → "25G"
		}
		return s
	}
	// Numeric: "25000" (Mbps) → "25G"
	if len(s) >= 4 {
		var mbps uint64
		for _, c := range s {
			if c >= '0' && c <= '9' {
				mbps = mbps*10 + uint64(c-'0')
			} else {
				return s // not purely numeric
			}
		}
		if mbps >= 1000 {
			return fmt.Sprintf("%dG", mbps/1000)
		}
		return fmt.Sprintf("%dM", mbps)
	}
	return s
}
