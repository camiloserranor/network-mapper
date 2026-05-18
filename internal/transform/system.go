package transform

import (
	"fmt"
	"strconv"
	"time"

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
	// Detect SONiC flat-leaf format
	if isFlatLeafFormat(notifs) {
		return parseSystemFlatLeaf(notifs)
	}

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
				info.Uptime = uptimeFromBootTime(bootTime)
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

// parseSystemFlatLeaf handles the SONiC Subscribe ONCE flat-leaf format for system info.
func parseSystemFlatLeaf(notifs []gnmi.Notification) SystemInfo {
	var info SystemInfo

	for _, n := range notifs {
		leafMap := buildLeafMap(n.Updates)

		if v := getLeafString(leafMap, "/hostname"); v != "" {
			info.Hostname = v
		}
		if v := getLeafString(leafMap, "/software-version"); v != "" {
			info.SoftwareVersion = v
		}
		if v := getLeafString(leafMap, "/boot-time"); v != "" {
			info.Uptime = uptimeFromBootTime(v)
		}
		if v := getLeafString(leafMap, "/current-datetime"); v != "" {
			info.Uptime = v
		}
	}

	return info
}

// uptimeFromBootTime converts a boot-time value (nanoseconds since epoch) into
// a human-readable uptime string. If parsing fails, returns the raw value prefixed.
func uptimeFromBootTime(bootTimeStr string) string {
	bootNano, err := strconv.ParseInt(bootTimeStr, 10, 64)
	if err != nil {
		return "boot: " + bootTimeStr
	}
	bootTime := time.Unix(0, bootNano)
	uptime := time.Since(bootTime)
	if uptime < 0 {
		return "boot: " + bootTimeStr
	}
	return formatUptime(uint64(uptime.Seconds()))
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
