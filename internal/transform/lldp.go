package transform

import (
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// LLDP gNMI paths per platform.
const (
	LLDPPathOpenConfig = "/openconfig-lldp:lldp/interfaces/interface/neighbors"
	LLDPPathNXOS       = "/System/lldp-items/inst-items/if-items/If-list"
)

// LLDPNeighbor holds parsed LLDP neighbor data from a single gNMI response.
type LLDPNeighbor struct {
	LocalPort         string
	ChassisID         string
	PortID            string
	PortDescription   string
	SystemName        string
	SystemDescription string
	ManagementAddress string
	Capabilities      string // LLDP system capabilities (e.g. "bridge", "router", "station-only")
}

// ParseLLDPOpenConfig extracts LLDP neighbors from OpenConfig gNMI responses.
// This works for SONiC and other OpenConfig-compliant switches.
func ParseLLDPOpenConfig(notifs []gnmi.Notification) []LLDPNeighbor {
	var neighbors []LLDPNeighbor

	for _, n := range notifs {
		for _, u := range n.Updates {
			localPort := ExtractPathKey(u.Path, "name")

			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Bulk response: {"neighbor": [...]}
			nbrList := GetSlice(vals, "neighbor")

			// Single-neighbor from Subscribe ONCE
			if nbrList == nil {
				if GetMap(vals, "state") != nil {
					nbrList = []interface{}{vals}
				}
			}

			// The response might be wrapped at interface level
			if nbrList == nil {
				if iface := GetMap(vals, "neighbors"); iface != nil {
					nbrList = GetSlice(iface, "neighbor")
				}
			}

			for _, raw := range nbrList {
				nbr, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}

				state := GetMap(nbr, "state")
				if state == nil {
					// Some devices put fields directly in the neighbor map
					state = nbr
				}

				neighbor := LLDPNeighbor{
					LocalPort:         NormalizeInterfaceName(localPort),
					ChassisID:         normalizeMACAddress(GetFirstString(state, "chassis-id", "chassis_id")),
					PortID:            GetFirstString(state, "port-id", "port_id"),
					PortDescription:   GetFirstString(state, "port-description", "port_description"),
					SystemName:        GetFirstString(state, "system-name", "system_name"),
					SystemDescription: GetFirstString(state, "system-description", "system_description"),
					ManagementAddress: GetFirstString(state, "management-address", "management_address", "mgmt-ip"),
				}

				// Parse system capabilities from OpenConfig capabilities list
				if caps := GetMap(nbr, "capabilities"); caps != nil {
					if capList := GetSlice(caps, "capability"); capList != nil {
						neighbor.Capabilities = extractCapabilities(capList)
					}
				}
				if neighbor.Capabilities == "" {
					if capsStr := GetFirstString(state, "system-capabilities", "system_capabilities"); capsStr != "" {
						neighbor.Capabilities = capsStr
					}
				}

				if neighbor.ChassisID != "" || neighbor.PortID != "" {
					neighbors = append(neighbors, neighbor)
				}
			}
		}
	}

	return neighbors
}

// ParseLLDPNXOS extracts LLDP neighbors from Cisco NX-OS native gNMI responses.
func ParseLLDPNXOS(notifs []gnmi.Notification) []LLDPNeighbor {
	var neighbors []LLDPNeighbor

	for _, n := range notifs {
		for _, u := range n.Updates {
			entries := AsMapSlice(u.Value)
			if entries == nil {
				continue
			}

			for _, vals := range entries {
				localPort := GetString(vals, "id")
				if localPort == "" {
					localPort = ExtractPathKey(u.Path, "id")
				}

				adjItems := GetMap(vals, "adj-items")
				if adjItems == nil {
					continue
				}
				adjList := GetSlice(adjItems, "AdjEp-list")
				if adjList == nil {
					continue
				}

				for _, raw := range adjList {
					adj, ok := raw.(map[string]interface{})
					if !ok {
						continue
					}

					mgmtIP := GetString(adj, "mgmtIp")
					if mgmtIP == "unspecified" {
						mgmtIP = ""
					}

					neighbor := LLDPNeighbor{
						LocalPort:         NormalizeInterfaceName(localPort),
						ChassisID:         normalizeMACAddress(GetString(adj, "chassisIdV")),
						PortID:            GetString(adj, "portIdV"),
						PortDescription:   GetString(adj, "portDesc"),
						SystemName:        GetString(adj, "sysName"),
						SystemDescription: GetString(adj, "sysDesc"),
						ManagementAddress: mgmtIP,
						Capabilities:      GetString(adj, "sysCap"),
					}

					if neighbor.ChassisID != "" || neighbor.PortID != "" {
						neighbors = append(neighbors, neighbor)
					}
				}
			}
		}
	}

	return neighbors
}

// ClassifyDevice determines the device type using LLDP system capabilities,
// description, and name. Capabilities are checked first as they are the most
// reliable signal (standardized in IEEE 802.1AB), then we fall back to
// string-matching heuristics on description and name fields.
func ClassifyDevice(description, name, capabilities string) string {
	// Capabilities-based classification (most reliable)
	capsLower := strings.ToLower(capabilities)
	if capsLower != "" {
		isBridge := strings.Contains(capsLower, "bridge")
		isRouter := strings.Contains(capsLower, "router")
		isStation := strings.Contains(capsLower, "station")

		// Bridge+Router = network switch (TOR)
		if isBridge || isRouter {
			// But if also described as BMC, prefer BMC
			for _, hint := range []string{"iDRAC", "iLO", "BMC", "IPMI", "Redfish"} {
				if containsCI(description, hint) || containsCI(name, hint) {
					return "bmc"
				}
			}
			return "switch"
		}
		// Station-only = end host (server or BMC)
		if isStation {
			for _, hint := range []string{"iDRAC", "iLO", "BMC", "IPMI", "Redfish"} {
				if containsCI(description, hint) || containsCI(name, hint) {
					return "bmc"
				}
			}
			return "host"
		}
	}

	// Fallback: description/name heuristics
	// BMC detection
	for _, hint := range []string{"iDRAC", "iLO", "BMC", "IPMI", "Redfish"} {
		if containsCI(description, hint) || containsCI(name, hint) {
			return "bmc"
		}
	}

	// Switch detection
	for _, hint := range []string{"SONiC", "NX-OS", "Arista", "Cumulus", "FTOS", "Dell EMC", "Cisco"} {
		if containsCI(description, hint) {
			return "switch"
		}
	}

	// Host detection
	for _, hint := range []string{"Linux", "Ubuntu", "Windows", "Red Hat", "CentOS", "SLES"} {
		if containsCI(description, hint) {
			return "host"
		}
	}

	return "unknown"
}

// classifyDevice is the internal version using LLDPNeighbor.
func classifyDevice(nbr LLDPNeighbor) string {
	return ClassifyDevice(nbr.SystemDescription, nbr.SystemName, nbr.Capabilities)
}

// extractCapabilities builds a comma-separated string of enabled LLDP
// capabilities from an OpenConfig capability list.
func extractCapabilities(capList []interface{}) string {
	var caps []string
	for _, raw := range capList {
		capMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		state := GetMap(capMap, "state")
		if state == nil {
			state = capMap
		}
		enabled := false
		if v, ok := state["enabled"]; ok {
			if b, ok := v.(bool); ok {
				enabled = b
			}
		}
		if enabled {
			if name := GetFirstString(state, "name", "capability"); name != "" {
				caps = append(caps, name)
			}
		}
	}
	return strings.Join(caps, ",")
}

func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
