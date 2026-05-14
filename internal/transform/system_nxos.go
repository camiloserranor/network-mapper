package transform

import (
	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// ParseSystemNXOS extracts system information from NX-OS native gNMI responses
// (path: /System/showversion-items).
func ParseSystemNXOS(notifs []gnmi.Notification) SystemInfo {
	var info SystemInfo

	for _, n := range notifs {
		for _, u := range n.Updates {
			vals, ok := u.Value.(map[string]interface{})
			if !ok {
				continue
			}

			// May be wrapped under the path key
			if inner := GetMap(vals, "System/showversion-items"); inner != nil {
				vals = inner
			}

			if v := GetString(vals, "nxosVersion"); v != "" {
				info.SoftwareVersion = "NX-OS " + v
			}
			if v := GetString(vals, "kernelUptime"); v != "" {
				info.Uptime = v
			}
			// NX-OS doesn't expose hostname here — caller uses switch directory name
		}
	}

	return info
}
