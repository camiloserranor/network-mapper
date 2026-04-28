package transform

import (
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// gNMI paths for ARP table.
const (
	ARPPathNXOS = "/System/arp-items/inst-items/dom-items/Dom-list/db-items/Db-list/adj-items/AdjEp-list"
)

// ARPEntry represents a single ARP table entry mapping IP to MAC.
type ARPEntry struct {
	IP        string
	MAC       string
	Interface string
	SwitchID  string
}

// ParseARPTableNXOS extracts ARP entries from NX-OS gNMI responses.
func ParseARPTableNXOS(notifs []gnmi.Notification, switchID string) []ARPEntry {
	var entries []ARPEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			items := AsMapSlice(u.Value)
			if items == nil {
				continue
			}

			for _, item := range items {
				entries = append(entries, extractARPFromMap(item, switchID)...)
			}
		}
	}

	return entries
}

func extractARPFromMap(m map[string]interface{}, switchID string) []ARPEntry {
	var entries []ARPEntry

	// Direct ARP entry
	ip := GetFirstString(m, "ip", "addr", "ipAddr")
	mac := GetFirstString(m, "mac", "physAddr", "macAddr")

	if ip != "" && mac != "" {
		iface := GetFirstString(m, "ifId", "interface", "intf")
		entries = append(entries, ARPEntry{
			IP:        ip,
			MAC:       normalizeMACAddress(mac),
			Interface: NormalizeInterfaceName(iface),
			SwitchID:  switchID,
		})
		return entries
	}

	// Walk nested structures (Dom-list → db-items → Db-list → adj-items → AdjEp-list)
	for _, key := range []string{"Dom-list", "dom-items"} {
		domList := GetSlice(m, key)
		for _, domRaw := range domList {
			dom, ok := domRaw.(map[string]interface{})
			if !ok {
				continue
			}
			dbItems := GetMap(dom, "db-items")
			if dbItems == nil {
				continue
			}
			for _, dbKey := range []string{"Db-list", "db-items"} {
				dbList := GetSlice(dbItems, dbKey)
				for _, dbRaw := range dbList {
					db, ok := dbRaw.(map[string]interface{})
					if !ok {
						continue
					}
					adjItems := GetMap(db, "adj-items")
					if adjItems == nil {
						continue
					}
					adjList := GetSlice(adjItems, "AdjEp-list")
					for _, adjRaw := range adjList {
						adj, ok := adjRaw.(map[string]interface{})
						if !ok {
							continue
						}
						aIP := GetFirstString(adj, "ip", "addr")
						aMAC := GetFirstString(adj, "mac", "physAddr")
						if aIP != "" && aMAC != "" {
							iface := GetFirstString(adj, "ifId", "interface")
							// Clean up interface: sometimes has "vlan" prefix with VLAN number
							iface = cleanARPInterface(iface)
							entries = append(entries, ARPEntry{
								IP:        aIP,
								MAC:       normalizeMACAddress(aMAC),
								Interface: iface,
								SwitchID:  switchID,
							})
						}
					}
				}
			}
		}
	}

	return entries
}

func cleanARPInterface(iface string) string {
	iface = strings.TrimSpace(iface)
	if iface == "" || iface == "unspecified" {
		return ""
	}
	return NormalizeInterfaceName(iface)
}
