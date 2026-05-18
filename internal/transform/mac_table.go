package transform

import (
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// gNMI paths for MAC address table.
const (
	MACTablePathNXOS       = "/System/mac-items"
	MACTablePathOpenConfig = "/openconfig-network-instance:network-instances/network-instance[name=default]/fdb/mac-table"
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

// ParseMACTableOpenConfig extracts MAC address table entries from OpenConfig gNMI responses.
//
// OpenConfig path: /openconfig-network-instance:network-instances/network-instance[name=default]/fdb/mac-table
// Response structure (verified on SONiC/Dell switches):
//
//	value = {
//	  "entries": {
//	    "entry": [
//	      {
//	        "state": {"mac-address":"aa:bb:cc:dd:ee:ff","vlan":"100","entry-type":"DYNAMIC"},
//	        "interface": {"interface-ref": {"state": {"interface": "Ethernet48"}}}
//	      }
//	    ]
//	  }
//	}
//
// Subscribe ONCE responses may deliver each entry as a separate notification
// with "state" at the top level.
func ParseMACTableOpenConfig(notifs []gnmi.Notification, switchID string) []MACEntry {
	var entries []MACEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			root := AsMapSlice(u.Value)
			if root == nil {
				continue
			}

			for _, item := range root {
				entries = append(entries, extractMACFromOpenConfig(item, switchID)...)
			}
		}
	}

	return entries
}

// extractMACFromOpenConfig handles bulk (entries.entry[]) and single-entry formats.
func extractMACFromOpenConfig(item map[string]interface{}, switchID string) []MACEntry {
	// Bulk format: entries → entry[]
	if entriesMap := GetMap(item, "entries"); entriesMap != nil {
		if entryList := GetSlice(entriesMap, "entry"); entryList != nil {
			return parseOpenConfigEntryList(entryList, switchID)
		}
	}

	// Direct entry[] at root (some implementations)
	if entryList := GetSlice(item, "entry"); entryList != nil {
		return parseOpenConfigEntryList(entryList, switchID)
	}

	// Single entry (Subscribe ONCE): state at top level
	if state := GetMap(item, "state"); state != nil {
		if e, ok := parseOpenConfigSingleEntry(item, switchID); ok {
			return []MACEntry{e}
		}
	}

	return nil
}

func parseOpenConfigEntryList(entryList []interface{}, switchID string) []MACEntry {
	var entries []MACEntry

	for _, raw := range entryList {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if e, ok := parseOpenConfigSingleEntry(entry, switchID); ok {
			entries = append(entries, e)
		}
	}

	return entries
}

func parseOpenConfigSingleEntry(entry map[string]interface{}, switchID string) (MACEntry, bool) {
	state := GetMap(entry, "state")
	if state == nil {
		return MACEntry{}, false
	}

	mac := GetFirstString(state, "mac-address", "macAddress")
	if mac == "" {
		return MACEntry{}, false
	}

	vlanID := parseVLANID(GetFirstString(state, "vlan"))
	entryType := strings.ToLower(GetFirstString(state, "entry-type", "entryType"))

	// Extract port from interface.interface-ref.state.interface
	var port string
	if ifaceMap := GetMap(entry, "interface"); ifaceMap != nil {
		if ifRef := GetMap(ifaceMap, "interface-ref"); ifRef != nil {
			if refState := GetMap(ifRef, "state"); refState != nil {
				port = GetFirstString(refState, "interface")
			}
		}
	}

	return MACEntry{
		MAC:      normalizeMACAddress(mac),
		VLAN:     vlanID,
		Port:     NormalizeInterfaceName(port),
		Type:     entryType,
		SwitchID: switchID,
	}, true
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
