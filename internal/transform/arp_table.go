package transform

import (
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// gNMI paths for ARP table.
const (
	ARPPathNXOS       = "/System/arp-items/inst-items/dom-items/Dom-list/db-items/Db-list/adj-items/AdjEp-list"
	ARPPathOpenConfig = "/openconfig-if-ip:interfaces/interface/subinterfaces/subinterface/ipv4/neighbors"
)

// ARPEntry represents a single ARP table entry mapping IP to MAC.
type ARPEntry struct {
	IP        string
	MAC       string
	Interface string
	SwitchID  string
}

// ParseARPTableNXOS extracts ARP entries from NX-OS gNMI responses.
//
// NX-OS path: /System/arp-items/inst-items/dom-items/Dom-list/db-items/Db-list/adj-items/AdjEp-list
// Response structure (verified on NX-OS 10.5):
//
// Each update contains one ARP entry as a single-element array:
//
//	path: ...Dom-list[name=VRF]/db-items/Db-list[type=ip]/adj-items/AdjEp-list[ip=X][ifId=Y]
//	value: [{"ip":"10.0.2.1","mac":"AA:BB:CC:DD:EE:FF","ifId":"vlan100","physIfId":"eth1/3",...}]
//
// The physIfId field gives the physical port, which is critical for port correlation.
func ParseARPTableNXOS(notifs []gnmi.Notification, switchID string) []ARPEntry {
	var entries []ARPEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			items := AsMapSlice(u.Value)
			if items == nil {
				continue
			}

			for _, item := range items {
				// Direct ARP entry (when path drills into AdjEp-list)
				ip := GetFirstString(item, "ip", "addr")
				mac := GetFirstString(item, "mac", "physAddr")

				if ip != "" && mac != "" {
					// Prefer physIfId (physical port) over ifId (SVI/VLAN interface)
					iface := GetFirstString(item, "physIfId", "ifId", "interface")
					iface = cleanARPInterface(iface)
					entries = append(entries, ARPEntry{
						IP:        ip,
						MAC:       normalizeMACAddress(mac),
						Interface: NormalizeInterfaceName(iface),
						SwitchID:  switchID,
					})
					continue
				}

				// Nested structure (when querying a higher-level path)
				entries = append(entries, extractARPNested(item, switchID)...)
			}
		}
	}

	return entries
}

// extractARPNested walks Dom-list → db-items → Db-list → adj-items → AdjEp-list
// when the gNMI response is from a higher-level path.
func extractARPNested(m map[string]interface{}, switchID string) []ARPEntry {
	var entries []ARPEntry

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
							iface := GetFirstString(adj, "physIfId", "ifId", "interface")
							iface = cleanARPInterface(iface)
							entries = append(entries, ARPEntry{
								IP:        aIP,
								MAC:       normalizeMACAddress(aMAC),
								Interface: NormalizeInterfaceName(iface),
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

// ParseARPTableOpenConfig extracts ARP entries from OpenConfig (SONiC/Dell) gNMI responses.
//
// OpenConfig path: /openconfig-if-ip:interfaces/interface/subinterfaces/subinterface/ipv4/neighbors
// The interface name is extracted from the path key [name=X].
//
// Bulk Get format:
//
//	value: {"neighbor": [{"state": {"ip": "10.0.2.1", "link-layer-address": "aa:bb:cc:dd:ee:ff"}}]}
//
// Subscribe ONCE format (per-entry):
//
//	value: {"state": {"ip": "10.0.2.1", "link-layer-address": "aa:bb:cc:dd:ee:ff"}}
func ParseARPTableOpenConfig(notifs []gnmi.Notification, switchID string) []ARPEntry {
	var entries []ARPEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			iface := ExtractPathKey(u.Path, "name")
			iface = NormalizeInterfaceName(iface)

			raw, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Bulk format: {"neighbor": [...]}
			if neighbors := GetSlice(raw, "neighbor"); neighbors != nil {
				for _, nbrRaw := range neighbors {
					nbr, ok := nbrRaw.(map[string]interface{})
					if !ok {
						continue
					}
					if e, ok := parseOpenConfigNeighbor(nbr, iface, switchID); ok {
						entries = append(entries, e)
					}
				}
				continue
			}

			// Subscribe ONCE format: {"state": {...}}
			if GetMap(raw, "state") != nil {
				if e, ok := parseOpenConfigNeighbor(raw, iface, switchID); ok {
					entries = append(entries, e)
				}
			}
		}
	}

	return entries
}

// parseOpenConfigNeighbor extracts IP and MAC from a single OpenConfig neighbor map.
func parseOpenConfigNeighbor(nbr map[string]interface{}, iface, switchID string) (ARPEntry, bool) {
	state := GetMap(nbr, "state")
	if state == nil {
		return ARPEntry{}, false
	}

	ip := GetFirstString(state, "ip")
	mac := GetFirstString(state, "link-layer-address")

	if ip == "" {
		return ARPEntry{}, false
	}

	return ARPEntry{
		IP:        ip,
		MAC:       normalizeMACAddress(mac),
		Interface: iface,
		SwitchID:  switchID,
	}, true
}

func cleanARPInterface(iface string) string {
	iface = strings.TrimSpace(iface)
	if iface == "" || iface == "unspecified" {
		return ""
	}
	return NormalizeInterfaceName(iface)
}
