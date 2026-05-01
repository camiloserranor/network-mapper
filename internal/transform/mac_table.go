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
//
// NX-OS path: /System/mac-items
// Response structure (verified on NX-OS 10.5):
//
//	value = {
//	  "table-items": {
//	    "vlan-items": {
//	      "MacAddressEntry-list": [
//	        {"vlan":"vlan-100","macAddress":"aa:bb:cc:dd:ee:ff","port":"eth1/3","type":"primary",...},
//	        ...
//	      ]
//	    }
//	  }
//	}
//
// Each entry in MacAddressEntry-list carries its own "vlan" field.
func ParseMACTableNXOS(notifs []gnmi.Notification, switchID string) []MACEntry {
	var entries []MACEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			root := AsMapSlice(u.Value)
			if root == nil {
				continue
			}

			for _, item := range root {
				entries = append(entries, extractMACFromRoot(item, switchID)...)
			}
		}
	}

	return entries
}

// extractMACFromRoot navigates the NX-OS MAC table structure to find entries.
// It handles multiple response layouts:
//   - Full path: table-items → vlan-items → MacAddressEntry-list (flat list with per-entry vlan)
//   - Full path: table-items → vlan-items (as array of VLANs) → MacAddressEntry-list (per VLAN)
//   - Direct: MacAddressEntry-list at root (when querying deeper paths)
func extractMACFromRoot(item map[string]interface{}, switchID string) []MACEntry {
	// Try full structure: table-items → ...
	if tableItems := GetMap(item, "table-items"); tableItems != nil {
		return extractMACFromTableItems(tableItems, switchID)
	}
	// Try direct MacAddressEntry-list at root
	return extractMACEntries(item, switchID)
}

func extractMACFromTableItems(tableItems map[string]interface{}, switchID string) []MACEntry {
	var entries []MACEntry

	// Case 1: vlan-items is an object with MacAddressEntry-list directly
	// (flat structure where each entry carries its own vlan field)
	if vlanObj := GetMap(tableItems, "vlan-items"); vlanObj != nil {
		if macList := GetSlice(vlanObj, "MacAddressEntry-list"); macList != nil {
			for _, raw := range macList {
				entry, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				mac := GetFirstString(entry, "macAddress", "mac_address", "addr")
				if mac == "" {
					continue
				}
				port := NormalizeInterfaceName(GetFirstString(entry, "port", "if", "intf"))
				vlanID := parseVLANID(GetFirstString(entry, "vlan", "encap", "id"))
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
	}

	// Case 2: vlan-items is an array of VLAN containers
	if vlanSlice := GetSlice(tableItems, "vlan-items"); vlanSlice != nil {
		for _, vlanRaw := range vlanSlice {
			vlan, ok := vlanRaw.(map[string]interface{})
			if !ok {
				continue
			}
			entries = append(entries, extractMACEntries(vlan, switchID)...)
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
