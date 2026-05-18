package platform

import (
	"testing"
)

func TestForPlatform(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantEnc  string
	}{
		{"nxos", "nxos", "JSON"},
		{"sonic", "sonic", "JSON_IETF"},
		{"dell_os10", "openconfig", "JSON_IETF"},
		{"", "openconfig", "JSON_IETF"},
		{"unknown", "openconfig", "JSON_IETF"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := ForPlatform(tt.input)
			if p.Name() != tt.wantName {
				t.Errorf("ForPlatform(%q).Name() = %q, want %q", tt.input, p.Name(), tt.wantName)
			}
			if p.Encoding() != tt.wantEnc {
				t.Errorf("ForPlatform(%q).Encoding() = %q, want %q", tt.input, p.Encoding(), tt.wantEnc)
			}
		})
	}
}

func TestEnrichInterfacesFromVLANs(t *testing.T) {
	// NX-OS has its own per-port VLAN path, so no enrichment needed
	nxos := ForPlatform("nxos")
	if nxos.EnrichInterfacesFromVLANs() {
		t.Error("NXOSPlatform should not enrich interfaces from VLANs")
	}

	// SONiC and OpenConfig need enrichment from VLAN member lists
	sonic := ForPlatform("sonic")
	if !sonic.EnrichInterfacesFromVLANs() {
		t.Error("SONiCPlatform should enrich interfaces from VLANs")
	}

	oc := ForPlatform("other")
	if !oc.EnrichInterfacesFromVLANs() {
		t.Error("OpenConfigPlatform should enrich interfaces from VLANs")
	}
}

// Compile-time interface conformance checks.
var (
	_ Platform = (*NXOSPlatform)(nil)
	_ Platform = (*SONiCPlatform)(nil)
	_ Platform = (*OpenConfigPlatform)(nil)
)
