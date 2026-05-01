package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestEnrichDevicesFromSwitchData_BasicCorrelation(t *testing.T) {
	// A device discovered via LLDP with only a chassis-id (no system-name).
	// The MAC table shows traffic MACs on that port, and ARP maps one to an IP.
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "TOR-1", Type: "switch"},
			{ID: "aa:bb:cc:00:00:02", Type: "unknown", ChassisID: "aa:bb:cc:00:00:02"},
		},
		Links: []topology.Link{
			{LocalDevice: "TOR-1", LocalPort: "Eth1/3", RemoteDevice: "aa:bb:cc:00:00:02"},
		},
	}

	inputs := []HostEnrichmentInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/3", ChassisID: "aa:bb:cc:00:00:02"},
			},
			MACEntries: []MACEntry{
				{MAC: "aa:bb:cc:00:00:00", VLAN: 100, Port: "Eth1/3", SwitchID: "TOR-1"}, // host traffic MAC (chassis-2)
				{MAC: "dd:ee:ff:11:22:33", VLAN: 100, Port: "Eth1/3", SwitchID: "TOR-1"}, // VM MAC
			},
			ARPEntries: []ARPEntry{
				{IP: "10.0.2.1", MAC: "aa:bb:cc:00:00:00", SwitchID: "TOR-1"},   // host IP
				{IP: "10.0.5.99", MAC: "dd:ee:ff:11:22:33", SwitchID: "TOR-1"},  // VM IP
			},
		},
	}

	EnrichDevicesFromSwitchData(topo, inputs, HostEnrichmentConfig{})

	dev := topo.Devices[1]
	if dev.ManagementAddress != "10.0.2.1" {
		t.Errorf("ManagementAddress = %q, want '10.0.2.1'", dev.ManagementAddress)
	}
	if dev.Type != "host" {
		t.Errorf("Type = %q, want 'host'", dev.Type)
	}
	if dev.Annotations["ip_source"] != "arp_port_correlation" {
		t.Errorf("ip_source annotation = %q, want 'arp_port_correlation'", dev.Annotations["ip_source"])
	}
}

func TestEnrichDevicesFromSwitchData_VMIPNotAssigned(t *testing.T) {
	// Ensure that VM MACs on the same port do NOT get assigned to the host device.
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "TOR-1", Type: "switch"},
			{ID: "aa:bb:cc:00:00:02", Type: "unknown", ChassisID: "aa:bb:cc:00:00:02"},
		},
		Links: []topology.Link{
			{LocalDevice: "TOR-1", LocalPort: "Eth1/3", RemoteDevice: "aa:bb:cc:00:00:02"},
		},
	}

	inputs := []HostEnrichmentInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/3", ChassisID: "aa:bb:cc:00:00:02"},
			},
			MACEntries: []MACEntry{
				// Only VM MAC on port — no host MAC matches chassis within ±2
				{MAC: "dd:ee:ff:11:22:33", VLAN: 100, Port: "Eth1/3", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "10.0.5.99", MAC: "dd:ee:ff:11:22:33", SwitchID: "TOR-1"},
			},
		},
	}

	EnrichDevicesFromSwitchData(topo, inputs, HostEnrichmentConfig{})

	dev := topo.Devices[1]
	// Should NOT have been enriched — the only ARP MAC is a VM
	if dev.ManagementAddress != "" {
		t.Errorf("ManagementAddress = %q, want empty (VM IP should not be assigned to host)", dev.ManagementAddress)
	}
}

func TestEnrichDevicesFromSwitchData_ExactChassisMatch(t *testing.T) {
	// chassis-id == port MAC (offset 0, typical for some NIC types)
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "d8:94:24:3c:cb:56", Type: "unknown", ChassisID: "d8:94:24:3c:cb:56"},
		},
	}

	inputs := []HostEnrichmentInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/42", ChassisID: "d8:94:24:3c:cb:56"},
			},
			MACEntries: []MACEntry{
				{MAC: "d8:94:24:3c:cb:56", VLAN: 7, Port: "Eth1/42", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "10.218.5.42", MAC: "d8:94:24:3c:cb:56", SwitchID: "TOR-1"},
			},
		},
	}

	EnrichDevicesFromSwitchData(topo, inputs, HostEnrichmentConfig{})

	dev := topo.Devices[0]
	if dev.ManagementAddress != "10.218.5.42" {
		t.Errorf("ManagementAddress = %q, want '10.218.5.42'", dev.ManagementAddress)
	}
	if dev.Type != "host" {
		t.Errorf("Type = %q, want 'host'", dev.Type)
	}
}

func TestEnrichDevicesFromSwitchData_DoesNotOverwriteExisting(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "aa:bb:cc:00:00:02", Type: "switch", ChassisID: "aa:bb:cc:00:00:02",
				ManagementAddress: "10.0.0.99"},
		},
	}

	inputs := []HostEnrichmentInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/3", ChassisID: "aa:bb:cc:00:00:02"},
			},
			MACEntries: []MACEntry{
				{MAC: "aa:bb:cc:00:00:02", VLAN: 100, Port: "Eth1/3", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "10.0.2.1", MAC: "aa:bb:cc:00:00:02", SwitchID: "TOR-1"},
			},
		},
	}

	EnrichDevicesFromSwitchData(topo, inputs, HostEnrichmentConfig{})

	dev := topo.Devices[0]
	// Should not overwrite existing ManagementAddress
	if dev.ManagementAddress != "10.0.0.99" {
		t.Errorf("ManagementAddress was overwritten: got %q, want '10.0.0.99'", dev.ManagementAddress)
	}
	// Should not reclassify switch to host
	if dev.Type != "switch" {
		t.Errorf("Type was changed: got %q, want 'switch'", dev.Type)
	}
}

func TestEnrichDevicesFromSwitchData_LinkLocalSkipped(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "aa:bb:cc:00:00:02", Type: "unknown", ChassisID: "aa:bb:cc:00:00:02"},
		},
	}

	inputs := []HostEnrichmentInput{
		{
			SwitchID: "TOR-1",
			Neighbors: []LLDPNeighbor{
				{LocalPort: "Eth1/3", ChassisID: "aa:bb:cc:00:00:02"},
			},
			MACEntries: []MACEntry{
				{MAC: "aa:bb:cc:00:00:02", VLAN: 100, Port: "Eth1/3", SwitchID: "TOR-1"},
			},
			ARPEntries: []ARPEntry{
				{IP: "169.254.1.1", MAC: "aa:bb:cc:00:00:02", SwitchID: "TOR-1"}, // link-local only
			},
		},
	}

	EnrichDevicesFromSwitchData(topo, inputs, HostEnrichmentConfig{})

	dev := topo.Devices[0]
	if dev.ManagementAddress != "" {
		t.Errorf("link-local IP should not be assigned, got %q", dev.ManagementAddress)
	}
}

func TestPickBestIP(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		want string
	}{
		{"single", []string{"10.0.2.1"}, "10.0.2.1"},
		{"multiple picks lowest", []string{"10.0.5.3", "10.0.2.1", "10.0.8.4"}, "10.0.2.1"},
		{"empty", []string{}, ""},
		{"link-local not filtered by pickBestIP", []string{"169.254.0.1"}, "169.254.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickBestIP(tt.ips)
			if got != tt.want {
				t.Errorf("pickBestIP(%v) = %q, want %q", tt.ips, got, tt.want)
			}
		})
	}
}

func TestIsLinkLocalOrSpecial(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.2.1", false},
		{"169.254.1.1", true},
		{"127.0.0.1", true},
		{"224.0.0.1", true},
		{"fe80::1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"192.168.1.1", false},
		{"not-an-ip", true},
	}
	for _, tt := range tests {
		got := isLinkLocalOrSpecial(tt.ip)
		if got != tt.want {
			t.Errorf("isLinkLocalOrSpecial(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}
