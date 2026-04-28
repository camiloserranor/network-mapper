package transform

import (
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// gNMI paths for MAC address table.
const (
	MACTablePathNXOS = "/System/mac-items"
)

// MACEntry represents a single learned MAC address entry from the switch.
type MACEntry struct {
	MAC       string
	VLAN      int
	Port      string
	Type      string // static, dynamic
	SwitchID  string
}

// ParseMACTableNXOS extracts MAC address table entries from NX-OS gNMI responses.
// NX-OS path: /System/mac-items → table-items → vlan-items → MacAddressEntry-list
func ParseMACTableNXOS(notifs []gnmi.Notification, switchID string) []MACEntry {
	var entries []MACEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			// Try to walk the nested structure
			root := AsMapSlice(u.Value)
			if root == nil {
				continue
			}

			for _, item := range root {
				// Navigate: table-items → vlan-items
				tableItems := GetMap(item, "table-items")
				if tableItems == nil {
					// Might be flattened
					entries = append(entries, extractMACEntries(item, switchID)...)
					continue
				}

				vlanItems := GetSlice(tableItems, "vlan-items")
				if vlanItems == nil {
					// Try as VlanMacEntry-list directly
					vlanItems = GetSlice(tableItems, "VlanMacEntry-list")
				}

				for _, vlanRaw := range vlanItems {
					vlan, ok := vlanRaw.(map[string]interface{})
					if !ok {
						continue
					}
					entries = append(entries, extractMACEntries(vlan, switchID)...)
				}
			}
		}
	}

	return entries
}

func extractMACEntries(vlanMap map[string]interface{}, switchID string) []MACEntry {
	var entries []MACEntry

	// Get VLAN ID from the map
	vlanID := parseVLANID(GetFirstString(vlanMap, "encap", "id", "vlan"))

	// Find MAC entries list
	macList := GetSlice(vlanMap, "MacAddressEntry-list")
	if macList == nil {
		macList = GetSlice(vlanMap, "mac-items")
	}
	if macList == nil {
		// Single entry
		if mac := GetFirstString(vlanMap, "mac_address", "addr", "macAddr"); mac != "" {
			entries = append(entries, MACEntry{
				MAC:      normalizeMACAddress(mac),
				VLAN:     vlanID,
				Port:     NormalizeInterfaceName(GetFirstString(vlanMap, "port", "if", "intf")),
				Type:     GetFirstString(vlanMap, "type", "macType"),
				SwitchID: switchID,
			})
		}
		return entries
	}

	for _, raw := range macList {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		mac := GetFirstString(entry, "mac_address", "addr", "macAddr")
		if mac == "" {
			continue
		}

		port := NormalizeInterfaceName(GetFirstString(entry, "port", "if", "intf"))

		entries = append(entries, MACEntry{
			MAC:      normalizeMACAddress(mac),
			VLAN:     vlanID,
			Port:     port,
			Type:     GetFirstString(entry, "type", "macType"),
			SwitchID: switchID,
		})
	}

	return entries
}

// parseVLANID extracts numeric VLAN ID from strings like "vlan-100" or "100".
func parseVLANID(s string) int {
	s = strings.TrimPrefix(s, "vlan-")
	s = strings.TrimPrefix(s, "Vlan")
	s = strings.TrimPrefix(s, "vlan")
	id, _ := strconv.Atoi(s)
	return id
}

// normalizeMACAddress converts MAC to lowercase colon-separated format.
func normalizeMACAddress(mac string) string {
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, "-", ":")
	// Handle dot-separated (e.g., aabb.ccdd.eeff)
	if strings.Count(mac, ".") == 2 {
		mac = strings.ReplaceAll(mac, ".", "")
		if len(mac) == 12 {
			mac = mac[0:2] + ":" + mac[2:4] + ":" + mac[4:6] + ":" +
				mac[6:8] + ":" + mac[8:10] + ":" + mac[10:12]
		}
	}
	return mac
}
