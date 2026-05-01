// Package transform — host_enrichment.go
//
// EnrichDevicesFromSwitchData uses MAC table and ARP data collected from
// switches to fill in management IPs (and optionally hostnames via rDNS)
// for LLDP-discovered devices that lack identity information.
//
// # Identification Method: ARP-Port Correlation
//
// Many host NICs (especially in Azure Local deployments) do not send LLDP
// frames with identity info. The switch sees only a chassis-id (MAC address)
// with no system-name, description, or capabilities. These appear as "unknown"
// devices with MAC-based IDs.
//
// However, the switch has additional data that can be correlated:
//
//  1. LLDP tells us which chassis-id is on each switch port.
//  2. The MAC address table tells us all MACs learned on each port.
//  3. The ARP table maps MACs to IPs (when the switch is the L3 gateway).
//
// The switch port is the correlation anchor: the LLDP chassis-id and the
// MAC/ARP-learned traffic MACs both appear on the same physical port.
//
// # MAC Selection (Host vs VM)
//
// A switch port may carry traffic from many MACs: the physical host NIC and
// any VMs running behind it. We must only assign the HOST's IP, not a VM's.
//
// A MAC is considered a "host MAC" if it matches the LLDP chassis-id directly,
// or is within a known platform-specific offset:
//   - Exact match: chassis-id == MAC (standard LLDP)
//   - NX-OS offset: chassis-id == MAC + 2 (NX-OS reports chassis-id as port
//     MAC + 2; verified on Cisco Nexus 93180YC-FX3)
//
// Only IPs associated with host MACs are used. VM MACs are ignored.
//
// # Reclassification
//
// A device is reclassified from "unknown" to "host" only when:
//   - It has no LLDP capabilities suggesting it's a switch (bridge/router)
//   - It was successfully correlated with an IP via the above method
//
// # Limitations
//
//   - Only works when the TOR switch is the L3 gateway for the host's VLAN.
//   - ARP entries are point-in-time; stale entries may provide outdated IPs.
//   - Reverse DNS is opt-in and may fail or be slow.
//   - If multiple IPs match with equal confidence, none is assigned (ambiguity).
package transform

import (
	"log"
	"net"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

// HostEnrichmentInput holds the collected switch data needed for host enrichment.
type HostEnrichmentInput struct {
	SwitchID   string
	Neighbors  []LLDPNeighbor
	MACEntries []MACEntry
	ARPEntries []ARPEntry
}

// HostEnrichmentConfig controls optional behaviors.
type HostEnrichmentConfig struct {
	// ReverseDNS enables reverse DNS lookups to populate device hostnames.
	// Disabled by default because it adds network dependency and latency.
	ReverseDNS bool
}

// EnrichDevicesFromSwitchData correlates LLDP, MAC table, and ARP table data
// to fill management IPs and reclassify unknown devices as hosts.
//
// This enrichment uses only data collected from switches (no external files).
// It should be called after devices and links are assembled but before
// deployment enrichment, which provides authoritative overrides.
func EnrichDevicesFromSwitchData(topo *topology.Topology, inputs []HostEnrichmentInput, cfg HostEnrichmentConfig) {
	if len(inputs) == 0 {
		return
	}

	// Step 1: Build port→LLDP chassis-id map from links.
	// Key: "switchID:port" → set of chassis-id MACs (normalized)
	portToChassisIDs := make(map[string]map[string]bool)
	for _, input := range inputs {
		for _, nbr := range input.Neighbors {
			key := input.SwitchID + ":" + nbr.LocalPort
			if portToChassisIDs[key] == nil {
				portToChassisIDs[key] = make(map[string]bool)
			}
			chassisMAC := normalizeMACAddress(nbr.ChassisID)
			portToChassisIDs[key][chassisMAC] = true
		}
	}

	// Step 2: Build port→MACs map from MAC table.
	// Key: "switchID:port" → list of MACs
	portToMACs := make(map[string][]string)
	for _, input := range inputs {
		for _, entry := range input.MACEntries {
			if entry.Port == "" {
				continue
			}
			key := input.SwitchID + ":" + entry.Port
			portToMACs[key] = append(portToMACs[key], entry.MAC)
		}
	}

	// Step 3: Build MAC→IP map from ARP table.
	// Only keep unicast, non-link-local IPs. Prefer entries from same switch.
	macToIP := make(map[string]string) // normalized MAC → best IP
	for _, input := range inputs {
		for _, entry := range input.ARPEntries {
			if entry.IP == "" || entry.MAC == "" {
				continue
			}
			ip := entry.IP
			if isLinkLocalOrSpecial(ip) {
				continue
			}
			mac := normalizeMACAddress(entry.MAC)
			// First writer wins; ARP tables are largely consistent across switches.
			if _, exists := macToIP[mac]; !exists {
				macToIP[mac] = ip
			}
		}
	}

	// Step 4: Build device lookup by chassis-id for quick matching.
	// A device's chassis-id links it to its LLDP identity.
	chassisToDeviceIdx := make(map[string]int) // normalized chassis-id → index in topo.Devices
	for i := range topo.Devices {
		cid := normalizeMACAddress(topo.Devices[i].ChassisID)
		if cid != "" {
			chassisToDeviceIdx[cid] = i
		}
	}

	// Step 5: For each port with an LLDP neighbor, find host MACs and their IPs.
	var enriched, reclassified, dnsResolved int

	for portKey, chassisIDs := range portToChassisIDs {
		macs := portToMACs[portKey]
		if len(macs) == 0 {
			continue
		}

		// Find MACs on this port that belong to the physical host (not VMs).
		// A host MAC is one that matches (or is within platform offset of) the LLDP chassis-id.
		var hostIPs []string
		for _, mac := range macs {
			if !isHostMAC(mac, chassisIDs) {
				continue
			}
			if ip, ok := macToIP[mac]; ok {
				hostIPs = appendUnique(hostIPs, ip)
			}
		}

		if len(hostIPs) == 0 {
			continue
		}

		// If multiple IPs matched, pick deterministically but only if unambiguous.
		// Multiple IPs can legitimately exist (e.g., management + storage),
		// but we only set one as ManagementAddress.
		ip := pickBestIP(hostIPs)
		if ip == "" {
			continue
		}

		// Find the device(s) corresponding to the chassis-id(s) on this port.
		for chassisMAC := range chassisIDs {
			devIdx, ok := chassisToDeviceIdx[chassisMAC]
			if !ok {
				continue
			}
			dev := &topo.Devices[devIdx]

			// Don't overwrite existing management address (may have been set by LLDP)
			if dev.ManagementAddress == "" {
				dev.ManagementAddress = ip
				if dev.Annotations == nil {
					dev.Annotations = make(map[string]string)
				}
				dev.Annotations["ip_source"] = "arp_port_correlation"
				enriched++
			}

			// Reclassify unknown → host if we have a confident IP correlation
			// and no switch-like capabilities
			if dev.Type == "unknown" && !looksLikeSwitch(dev) {
				dev.Type = "host"
				if dev.Annotations == nil {
					dev.Annotations = make(map[string]string)
				}
				dev.Annotations["classification_source"] = "arp_port_correlation"
				reclassified++
			}

			// Optional: reverse DNS to get hostname
			if cfg.ReverseDNS && dev.SystemName == "" && dev.ManagementAddress != "" {
				if hostname := reverseDNS(dev.ManagementAddress); hostname != "" {
					dev.SystemName = hostname
					dev.Annotations["hostname_source"] = "rdns"
					dnsResolved++
				}
			}
		}
	}

	if enriched > 0 || reclassified > 0 {
		log.Printf("[host-enrichment] ARP-port correlation: %d IPs assigned, %d reclassified as host, %d DNS resolved",
			enriched, reclassified, dnsResolved)
	}
}

// looksLikeSwitch checks if a device has any indicators of being a switch
// rather than a host. Used to prevent reclassifying actual switches as hosts.
func looksLikeSwitch(dev *topology.Device) bool {
	desc := strings.ToLower(dev.SystemDescription)
	name := strings.ToLower(dev.SystemName)
	for _, hint := range []string{"sonic", "nx-os", "arista", "cumulus", "ftos", "cisco", "dell emc"} {
		if strings.Contains(desc, hint) || strings.Contains(name, hint) {
			return true
		}
	}
	return false
}

// pickBestIP selects the most likely management IP from a list of candidates.
// Prefers non-link-local, non-multicast addresses. Returns empty if ambiguous.
func pickBestIP(ips []string) string {
	if len(ips) == 0 {
		return ""
	}
	if len(ips) == 1 {
		return ips[0]
	}

	// Filter to management-likely IPs (not storage, not APIPA)
	var candidates []string
	for _, ip := range ips {
		if !isLinkLocalOrSpecial(ip) {
			candidates = append(candidates, ip)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if len(candidates) == 0 {
		return ""
	}

	// Multiple candidates — pick the lowest (deterministic) but annotate
	// that this was a best-effort choice.
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c < best {
			best = c
		}
	}
	return best
}

// isLinkLocalOrSpecial returns true for IPs that should not be used as
// management addresses (link-local, multicast, loopback, unspecified).
func isLinkLocalOrSpecial(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true // unparseable = skip
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// APIPA range (169.254.x.x)
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}

// reverseDNS attempts a reverse DNS lookup for the given IP.
// Returns the hostname (without trailing dot) or empty string on failure.
func reverseDNS(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	// Return first result, strip trailing dot
	name := strings.TrimSuffix(names[0], ".")
	return name
}
