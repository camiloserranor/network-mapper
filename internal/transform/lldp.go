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
					ChassisID:         GetFirstString(state, "chassis-id", "chassis_id"),
					PortID:            GetFirstString(state, "port-id", "port_id"),
					PortDescription:   GetFirstString(state, "port-description", "port_description"),
					SystemName:        GetFirstString(state, "system-name", "system_name"),
					SystemDescription: GetFirstString(state, "system-description", "system_description"),
					ManagementAddress: GetFirstString(state, "management-address", "management_address", "mgmt-ip"),
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
						ChassisID:         GetString(adj, "chassisIdV"),
						PortID:            GetString(adj, "portIdV"),
						PortDescription:   GetString(adj, "portDesc"),
						SystemName:        GetString(adj, "sysName"),
						SystemDescription: GetString(adj, "sysDesc"),
						ManagementAddress: mgmtIP,
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

// ClassifyDevice guesses the device type from LLDP system description and name.
func ClassifyDevice(description, name string) string {
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
	return ClassifyDevice(nbr.SystemDescription, nbr.SystemName)
}

func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
