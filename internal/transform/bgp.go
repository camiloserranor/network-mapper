package transform

import (
	"fmt"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// BGP gNMI paths per platform.
const (
	BGPNeighborsPathOpenConfig = "/openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors"
	BGPNeighborsPathNXOS       = "/System/bgp-items/inst-items/dom-items/Dom-list/peer-items/Peer-list"
)

// BGPNeighbor holds parsed BGP neighbor state from a gNMI response.
type BGPNeighbor struct {
	NeighborAddress        string `json:"neighbor_address"`
	PeerAS                 uint32 `json:"peer_as"`
	LocalAS                uint32 `json:"local_as,omitempty"`
	PeerType               string `json:"peer_type,omitempty"` // INTERNAL, EXTERNAL
	Description            string `json:"description,omitempty"`
	SessionState           string `json:"session_state"`          // ESTABLISHED, IDLE, ACTIVE, CONNECT, OPENSENT, OPENCONFIRM
	Enabled                bool   `json:"enabled"`
	EstablishedTransitions uint64 `json:"established_transitions,omitempty"`
	LastEstablished        string `json:"last_established,omitempty"`
	VRFName                string `json:"vrf_name,omitempty"`
	MessagesReceived       int64  `json:"messages_received,omitempty"`
	MessagesSent           int64  `json:"messages_sent,omitempty"`
	PrefixesReceived       int64  `json:"prefixes_received,omitempty"`
	PrefixesSent           int64  `json:"prefixes_sent,omitempty"`
}

// ParseBGPOpenConfig extracts BGP neighbor state from OpenConfig gNMI responses.
func ParseBGPOpenConfig(notifs []gnmi.Notification) []BGPNeighbor {
	var neighbors []BGPNeighbor

	for _, n := range notifs {
		for _, u := range n.Updates {
			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// Handle bulk response: list of neighbors
			nbrList := GetSlice(vals, "neighbor")
			if nbrList == nil {
				// Might be a single neighbor entry with "state" inside
				if GetMap(vals, "state") != nil || GetString(vals, "neighbor-address") != "" {
					nbrList = []interface{}{vals}
				}
			}
			// Wrapped in "neighbors" container
			if nbrList == nil {
				if nbrs := GetMap(vals, "neighbors"); nbrs != nil {
					nbrList = GetSlice(nbrs, "neighbor")
				}
			}

			vrfName := ExtractPathKey(u.Path, "name")
			if vrfName == "" {
				vrfName = "default"
			}

			for _, raw := range nbrList {
				nbr, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}

				state := GetMap(nbr, "state")
				if state == nil {
					state = nbr
				}

				neighborAddr := GetFirstString(state, "neighbor-address")
				if neighborAddr == "" {
					neighborAddr = ExtractPathKey(u.Path, "neighbor-address")
				}
				if neighborAddr == "" {
					continue
				}

				// Strip subnet mask if present
				if idx := strings.Index(neighborAddr, "/"); idx > 0 {
					neighborAddr = neighborAddr[:idx]
				}

				entry := BGPNeighbor{
					NeighborAddress:        neighborAddr,
					PeerAS:                 uint32(GetNumber(state, "peer-as")),
					LocalAS:                uint32(GetNumber(state, "local-as")),
					PeerType:               strings.ToUpper(GetFirstString(state, "peer-type")),
					Description:            GetFirstString(state, "description"),
					SessionState:           strings.ToUpper(GetFirstString(state, "session-state")),
					Enabled:                GetBool(state, "enabled"),
					VRFName:                vrfName,
					LastEstablished:        GetFirstString(state, "last-established"),
					EstablishedTransitions: GetNumber(state, "established-transitions"),
				}

				// Message counters (nested under "messages")
				if messages := GetMap(state, "messages"); messages != nil {
					if rcv := GetMap(messages, "received"); rcv != nil {
						entry.MessagesReceived = sumMapValues(rcv)
					}
					if snt := GetMap(messages, "sent"); snt != nil {
						entry.MessagesSent = sumMapValues(snt)
					}
				}

				// Prefix counters (nested in afi-safis)
				if afiSafis := GetMap(state, "afi-safis"); afiSafis != nil {
					if asList := GetSlice(afiSafis, "afi-safi"); asList != nil {
						for _, asRaw := range asList {
							as, ok := asRaw.(map[string]interface{})
							if !ok {
								continue
							}
							asState := GetMap(as, "state")
							if asState == nil {
								asState = as
							}
							if prefixes := GetMap(asState, "prefixes"); prefixes != nil {
								entry.PrefixesReceived += int64(GetNumber(prefixes, "received"))
								entry.PrefixesSent += int64(GetNumber(prefixes, "sent"))
							}
						}
					}
				}

				neighbors = append(neighbors, entry)
			}
		}
	}
	return neighbors
}

// ParseBGPNXOS extracts BGP neighbor state from Cisco NX-OS native YANG responses.
func ParseBGPNXOS(notifs []gnmi.Notification) []BGPNeighbor {
	var neighbors []BGPNeighbor

	for _, n := range notifs {
		for _, u := range n.Updates {
			entries := AsMapSlice(u.Value)
			if entries == nil {
				continue
			}

			vrfName := ExtractPathKey(u.Path, "name")
			if vrfName == "" {
				vrfName = "default"
			}

			for _, vals := range entries {
				peerAddr := GetString(vals, "addr")
				if peerAddr == "" {
					peerAddr = ExtractPathKey(u.Path, "addr")
				}
				if peerAddr == "" {
					continue
				}

				entry := BGPNeighbor{
					NeighborAddress: peerAddr,
					PeerAS:          uint32(GetNumber(vals, "asn")),
					Description:     GetString(vals, "name"),
					VRFName:         vrfName,
					Enabled:         true,
				}

				// Navigate to the peer entry for session state and stats
				entItems := GetMap(vals, "ent-items")
				if entItems != nil {
					peerEntryList := GetSlice(entItems, "PeerEntry-list")
					if peerEntryList == nil {
						if pe := GetMap(entItems, "PeerEntry-list"); pe != nil {
							peerEntryList = []interface{}{pe}
						}
					}
					for _, peRaw := range peerEntryList {
						pe, ok := peRaw.(map[string]interface{})
						if !ok {
							continue
						}
						entry.SessionState = mapNXOSBGPState(GetString(pe, "operSt"))
						entry.PeerAS = uint32(GetNumber(pe, "operAsn"))

						if stats := GetMap(pe, "peerstats-items"); stats != nil {
							entry.MessagesReceived = int64(GetNumber(stats, "msgRcvd"))
							entry.MessagesSent = int64(GetNumber(stats, "msgSent"))
							entry.PrefixesReceived = int64(GetNumber(stats, "pfxRcvd"))
							entry.PrefixesSent = int64(GetNumber(stats, "pfxSent"))
							entry.EstablishedTransitions = GetNumber(stats, "estabTransitions")
						}
						break
					}
				}

				if entry.SessionState == "" {
					entry.SessionState = mapNXOSBGPState(GetString(vals, "operSt"))
				}

				neighbors = append(neighbors, entry)
			}
		}
	}
	return neighbors
}

// mapNXOSBGPState normalizes NX-OS BGP state strings to standard names.
func mapNXOSBGPState(state string) string {
	switch strings.ToLower(state) {
	case "established":
		return "ESTABLISHED"
	case "idle":
		return "IDLE"
	case "active":
		return "ACTIVE"
	case "connect", "connecting":
		return "CONNECT"
	case "opensent", "open-sent":
		return "OPENSENT"
	case "openconfirm", "open-confirm":
		return "OPENCONFIRM"
	default:
		return strings.ToUpper(state)
	}
}

// sumMapValues sums all numeric values in a map (for message counters).
func sumMapValues(m map[string]interface{}) int64 {
	var total int64
	for _, v := range m {
		total += int64(toNumber(v))
	}
	return total
}

// toNumber converts interface{} to float64.
func toNumber(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	}
	return 0
}
