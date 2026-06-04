package transform

// NVEPeer represents a remote VTEP peer discovered via the NVE subsystem.
// On NX-OS this comes from /System/eps-items (DyPeer-list).
type NVEPeer struct {
	PeerIP  string `json:"peer_ip"`  // VTEP tunnel IP (e.g., "100.71.93.158")
	PeerMAC string `json:"peer_mac"` // VTEP MAC address (may differ from LLDP chassis-id)
	State   string `json:"state"`    // "Up", "Down", etc.
}

// L2RIBEntry represents a MAC route from the L2 Routing Information Base,
// mapping a learned MAC address to its next-hop VTEP peer IP. This enables
// VM-to-VTEP correlation in VXLAN/EVPN environments where the MAC table
// only shows "Nve" as the learning port.
type L2RIBEntry struct {
	MAC       string `json:"mac"`         // endpoint MAC address (normalized)
	NextHopIP string `json:"next_hop_ip"` // VTEP peer IP that owns this MAC
	VNI       int    `json:"vni"`         // VNI / topology ID
}

// QoSStats holds per-queue QoS counters for a single interface.
// On NX-OS this comes from /System/ipqos-items/queuing-items.
type QoSStats struct {
	InterfaceName string      `json:"interface_name"`
	Queues        []QueueStat `json:"queues,omitempty"`
}

// QueueStat holds counters for a single queue/class-map on an interface.
// These are critical for RDMA health monitoring (PFC storms, ECN marking,
// queue drops on the lossless priority).
type QueueStat struct {
	QueueName         string `json:"queue_name"`                    // class-map name (e.g., "c-out-8q-q3")
	Direction         string `json:"direction"`                     // "ingress" or "egress"
	TxBytes           uint64 `json:"tx_bytes,omitempty"`
	TxPackets         uint64 `json:"tx_packets,omitempty"`
	PFCPauseFramesTx  uint64 `json:"pfc_pause_frames_tx,omitempty"`
	PFCPauseFramesRx  uint64 `json:"pfc_pause_frames_rx,omitempty"`
	PFCWatchdogDrops  uint64 `json:"pfc_watchdog_drops,omitempty"`
	ECNMarkedPackets  uint64 `json:"ecn_marked_packets,omitempty"`
	DropPackets       uint64 `json:"drop_packets,omitempty"`
	DropBytes         uint64 `json:"drop_bytes,omitempty"`
	CurrentQueueDepth uint64 `json:"current_queue_depth_bytes,omitempty"`
	MaxQueueDepth     uint64 `json:"max_queue_depth_bytes,omitempty"`
}

// PFCConfig holds Priority Flow Control configuration for a single interface.
// This validates whether RDMA lossless requirements are met.
type PFCConfig struct {
	InterfaceName string `json:"interface_name"`
	Mode          string `json:"mode"`                    // "on", "off", "auto"
	SendTLV       bool   `json:"send_tlv"`                // DCBX PFC TLV advertisement
	LosslessCos   []int  `json:"lossless_cos,omitempty"`  // CoS priorities with PFC enabled
}

// IsNVEInterface returns true if the given port name refers to an NVE
// (Network Virtualization Edge) tunnel interface. Handles common variants:
// "Nve", "nve1", "Nve1", "NVE1", "nve 1", etc.
func IsNVEInterface(port string) bool {
	if port == "" {
		return false
	}
	lower := normalizeLower(port)
	return lower == "nve" || hasPrefix(lower, "nve")
}

func normalizeLower(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b = append(b, c)
	}
	return string(b)
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
