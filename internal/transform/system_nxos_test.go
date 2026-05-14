package transform

import (
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseSystemNXOS_FullResponse(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/showversion-items",
					Value: map[string]interface{}{
						"hostName":      "TOR-SWITCH-01",
						"nxosVersion":   "10.5(1)",
						"kernelUptime":  float64(864000), // 10 days
					},
				},
			},
		},
	}

	info := ParseSystemNXOS(notifs)

	if info.Hostname != "TOR-SWITCH-01" {
		t.Errorf("Hostname = %q, want %q", info.Hostname, "TOR-SWITCH-01")
	}
	if info.SoftwareVersion != "10.5(1)" {
		t.Errorf("SoftwareVersion = %q, want %q", info.SoftwareVersion, "10.5(1)")
	}
	if info.Uptime != "10d 0h 0m" {
		t.Errorf("Uptime = %q, want %q", info.Uptime, "10d 0h 0m")
	}
}

func TestParseSystemNXOS_AlternateFieldNames(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/showversion-items",
					Value: map[string]interface{}{
						"host_name":     "alt-switch",
						"bios_version":  "3.7.0",
						"kernel_uptime": float64(7200), // 2 hours
					},
				},
			},
		},
	}

	info := ParseSystemNXOS(notifs)

	if info.Hostname != "alt-switch" {
		t.Errorf("Hostname = %q, want %q", info.Hostname, "alt-switch")
	}
	if info.SoftwareVersion != "3.7.0" {
		t.Errorf("SoftwareVersion = %q, want %q", info.SoftwareVersion, "3.7.0")
	}
	if info.Uptime != "2h 0m" {
		t.Errorf("Uptime = %q, want %q", info.Uptime, "2h 0m")
	}
}

func TestParseSystemNXOS_PartialData(t *testing.T) {
	// Only hostname, no version or uptime
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/showversion-items",
					Value: map[string]interface{}{
						"hostname": "partial-switch",
					},
				},
			},
		},
	}

	info := ParseSystemNXOS(notifs)

	if info.Hostname != "partial-switch" {
		t.Errorf("Hostname = %q, want %q", info.Hostname, "partial-switch")
	}
	if info.SoftwareVersion != "" {
		t.Errorf("SoftwareVersion = %q, want empty", info.SoftwareVersion)
	}
	if info.Uptime != "" {
		t.Errorf("Uptime = %q, want empty", info.Uptime)
	}
}

func TestParseSystemNXOS_EmptyInput(t *testing.T) {
	info := ParseSystemNXOS(nil)

	if info.Hostname != "" {
		t.Errorf("Hostname = %q, want empty", info.Hostname)
	}
	if info.SoftwareVersion != "" {
		t.Errorf("SoftwareVersion = %q, want empty", info.SoftwareVersion)
	}
	if info.Uptime != "" {
		t.Errorf("Uptime = %q, want empty", info.Uptime)
	}
}

func TestParseSystemNXOS_ArrayValue(t *testing.T) {
	// AsMapSlice handles []interface{} too
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/showversion-items",
					Value: []interface{}{
						map[string]interface{}{
							"hostName":    "array-switch",
							"nxosVersion": "9.3(8)",
						},
					},
				},
			},
		},
	}

	info := ParseSystemNXOS(notifs)

	if info.Hostname != "array-switch" {
		t.Errorf("Hostname = %q, want %q", info.Hostname, "array-switch")
	}
	if info.SoftwareVersion != "9.3(8)" {
		t.Errorf("SoftwareVersion = %q, want %q", info.SoftwareVersion, "9.3(8)")
	}
}

func TestParseSystemNXOS_BIOSFallback(t *testing.T) {
	// When nxosVersion is missing, biosVer is used as fallback
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/showversion-items",
					Value: map[string]interface{}{
						"hostname": "bios-switch",
						"biosVer":  "5.2.1",
					},
				},
			},
		},
	}

	info := ParseSystemNXOS(notifs)

	if info.SoftwareVersion != "5.2.1" {
		t.Errorf("SoftwareVersion = %q, want %q (BIOS fallback)", info.SoftwareVersion, "5.2.1")
	}
}

func TestParseSystemNXOS_UptimeMinutesOnly(t *testing.T) {
	notifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/System/showversion-items",
					Value: map[string]interface{}{
						"hostname":     "short-uptime",
						"kernelUptime": float64(300), // 5 minutes
					},
				},
			},
		},
	}

	info := ParseSystemNXOS(notifs)

	if info.Uptime != "5m" {
		t.Errorf("Uptime = %q, want %q", info.Uptime, "5m")
	}
}
