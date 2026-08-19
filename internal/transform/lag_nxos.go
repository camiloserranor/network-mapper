package transform

import (
	"fmt"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// LAGMembershipPathNXOS is the gNMI path for port-channel (LAG) interface data on NX-OS.
// Each AggrIf entry contains the port-channel ID and its member physical ports.
const LAGMembershipPathNXOS = "/System/intf-items/aggr-items/AggrIf-list"

// LAGMembership maps normalized port-channel names to their member physical ports.
// Key: normalized port-channel name (e.g., "po501")
// Value: list of normalized member port names (e.g., ["Eth1/49", "Eth1/50"])
type LAGMembership map[string][]string

// ParseLAGMembershipNXOS extracts port-channel membership from NX-OS gNMI responses.
//
// NX-OS path: /System/intf-items/aggr-items/AggrIf-list
// Response structure (NX-OS DME model):
//
//	value = [
//	  {
//	    "id": "po501",
//	    "pcId": "501",
//	    "adminSt": "up",
//	    "rsmbrIfs-list": [
//	      {"tDn": "sys/intf/phys-[eth1/49]"},
//	      {"tDn": "sys/intf/phys-[eth1/50]"}
//	    ]
//	  }
//	]
//
// Alternate response format — member ports may appear under "mbr-items":
//
//	{
//	  "id": "po501",
//	  "mbr-items": {
//	    "AggrMbrIf-list": [
//	      {"id": "eth1/49"},
//	      {"id": "eth1/50"}
//	    ]
//	  }
//	}
func ParseLAGMembershipNXOS(notifs []gnmi.Notification) LAGMembership {
	membership := make(LAGMembership)

	for _, n := range notifs {
		for _, u := range n.Updates {
			var items []map[string]interface{}

			// Check if Value is a map that wraps the data in containers
			if vals, ok := u.Value.(map[string]interface{}); ok {
				// Wrapped in aggr-items container
				if aggrItems := GetMap(vals, "aggr-items"); aggrItems != nil {
					if list := GetSlice(aggrItems, "AggrIf-list"); list != nil {
						items = AsMapSlice(list)
					}
				}
				// Wrapped in AggrIf-list directly
				if items == nil {
					if list := GetSlice(vals, "AggrIf-list"); list != nil {
						items = AsMapSlice(list)
					}
				}
				// Single entry with an id field (this IS an AggrIf entry)
				if items == nil && GetFirstString(vals, "id", "name") != "" {
					items = []map[string]interface{}{vals}
				}
			}

			// Value is a direct list of AggrIf entries
			if items == nil {
				if list, ok := u.Value.([]interface{}); ok {
					items = AsMapSlice(list)
				}
			}

			for _, item := range items {
				pcName, members := parseOneLAGEntry(item)
				if pcName != "" && len(members) > 0 {
					membership[pcName] = members
				}
			}
		}
	}

	return membership
}

// parseOneLAGEntry extracts port-channel name and member ports from a single AggrIf entry.
func parseOneLAGEntry(item map[string]interface{}) (string, []string) {
	pcName := NormalizePortChannelName(GetFirstString(item, "id", "name"))
	if pcName == "" {
		return "", nil
	}

	var members []string

	// Method 1: rsMbrIfs-list (relation set to member interfaces)
	// Each entry has "tDn" like "sys/intf/phys-[eth1/49]"
	if rsList := GetSlice(item, "rsMbrIfs-list"); rsList != nil {
		for _, raw := range rsList {
			entry, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			tDn := GetFirstString(entry, "tDn", "dn", "parentDn")
			if port := extractPortFromDn(tDn); port != "" {
				members = append(members, NormalizeInterfaceName(port))
			}
		}
	}

	// Method 2: mbr-items → AggrMbrIf-list
	if len(members) == 0 {
		if mbrItems := GetMap(item, "mbr-items"); mbrItems != nil {
			if mbrList := GetSlice(mbrItems, "AggrMbrIf-list"); mbrList != nil {
				for _, raw := range mbrList {
					entry, ok := raw.(map[string]interface{})
					if !ok {
						continue
					}
					port := GetFirstString(entry, "id", "name", "ifId")
					if port != "" {
						members = append(members, NormalizeInterfaceName(port))
					}
				}
			}
		}
	}

	// Method 3: rsmbrIfs (without -list suffix — single string or direct list)
	if len(members) == 0 {
		if rsMbr := GetSlice(item, "rsmbrIfs"); rsMbr != nil {
			for _, raw := range rsMbr {
				entry, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				tDn := GetFirstString(entry, "tDn", "dn")
				if port := extractPortFromDn(tDn); port != "" {
					members = append(members, NormalizeInterfaceName(port))
				}
			}
		}
	}

	// Method 4: memberPorts as comma-separated string (some NX-OS versions)
	if len(members) == 0 {
		if memberStr := GetFirstString(item, "memberPorts", "members", "activeMbrs"); memberStr != "" {
			for _, p := range strings.Split(memberStr, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					members = append(members, NormalizeInterfaceName(p))
				}
			}
		}
	}

	return pcName, members
}

// extractPortFromDn extracts an interface name from an NX-OS DME distinguished name.
// Examples:
//
//	"sys/intf/phys-[eth1/49]" → "eth1/49"
//	"sys/intf/phys-[Eth1/49]" → "Eth1/49"
//	"topology/pod-1/node-1/sys/phys-[eth1/49]" → "eth1/49"
func extractPortFromDn(dn string) string {
	// Look for pattern: ...-[portName]
	start := strings.LastIndex(dn, "[")
	end := strings.LastIndex(dn, "]")
	if start >= 0 && end > start {
		return dn[start+1 : end]
	}
	return ""
}

// NormalizePortChannelName normalizes port-channel names to the short NX-OS form.
// Examples: "port-channel501" → "po501", "Po501" → "po501", "po501" → "po501"
func NormalizePortChannelName(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "port-channel") {
		return "po" + name[len("port-channel"):]
	}
	if strings.HasPrefix(lower, "portchannel") {
		return "po" + name[len("portchannel"):]
	}
	if strings.HasPrefix(lower, "po") {
		return "po" + name[2:]
	}
	return ""
}

// IsPortChannel returns true if the port name refers to a port-channel/LAG interface.
func IsPortChannel(port string) bool {
	lower := strings.ToLower(port)
	return strings.HasPrefix(lower, "po") ||
		strings.HasPrefix(lower, "port-channel") ||
		strings.HasPrefix(lower, "portchannel")
}

// ResolveLAGPort resolves a port-channel name to its member physical ports.
// If the port is not a port-channel or not found in the membership map, returns nil.
func ResolveLAGPort(port string, membership LAGMembership) []string {
	if membership == nil || !IsPortChannel(port) {
		return nil
	}
	normalized := NormalizePortChannelName(port)
	if members, ok := membership[normalized]; ok {
		return members
	}
	return nil
}

// ParseLAGMembershipFromPhysIf derives port-channel membership from physical interface
// data by looking for pcId/channelGroup fields on each PhysIf entry. This is a fallback
// for NX-OS versions where /System/intf-items/aggr-items/AggrIf-list returns empty.
//
// NX-OS path: /System/intf-items/phys-items/PhysIf-list
// Each PhysIf entry may contain: "pcId" (uint) indicating port-channel membership.
// If pcId > 0, the interface is a member of port-channel <pcId>.
func ParseLAGMembershipFromPhysIf(notifs []gnmi.Notification) LAGMembership {
	membership := make(LAGMembership)

	for _, n := range notifs {
		for _, u := range n.Updates {
			// Parse the interface list (same formats as interface-vlans parser)
			var items []map[string]interface{}

			if vals, ok := u.Value.(map[string]interface{}); ok {
				if list := GetSlice(vals, "PhysIf-list"); list != nil {
					items = AsMapSlice(list)
				}
				if items == nil && GetFirstString(vals, "id", "name") != "" {
					items = []map[string]interface{}{vals}
				}
			}
			if items == nil {
				if list, ok := u.Value.([]interface{}); ok {
					items = AsMapSlice(list)
				}
			}

			for _, item := range items {
				ifName := GetFirstString(item, "id", "name")
				if ifName == "" {
					continue
				}

				// Look for pcId in the item or in nested phys-items
				source := item
				if physItems, ok := item["phys-items"]; ok {
					if pm, ok := physItems.(map[string]interface{}); ok {
						source = pm
					}
				}

				pcID := GetInt(source, "pcId")
				if pcID == 0 {
					pcID = GetInt(source, "channelGroup")
				}
				if pcID == 0 {
					pcID = GetInt(source, "bundleIndex")
				}
				if pcID == 0 {
					// Try string-form fields
					pcStr := GetFirstString(source, "pcId", "channelGroup", "bundleIndex")
					if pcStr != "" {
						fmt.Sscanf(pcStr, "%d", &pcID)
					}
				}

				if pcID > 0 {
					pcName := fmt.Sprintf("po%d", pcID)
					membership[pcName] = append(membership[pcName], NormalizeInterfaceName(ifName))
				}
			}
		}
	}

	return membership
}
