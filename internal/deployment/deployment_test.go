package deployment

import (
	"os"
	"testing"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff"},
		{"aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff"},
		{"AA-BB-CC-DD-EE-FF", "aa:bb:cc:dd:ee:ff"},
		{"aabb.ccdd.eeff", "aa:bb:cc:dd:ee:ff"},
		{"AABBCCDDEEFF", "aa:bb:cc:dd:ee:ff"},
		{"  AA:BB:CC:DD:EE:FF  ", "aa:bb:cc:dd:ee:ff"},
	}

	for _, tt := range tests {
		got := normalizeMAC(tt.input)
		if got != tt.want {
			t.Errorf("normalizeMAC(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEnrichTopology_MACMatch(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "aa:bb:cc:01:01:01", Type: "unknown", ChassisID: "AA:BB:CC:01:01:01"},
		},
		Links: []topology.Link{
			{LocalDevice: "TOR-1", LocalPort: "Eth1/1", RemoteDevice: "aa:bb:cc:01:01:01"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:        "HCI-Node-01",
				IPv4Address: "10.0.2.1",
				NICs: []NIC{
					{MacAddress: "AA:BB:CC:01:01:01", Name: "ethernet"},
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	// Device should be reclassified and renamed
	dev := findDevice(topo, "HCI-Node-01")
	if dev == nil {
		t.Fatal("expected device 'HCI-Node-01' after enrichment, got nil")
	}
	if dev.Type != "host" {
		t.Errorf("device type = %q, want 'host'", dev.Type)
	}
	if dev.ManagementAddress != "10.0.2.1" {
		t.Errorf("management address = %q, want '10.0.2.1'", dev.ManagementAddress)
	}
	if dev.Annotations["deployment_source"] != "true" {
		t.Error("expected deployment_source annotation")
	}

	// Link should reference the new ID
	if topo.Links[0].RemoteDevice != "HCI-Node-01" {
		t.Errorf("link remote device = %q, want 'HCI-Node-01'", topo.Links[0].RemoteDevice)
	}
}

func TestEnrichTopology_BMCIPFillsManagementAddress(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "aa:bb:cc:01:01:01", Type: "unknown", ChassisID: "AA:BB:CC:01:01:01"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:         "HCI-Node-01",
				BMCIPAddress: "10.0.10.101",
				NICs: []NIC{
					{MacAddress: "AA:BB:CC:01:01:01", Name: "ethernet"},
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	dev := findDevice(topo, "HCI-Node-01")
	if dev == nil {
		t.Fatal("expected device 'HCI-Node-01' after enrichment")
	}
	// BMCIPAddress is used as fallback when IPv4Address is empty
	if dev.ManagementAddress != "10.0.10.101" {
		t.Errorf("management address = %q, want '10.0.10.101'", dev.ManagementAddress)
	}
}

func TestEnrichTopology_SynthesizeMissing(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "TOR-1", Type: "switch"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:         "HCI-Node-01",
				IPv4Address:  "10.0.2.1",
				BMCIPAddress: "10.0.10.101",
				NICs: []NIC{
					{MacAddress: "AA:BB:CC:01:01:01", Name: "ethernet"},
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	// Should synthesize the host
	host := findDevice(topo, "HCI-Node-01")
	if host == nil {
		t.Fatal("expected synthesized host 'HCI-Node-01'")
	}
	if host.Type != "host" {
		t.Errorf("synthesized host type = %q, want 'host'", host.Type)
	}
	if host.Annotations["deployment_synthesized"] != "true" {
		t.Error("expected deployment_synthesized annotation")
	}
	if host.ManagementAddress != "10.0.2.1" {
		t.Errorf("synthesized host mgmt address = %q, want '10.0.2.1'", host.ManagementAddress)
	}
}

func TestEnrichTopology_NoOverwriteExistingData(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{
				ID:                "HCI-Node-01",
				Type:              "host",
				ChassisID:         "AA:BB:CC:01:01:01",
				SystemName:        "HCI-Node-01",
				ManagementAddress: "10.0.2.99",
			},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:        "HCI-Node-01",
				IPv4Address: "10.0.2.1",
				NICs: []NIC{
					{MacAddress: "AA:BB:CC:01:01:01", Name: "ethernet"},
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	dev := findDevice(topo, "HCI-Node-01")
	if dev == nil {
		t.Fatal("device not found")
	}
	// Management address should NOT be overwritten
	if dev.ManagementAddress != "10.0.2.99" {
		t.Errorf("management address was overwritten: got %q, want '10.0.2.99'", dev.ManagementAddress)
	}
}

func TestEnrichTopology_HostnameMatch(t *testing.T) {
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "HCI-Node-01", Type: "unknown", SystemName: "HCI-Node-01"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:        "HCI-Node-01",
				IPv4Address: "10.0.2.1",
				NICs: []NIC{
					{MacAddress: "AA:BB:CC:01:01:01", Name: "ethernet"},
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	dev := findDevice(topo, "HCI-Node-01")
	if dev == nil {
		t.Fatal("device not found after hostname enrichment")
	}
	if dev.Type != "host" {
		t.Errorf("device type = %q, want 'host'", dev.Type)
	}
	if dev.Annotations["deployment_match"] != "hostname" {
		t.Errorf("match type = %q, want 'hostname'", dev.Annotations["deployment_match"])
	}
}

func TestLoad_RealFormat(t *testing.T) {
	const samplePath = "../../examples/sample-deployment.json"
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skip("sample-deployment.json not available; skipping integration test")
	}
	dd, err := Load(samplePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(dd.HostNodes) == 0 {
		t.Fatal("expected host nodes from sample deployment JSON")
	}
	// The sample has 64 physical nodes
	if len(dd.HostNodes) != 64 {
		t.Errorf("got %d host nodes, want 64", len(dd.HostNodes))
	}
	// First node should have NIC details from PhysicalNodesV2
	first := dd.HostNodes[0]
	if first.Name != "ASRR1N42R14U01" {
		t.Errorf("first host name = %q, want 'ASRR1N42R14U01'", first.Name)
	}
	if len(first.NICs) < 4 {
		t.Errorf("first host NIC count = %d, want at least 4", len(first.NICs))
	}
	if first.BMCIPAddress == "" {
		t.Error("expected BMCIPAddress to be populated")
	}
}

func findDevice(topo *topology.Topology, id string) *topology.Device {
	for i := range topo.Devices {
		if topo.Devices[i].ID == id {
			return &topo.Devices[i]
		}
	}
	return nil
}

func TestMACAddOffset(t *testing.T) {
	tests := []struct {
		mac    string
		offset int
		want   string
		wantOK bool
	}{
		{"d8:94:24:f2:cf:b2", 2, "d8:94:24:f2:cf:b4", true},
		{"d8:94:24:f2:cf:b3", 2, "d8:94:24:f2:cf:b5", true},
		{"00:00:00:00:00:fe", 2, "00:00:00:00:01:00", true}, // carry across byte boundary
		{"bad", 2, "", false},
	}
	for _, tt := range tests {
		got, ok := macAddOffset(tt.mac, tt.offset)
		if ok != tt.wantOK {
			t.Errorf("macAddOffset(%q, %d) ok = %v, want %v", tt.mac, tt.offset, ok, tt.wantOK)
		}
		if ok && got != tt.want {
			t.Errorf("macAddOffset(%q, %d) = %q, want %q", tt.mac, tt.offset, got, tt.want)
		}
	}
}

func TestEnrichTopology_MACOffsetMatch(t *testing.T) {
	// Simulate NX-OS: LLDP chassis-id = port MAC + 2
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "d894.24f2.cfb4", Type: "unknown", ChassisID: "d894.24f2.cfb4"},
		},
		Links: []topology.Link{
			{LocalDevice: "TOR-1", LocalPort: "Eth1/1", RemoteDevice: "d894.24f2.cfb4"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:        "ASRR1N42R14U01",
				IPv4Address: "10.0.2.1",
				NICs: []NIC{
					{MacAddress: "D8-94-24-F2-CF-B2", Name: "ethernet"},
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	dev := findDevice(topo, "ASRR1N42R14U01")
	if dev == nil {
		t.Fatal("expected device 'ASRR1N42R14U01' after offset match enrichment")
	}
	if dev.Type != "host" {
		t.Errorf("type = %q, want 'host'", dev.Type)
	}
	if dev.Annotations["deployment_match"] != "mac_offset_2" {
		t.Errorf("match type = %q, want 'mac_offset_2'", dev.Annotations["deployment_match"])
	}

	// Link should be rewritten
	if topo.Links[0].RemoteDevice != "ASRR1N42R14U01" {
		t.Errorf("link remote = %q, want 'ASRR1N42R14U01'", topo.Links[0].RemoteDevice)
	}
}

func TestEnrichTopology_MergeNICPorts(t *testing.T) {
	// Two NIC ports from the same host, each appearing as a separate device
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "TOR-1", Type: "switch"},
			{ID: "aa:bb:cc:00:00:02", Type: "unknown", ChassisID: "aa:bb:cc:00:00:02"}, // NIC port 1 (MAC+2)
			{ID: "aa:bb:cc:00:00:03", Type: "unknown", ChassisID: "aa:bb:cc:00:00:03"}, // NIC port 2 (MAC+2)
		},
		Links: []topology.Link{
			{LocalDevice: "TOR-1", LocalPort: "Eth1/1", RemoteDevice: "aa:bb:cc:00:00:02"},
			{LocalDevice: "TOR-1", LocalPort: "Eth1/25", RemoteDevice: "aa:bb:cc:00:00:03"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name:        "Host-01",
				IPv4Address: "10.0.2.1",
				NICs: []NIC{
					{MacAddress: "AA:BB:CC:00:00:00", Name: "ethernet"},   // +2 = 02
					{MacAddress: "AA:BB:CC:00:00:01", Name: "ethernet 2"}, // +2 = 03
				},
			},
		},
	}

	EnrichTopology(topo, dd)

	// Should have 2 devices: TOR-1 and Host-01 (merged from 2 NIC ports)
	if len(topo.Devices) != 2 {
		t.Errorf("expected 2 devices after merge, got %d", len(topo.Devices))
		for _, d := range topo.Devices {
			t.Logf("  device: ID=%s Type=%s", d.ID, d.Type)
		}
	}

	host := findDevice(topo, "Host-01")
	if host == nil {
		t.Fatal("expected merged device 'Host-01'")
	}

	// Both links should now point to the merged device
	for i, link := range topo.Links {
		if link.RemoteDevice != "Host-01" {
			t.Errorf("link[%d] remote = %q, want 'Host-01'", i, link.RemoteDevice)
		}
	}
}

func TestEnrichTopology_HostnameOnlyDoesNotMerge(t *testing.T) {
	// Two devices with same hostname match but no MAC match — should NOT merge
	topo := &topology.Topology{
		Devices: []topology.Device{
			{ID: "dev-1", Type: "unknown", SystemName: "Host-01"},
			{ID: "dev-2", Type: "unknown", SystemName: "Host-01"},
		},
	}

	dd := &DeploymentData{
		HostNodes: []HostNode{
			{
				Name: "Host-01",
				NICs: []NIC{{MacAddress: "AA:BB:CC:00:00:00", Name: "ethernet"}},
			},
		},
	}

	EnrichTopology(topo, dd)

	// Only the first device should be enriched (hostname match), not merged
	// Both should remain as separate devices since hostname match is weak
	hostCount := 0
	for _, d := range topo.Devices {
		if d.Annotations != nil && d.Annotations["deployment_name"] == "Host-01" {
			hostCount++
		}
	}
	// First match wins for hostname, second won't match since first already matched
	// But both devices remain — no merge for hostname-only matches
	if len(topo.Devices) < 2 {
		t.Errorf("hostname-only matches should not merge devices, got %d devices", len(topo.Devices))
	}
}
