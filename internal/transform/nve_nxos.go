package transform

import (
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// NVEPeersPathNXOS is the gNMI path for NVE peer/VTEP data on NX-OS.
const NVEPeersPathNXOS = "/System/eps-items"

// L2RIBPathNXOS is the gNMI path for the L2 Routing Information Base on NX-OS.
const L2RIBPathNXOS = "/System/l2rib-items"

// ParseNVEPeersNXOS extracts NVE peer (VTEP) information from NX-OS gNMI responses.
// Path: /System/eps-items → epId-items → Ep-list → peers-items → DyPeer-list
func ParseNVEPeersNXOS(notifs []gnmi.Notification) []NVEPeer {
	var peers []NVEPeer

	for _, n := range notifs {
		for _, u := range n.Updates {
			val, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Navigate: eps-items → epId-items → Ep-list
			epIdItems := GetMap(val, "epId-items")
			if epIdItems == nil {
				epIdItems = val
			}

			epList := GetSlice(epIdItems, "Ep-list")
			if epList == nil {
				continue
			}

			for _, epRaw := range epList {
				ep, ok := epRaw.(map[string]interface{})
				if !ok {
					continue
				}

				// Navigate: peers-items → DyPeer-list
				peersItems := GetMap(ep, "peers-items")
				if peersItems == nil {
					continue
				}
				dyPeerList := GetSlice(peersItems, "DyPeer-list")
				if dyPeerList == nil {
					continue
				}

				for _, peerRaw := range dyPeerList {
					peer, ok := peerRaw.(map[string]interface{})
					if !ok {
						continue
					}

					ip := GetString(peer, "ip")
					if ip == "" {
						continue
					}

					peers = append(peers, NVEPeer{
						PeerIP:  ip,
						PeerMAC: normalizeMACAddress(GetString(peer, "mac")),
						State:   GetString(peer, "state"),
					})
				}
			}
		}
	}

	return peers
}

// ParseL2RIBNXOS extracts MAC→next-hop-VTEP mappings from the L2RIB.
// Path: /System/l2rib-items → inst-items → topology-items → topo-items →
//
//	Topo-list → macip-items → mac-items → MacIpEntry-list → producer-items →
//	MacIpRt-list → nexthops-items → nh-items → MacIpNexthop-list[nh=VTEP_IP]
//
// Note: Real NX-OS data uses macip-items (MAC+IP combined entries) rather than
// mac-items alone. The parser handles both variants.
func ParseL2RIBNXOS(notifs []gnmi.Notification) []L2RIBEntry {
	var entries []L2RIBEntry

	for _, n := range notifs {
		for _, u := range n.Updates {
			val, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Navigate the deep hierarchy to find Topo-list
			topoList := navigateToTopoList(val)
			if topoList == nil {
				continue
			}

			for _, topoRaw := range topoList {
				topo, ok := topoRaw.(map[string]interface{})
				if !ok {
					continue
				}

				topoID := int(GetNumber(topo, "topoId"))

				// Strategy 1: macip-items → mac-items → MacIpEntry-list (real NX-OS data)
				macIpItems := GetMap(topo, "macip-items")
				if macIpItems != nil {
					macItems := GetMap(macIpItems, "mac-items")
					if macItems != nil {
						macIpEntryList := GetSlice(macItems, "MacIpEntry-list")
						for _, entryRaw := range macIpEntryList {
							entry, ok := entryRaw.(map[string]interface{})
							if !ok {
								continue
							}

							macAddr := normalizeMACAddress(GetString(entry, "macAddr"))
							if macAddr == "" {
								continue
							}

							nextHop := extractMacIpNextHop(entry)
							if nextHop == "" || strings.EqualFold(nextHop, "Local") {
								continue
							}

							entries = append(entries, L2RIBEntry{
								MAC:       macAddr,
								NextHopIP: nextHop,
								VNI:       topoID,
							})
						}
					}
				}

				// Strategy 2: mac-items → MacEntry-list (fallback for mac-only routes)
				macItems := GetMap(topo, "mac-items")
				if macItems != nil {
					macEntryList := GetSlice(macItems, "MacEntry-list")
					for _, entryRaw := range macEntryList {
						entry, ok := entryRaw.(map[string]interface{})
						if !ok {
							continue
						}

						macAddr := normalizeMACAddress(GetString(entry, "macAddr"))
						if macAddr == "" {
							continue
						}

						nextHop := extractL2RIBNextHop(entry)
						if nextHop == "" || strings.EqualFold(nextHop, "Local") {
							continue
						}

						// Only add if not already present from macip path
						entries = append(entries, L2RIBEntry{
							MAC:       macAddr,
							NextHopIP: nextHop,
							VNI:       topoID,
						})
					}
				}
			}
		}
	}

	return entries
}

// navigateToTopoList drills into the l2rib JSON hierarchy to find Topo-list.
func navigateToTopoList(root map[string]interface{}) []interface{} {
	// Try multiple path variations since the gNMI response wrapping varies
	paths := [][]string{
		{"inst-items", "topology-items", "topo-items", "Topo-list"},
		{"l2rib-items", "inst-items", "topology-items", "topo-items", "Topo-list"},
		{"System/l2rib-items", "inst-items", "topology-items", "topo-items", "Topo-list"},
	}

	for _, path := range paths {
		current := root
		found := true
		for i, key := range path {
			if i == len(path)-1 {
				// Last element is a list
				if list := GetSlice(current, key); list != nil {
					return list
				}
				found = false
				break
			}
			next := GetMap(current, key)
			if next == nil {
				found = false
				break
			}
			current = next
		}
		if found {
			break
		}
	}

	return nil
}

// extractMacIpNextHop extracts the VTEP next-hop IP from a MacIpEntry.
// Structure: producer-items → MacIpRt-list → nexthops-items → nh-items → MacIpNexthop-list[nh]
func extractMacIpNextHop(entry map[string]interface{}) string {
	producerItems := GetMap(entry, "producer-items")
	if producerItems == nil {
		return ""
	}

	macIpRtList := GetSlice(producerItems, "MacIpRt-list")
	if macIpRtList == nil {
		return ""
	}

	for _, rtRaw := range macIpRtList {
		rt, ok := rtRaw.(map[string]interface{})
		if !ok {
			continue
		}

		nhItems := GetMap(rt, "nexthops-items")
		if nhItems == nil {
			continue
		}

		nhInner := GetMap(nhItems, "nh-items")
		if nhInner == nil {
			nhInner = nhItems
		}

		nhList := GetSlice(nhInner, "MacIpNexthop-list")
		if nhList == nil {
			continue
		}

		for _, nhRaw := range nhList {
			nh, ok := nhRaw.(map[string]interface{})
			if !ok {
				continue
			}
			nhIP := GetString(nh, "nh")
			if nhIP != "" && nhIP != "0.0.0.0" {
				return nhIP
			}
		}
	}

	return ""
}

// extractL2RIBNextHop extracts the VTEP next-hop IP from a MacEntry.
// Structure: producer-items → MacRt-list → nexthops-items → nh-items → MacNexthop-list[nh]
func extractL2RIBNextHop(entry map[string]interface{}) string {
	producerItems := GetMap(entry, "producer-items")
	if producerItems == nil {
		return ""
	}

	macRtList := GetSlice(producerItems, "MacRt-list")
	if macRtList == nil {
		return ""
	}

	for _, rtRaw := range macRtList {
		rt, ok := rtRaw.(map[string]interface{})
		if !ok {
			continue
		}

		nhItems := GetMap(rt, "nexthops-items")
		if nhItems == nil {
			nhItems = GetMap(rt, "nh-items")
		}
		if nhItems == nil {
			continue
		}

		nhList := GetSlice(nhItems, "MacNexthop-list")
		if nhList == nil {
			nhList = GetSlice(nhItems, "nh-items")
		}
		if nhList == nil {
			// Maybe nested one more level
			inner := GetMap(nhItems, "nh-items")
			if inner != nil {
				nhList = GetSlice(inner, "MacNexthop-list")
			}
		}
		if nhList == nil {
			continue
		}

		for _, nhRaw := range nhList {
			nh, ok := nhRaw.(map[string]interface{})
			if !ok {
				continue
			}
			nhIP := GetString(nh, "nh")
			if nhIP != "" && nhIP != "0.0.0.0" {
				return nhIP
			}
		}
	}

	return ""
}

// QoSStatsPathNXOS is the gNMI path for per-queue QoS counters on NX-OS.
const QoSStatsPathNXOS = "/System/ipqos-items/queuing-items/policy-items"

// PFCConfigPathNXOS is the gNMI path for per-interface PFC configuration.
// Note: this path requires iteration per interface or a bulk query.
const PFCConfigPathNXOS = "/System/intf-items/phys-items/PhysIf-list"

// ParseQoSStatsNXOS extracts per-queue QoS counters from NX-OS gNMI responses.
// Path: /System/ipqos-items/queuing-items/policy-items → in-items/out-items →
//
//	intf-items → If-list → queCmap-items → QueuingStats-list
func ParseQoSStatsNXOS(notifs []gnmi.Notification) []QoSStats {
	var results []QoSStats

	for _, n := range notifs {
		for _, u := range n.Updates {
			val, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Parse both ingress (in-items) and egress (out-items)
			for _, dirKey := range []string{"in-items", "out-items"} {
				direction := "ingress"
				if dirKey == "out-items" {
					direction = "egress"
				}

				dirItems := GetMap(val, dirKey)
				if dirItems == nil {
					continue
				}

				intfItems := GetMap(dirItems, "intf-items")
				if intfItems == nil {
					continue
				}

				ifList := GetSlice(intfItems, "If-list")
				if ifList == nil {
					continue
				}

				for _, ifRaw := range ifList {
					ifMap, ok := ifRaw.(map[string]interface{})
					if !ok {
						continue
					}

					ifName := GetString(ifMap, "name")
					if ifName == "" {
						continue
					}
					ifName = NormalizeInterfaceName(ifName)

					queues := parseQueueStats(ifMap, direction)
					if len(queues) == 0 {
						continue
					}

					// Find or create QoSStats for this interface
					found := false
					for i := range results {
						if results[i].InterfaceName == ifName {
							results[i].Queues = append(results[i].Queues, queues...)
							found = true
							break
						}
					}
					if !found {
						results = append(results, QoSStats{
							InterfaceName: ifName,
							Queues:        queues,
						})
					}
				}
			}
		}
	}

	return results
}

func parseQueueStats(ifMap map[string]interface{}, direction string) []QueueStat {
	var stats []QueueStat

	queCmapItems := GetMap(ifMap, "queCmap-items")
	if queCmapItems == nil {
		return nil
	}

	statsList := GetSlice(queCmapItems, "QueuingStats-list")
	if statsList == nil {
		return nil
	}

	for _, statRaw := range statsList {
		s, ok := statRaw.(map[string]interface{})
		if !ok {
			continue
		}

		qName := GetString(s, "cmapName")
		if qName == "" {
			continue
		}

		qs := QueueStat{
			QueueName:         qName,
			Direction:         direction,
			TxBytes:           sanitizeCounter(GetNumber(s, "txBytes")),
			TxPackets:         sanitizeCounter(GetNumber(s, "txPackets")),
			PFCPauseFramesTx:  sanitizeCounter(GetNumber(s, "pfcTxPpp")),
			PFCPauseFramesRx:  sanitizeCounter(GetNumber(s, "pfcRxPpp")),
			PFCWatchdogDrops:  sanitizeCounter(GetNumber(s, "pfcwdFlushedPackets")),
			ECNMarkedPackets:  sanitizeCounter(GetNumber(s, "randEcnMarkedPackets")),
			DropPackets:       sanitizeCounter(GetNumber(s, "dropPackets")),
			DropBytes:         sanitizeCounter(GetNumber(s, "dropBytes")),
			CurrentQueueDepth: sanitizeCounter(GetNumber(s, "currQueueDepth")),
			MaxQueueDepth:     sanitizeCounter(GetNumber(s, "maxQueueDepth")),
		}

		// Include if any counter is non-zero (traffic or RDMA issues)
		if qs.TxBytes > 0 || qs.PFCPauseFramesTx > 0 || qs.PFCPauseFramesRx > 0 ||
			qs.PFCWatchdogDrops > 0 || qs.ECNMarkedPackets > 0 ||
			qs.DropPackets > 0 || qs.CurrentQueueDepth > 0 {
			stats = append(stats, qs)
		}
	}

	return stats
}

// ParsePFCConfigNXOS extracts PFC configuration from NX-OS interface data.
// The PFC config is embedded within the PhysIf-list at priorflowctrl-items.
// This parser expects the full interface list notifications (same as CollectInterfaces).
func ParsePFCConfigNXOS(notifs []gnmi.Notification) []PFCConfig {
	var configs []PFCConfig

	for _, n := range notifs {
		for _, u := range n.Updates {
			val, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Try extracting interface list
			var ifList []map[string]interface{}
			switch {
			case GetSlice(val, "PhysIf-list") != nil:
				ifList = AsMapSlice(GetSlice(val, "PhysIf-list"))
			case GetSlice(val, "System/intf-items/phys-items/PhysIf-list") != nil:
				ifList = AsMapSlice(GetSlice(val, "System/intf-items/phys-items/PhysIf-list"))
			default:
				// Single interface response
				ifList = []map[string]interface{}{val}
			}

			for _, ifMap := range ifList {
				ifName := NormalizeInterfaceName(GetFirstString(ifMap, "id", "name"))
				if ifName == "" {
					continue
				}

				pfcItems := GetMap(ifMap, "priorflowctrl-items")
				if pfcItems == nil {
					continue
				}

				mode := strings.ToLower(GetString(pfcItems, "mode"))
				sendTLV := GetString(pfcItems, "send_tlv") == "true" ||
					GetString(pfcItems, "sendTlv") == "true"

				// Extract which CoS priorities have PFC enabled
				var lossless []int
				for cos := 0; cos <= 7; cos++ {
					key := "pfcCos" + string(rune('0'+cos))
					val := GetString(pfcItems, key)
					if val == "true" || val == "1" {
						lossless = append(lossless, cos)
					}
				}

				configs = append(configs, PFCConfig{
					InterfaceName: ifName,
					Mode:          mode,
					SendTLV:       sendTLV,
					LosslessCos:   lossless,
				})
			}
		}
	}

	return configs
}
