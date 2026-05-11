package transform

import (
	"sort"
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

// InterfaceVLANPathNXOS is the NX-OS native gNMI path for interface switchport config.
const InterfaceVLANPathNXOS = "/System/intf-items/phys-items/PhysIf-list"

// InterfaceVLANConfig holds the switchport VLAN configuration for a single interface.
type InterfaceVLANConfig struct {
	Name       string
	Mode       string // access, trunk, routed
	AccessVLAN int
	NativeVLAN int
	TrunkVLANs []int
}

// ParseInterfaceVLANsNXOS extracts per-port VLAN configuration from NX-OS gNMI responses.
// NX-OS path: /System/intf-items/phys-items/PhysIf-list
// Each PhysIf entry may contain: accessVlan, nativeVlan, trunkVlans, switchportMode, mode.
func ParseInterfaceVLANsNXOS(notifs []gnmi.Notification) []InterfaceVLANConfig {
	var configs []InterfaceVLANConfig

	for _, n := range notifs {
		for _, u := range n.Updates {
			// Check if the path key gives us an interface name
			ifName := ExtractPathKey(u.Path, "id")
			if ifName == "" {
				ifName = ExtractPathKey(u.Path, "name")
			}

			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				// Value may be a direct slice of interface configs
				if ifaceList := AsMapSlice(u.Value); ifaceList != nil {
					for _, ifaceMap := range ifaceList {
						cfg := parseOneInterfaceVLAN(ifaceMap, "")
						if cfg.Name != "" {
							configs = append(configs, cfg)
						}
					}
				}
				continue
			}

			// Could be a list of interfaces or a single interface
			ifaceList := AsMapSlice(vals)
			if ifaceList == nil {
				if inner := GetSlice(vals, "PhysIf-list"); inner != nil {
					ifaceList = AsMapSlice(inner)
				}
			}

			if ifaceList != nil {
				for _, ifaceMap := range ifaceList {
					cfg := parseOneInterfaceVLAN(ifaceMap, "")
					if cfg.Name != "" {
						configs = append(configs, cfg)
					}
				}
			} else if ifName != "" {
				cfg := parseOneInterfaceVLAN(vals, ifName)
				if cfg.Name != "" {
					configs = append(configs, cfg)
				}
			}
		}
	}

	return configs
}

func parseOneInterfaceVLAN(vals map[string]interface{}, nameHint string) InterfaceVLANConfig {
	name := GetFirstString(vals, "id", "name")
	if name == "" {
		name = nameHint
	}
	if name == "" {
		return InterfaceVLANConfig{}
	}

	cfg := InterfaceVLANConfig{
		Name: NormalizeInterfaceName(name),
	}

	// Switchport mode: NX-OS uses "mode" or "switchportMode"
	mode := GetFirstString(vals, "mode", "switchportMode", "layer")
	cfg.Mode = normalizeSwitchportMode(mode)

	// Access VLAN
	accessStr := GetFirstString(vals, "accessVlan", "access_vlan")
	cfg.AccessVLAN = parseVLANIDFromEncap(accessStr)

	// Native VLAN (trunk)
	nativeStr := GetFirstString(vals, "nativeVlan", "native_vlan")
	cfg.NativeVLAN = parseVLANIDFromEncap(nativeStr)

	// Trunk VLANs (may be a comma-separated string like "1-100,200-300" or a list)
	trunkStr := GetFirstString(vals, "trunkVlans", "trunk_vlans", "allowedVlans")
	if trunkStr != "" {
		cfg.TrunkVLANs = parseTrunkVLANRange(trunkStr)
	}

	return cfg
}

func normalizeSwitchportMode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "" || s == "layer2":
		return ""
	case strings.Contains(s, "access"):
		return "access"
	case strings.Contains(s, "trunk"):
		return "trunk"
	case strings.Contains(s, "routed") || strings.Contains(s, "layer3") || s == "l3":
		return "routed"
	case strings.Contains(s, "fex-fabric"):
		return "fex-fabric"
	default:
		return s
	}
}

// parseTrunkVLANRange parses VLAN range strings like "1-100,200,300-400".
func parseTrunkVLANRange(s string) []int {
	s = strings.TrimPrefix(s, "vlan-")

	var vlans []int
	seen := make(map[int]bool)

	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if idx := strings.Index(part, "-"); idx > 0 {
			startStr := strings.TrimSpace(part[:idx])
			endStr := strings.TrimSpace(part[idx+1:])
			start, err1 := strconv.Atoi(startStr)
			end, err2 := strconv.Atoi(endStr)
			if err1 == nil && err2 == nil && start > 0 && end >= start && end <= 4094 {
				// Cap range expansion to avoid huge slices
				if end-start > 4094 {
					end = start + 4094
				}
				for v := start; v <= end; v++ {
					if !seen[v] {
						vlans = append(vlans, v)
						seen[v] = true
					}
				}
			}
		} else {
			v, err := strconv.Atoi(part)
			if err == nil && v > 0 && v <= 4094 && !seen[v] {
				vlans = append(vlans, v)
				seen[v] = true
			}
		}
	}

	sort.Ints(vlans)
	return vlans
}

// MergeInterfaceVLANConfigs merges VLAN configuration data into existing Interface objects.
func MergeInterfaceVLANConfigs(ifaces []topology.Interface, configs []InterfaceVLANConfig) {
	cfgByName := make(map[string]*InterfaceVLANConfig, len(configs))
	for i := range configs {
		cfgByName[configs[i].Name] = &configs[i]
	}

	for i := range ifaces {
		cfg, ok := cfgByName[ifaces[i].Name]
		if !ok {
			continue
		}
		if cfg.Mode != "" {
			ifaces[i].Mode = cfg.Mode
		}
		if cfg.AccessVLAN > 0 {
			ifaces[i].AccessVLAN = cfg.AccessVLAN
		}
		if cfg.NativeVLAN > 0 {
			ifaces[i].NativeVLAN = cfg.NativeVLAN
		}
		if len(cfg.TrunkVLANs) > 0 {
			ifaces[i].TrunkVLANs = cfg.TrunkVLANs
		}
	}
}
