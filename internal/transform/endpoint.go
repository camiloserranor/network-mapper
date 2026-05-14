package transform

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

// CorrelationInput holds all the intermediate data needed to discover endpoints.
type CorrelationInput struct {
	SwitchID   string
	Neighbors  []LLDPNeighbor
	MACEntries []MACEntry
	ARPEntries []ARPEntry
}

// CorrelateEndpoints discovers VM endpoints by comparing MAC table entries
// against known LLDP neighbors. MACs on a port that don't match the LLDP
// chassis ID are classified as VMs behind that host.
func CorrelateEndpoints(inputs []CorrelationInput) []topology.Endpoint {
	// Build lookup: port → LLDP chassis IDs (normalized)
	portToChassisIDs := make(map[string]map[string]bool) // "switchID:port" → set of chassis MACs

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

	// Build lookup: port → LLDP neighbor system name (for parent host ID)
	portToHostDevice := make(map[string]string) // "switchID:port" → host device ID
	for _, input := range inputs {
		for _, nbr := range input.Neighbors {
			key := input.SwitchID + ":" + nbr.LocalPort
			hostID := nbr.SystemName
			if hostID == "" {
				hostID = nbr.ChassisID
			}
			portToHostDevice[key] = hostID
		}
	}

	// Build lookup: MAC → IPs from ARP table
	macToIPs := make(map[string][]string)
	for _, input := range inputs {
		for _, arp := range input.ARPEntries {
			mac := normalizeMACAddress(arp.MAC)
			macToIPs[mac] = appendUnique(macToIPs[mac], arp.IP)
		}
	}

	// Find endpoint MACs: MACs that don't match their port's LLDP chassis
	endpointMap := make(map[string]*topology.Endpoint) // MAC → Endpoint

	for _, input := range inputs {
		for _, macEntry := range input.MACEntries {
			if macEntry.Port == "" || macEntry.VLAN == 0 {
				continue
			}

			portKey := input.SwitchID + ":" + macEntry.Port
			chassisIDs := portToChassisIDs[portKey]

			entryMAC := normalizeMACAddress(macEntry.MAC)

			// Skip if this MAC matches the LLDP chassis (it's the physical host, not a VM).
			// Also skip if the MAC is within ±2 of a chassis ID, which covers the
			// NX-OS LLDP offset where chassis-id = port MAC + 2.
			if chassisIDs != nil && isHostMAC(entryMAC, chassisIDs) {
				continue
			}

			// Skip if MAC looks like a switch or infrastructure MAC
			if isInfraMAC(entryMAC) {
				continue
			}

			// This is likely a VM or endpoint behind the host
			if existing, ok := endpointMap[entryMAC]; ok {
				existing.VLANs = appendUniqueInt(existing.VLANs, macEntry.VLAN)
				// Upgrade host association: prefer an entry learned on a physical
				// port with an LLDP neighbor over one learned via a peer-link.
				if existing.HostDevice == "" {
					if newHost := portToHostDevice[portKey]; newHost != "" {
						existing.HostDevice = newHost
						existing.HostPort = macEntry.Port
						existing.SwitchID = input.SwitchID
					}
				}
			} else {
				ep := &topology.Endpoint{
					MAC:        entryMAC,
					VLANs:      []int{macEntry.VLAN},
					HostPort:   macEntry.Port,
					HostDevice: portToHostDevice[portKey],
					SwitchID:   input.SwitchID,
					Type:       "vm",
				}
				endpointMap[entryMAC] = ep
			}
		}
	}

	// Assign IPs from ARP table
	var endpoints []topology.Endpoint
	for _, ep := range endpointMap {
		if ips, ok := macToIPs[ep.MAC]; ok {
			ep.IPs = ips
		}
		endpoints = append(endpoints, *ep)
	}

	return endpoints
}

// isInfraMAC returns true for well-known infrastructure MAC prefixes
// (multicast, broadcast, VRRP, HSRP, STP, etc.)
func isInfraMAC(mac string) bool {
	if mac == "ff:ff:ff:ff:ff:ff" {
		return true
	}
	// Multicast MACs (first byte odd)
	if len(mac) >= 2 {
		first := mac[0:2]
		b := hexVal(first[0])*16 + hexVal(first[1])
		if b%2 == 1 {
			return true
		}
	}
	// Well-known prefixes
	for _, prefix := range []string{
		"01:00:5e", // IPv4 multicast
		"33:33:",   // IPv6 multicast
		"00:00:5e", // VRRP/IANA
		"00:00:0c", // Cisco HSRP
		"01:80:c2", // STP
	} {
		if strings.HasPrefix(mac, prefix) {
			return true
		}
	}
	return false
}

func hexVal(b byte) byte {
	if b >= '0' && b <= '9' {
		return b - '0'
	}
	if b >= 'a' && b <= 'f' {
		return b - 'a' + 10
	}
	return 0
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func appendUniqueInt(slice []int, val int) []int {
	for _, n := range slice {
		if n == val {
			return slice
		}
	}
	return append(slice, val)
}

// isHostMAC returns true if the given MAC matches any chassis ID directly,
// or is within ±2 of a chassis ID (covering the NX-OS LLDP offset).
func isHostMAC(mac string, chassisIDs map[string]bool) bool {
	if chassisIDs[mac] {
		return true
	}
	// Check if mac is within ±2 of any chassis ID
	hex := strings.ReplaceAll(mac, ":", "")
	if len(hex) != 12 {
		return false
	}
	macVal, err := strconv.ParseUint(hex, 16, 48)
	if err != nil {
		return false
	}
	for chassisMAC := range chassisIDs {
		cHex := strings.ReplaceAll(chassisMAC, ":", "")
		if len(cHex) != 12 {
			continue
		}
		cVal, err := strconv.ParseUint(cHex, 16, 48)
		if err != nil {
			continue
		}
		diff := int64(macVal) - int64(cVal)
		if diff >= -2 && diff <= 2 {
			return true
		}
	}
	return false
}

// macAddOffsetStr adds an integer offset to a normalized MAC address.
func macAddOffsetStr(mac string, offset int) string {
	hex := strings.ReplaceAll(mac, ":", "")
	if len(hex) != 12 {
		return mac
	}
	val, err := strconv.ParseUint(hex, 16, 48)
	if err != nil {
		return mac
	}
	val = uint64(int64(val) + int64(offset))
	hex = fmt.Sprintf("%012x", val)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		hex[0:2], hex[2:4], hex[4:6], hex[6:8], hex[8:10], hex[10:12])
}
