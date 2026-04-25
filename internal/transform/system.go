package transform

import (
	"fmt"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// System gNMI paths.
const (
	SystemPathOpenConfig = "/openconfig-system:system/state"
)

// SystemInfo holds parsed system-level data from a switch.
type SystemInfo struct {
	Hostname        string
	SoftwareVersion string
	Uptime          string
}

// ParseSystemOpenConfig extracts system info from OpenConfig gNMI responses.
func ParseSystemOpenConfig(notifs []gnmi.Notification) SystemInfo {
	var info SystemInfo

	for _, n := range notifs {
		for _, u := range n.Updates {
			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			if v := GetFirstString(vals, "hostname", "host-name"); v != "" {
				info.Hostname = v
			}
			if v := GetFirstString(vals, "software-version", "software_version", "os-version"); v != "" {
				info.SoftwareVersion = v
			}

			// Uptime: might be in nanoseconds or seconds
			if bootTime := GetFirstString(vals, "boot-time", "boot_time"); bootTime != "" {
				info.Uptime = "boot: " + bootTime
			}
			if uptimeStr := GetFirstString(vals, "current-datetime", "uptime"); uptimeStr != "" {
				info.Uptime = uptimeStr
			}

			// SONiC sometimes has uptime as an integer (seconds)
			if info.Uptime == "" {
				if upSec := GetNumber(vals, "uptime"); upSec > 0 {
					info.Uptime = formatUptime(upSec)
				}
			}
		}
	}

	return info
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
