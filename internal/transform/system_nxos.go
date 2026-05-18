package transform

import (
	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// SystemPathNXOS is the gNMI path for NX-OS system/version info.
const SystemPathNXOS = "/System/showversion-items"

// ParseSystemNXOS extracts system information from NX-OS native gNMI responses
// (path: /System/showversion-items).
func ParseSystemNXOS(notifs []gnmi.Notification) SystemInfo {
	var info SystemInfo

	for _, n := range notifs {
		for _, u := range n.Updates {
			maps := AsMapSlice(u.Value)
			if maps == nil {
				if m, ok := u.Value.(map[string]interface{}); ok {
					// May be wrapped under the path key
					if inner := GetMap(m, "System/showversion-items"); inner != nil {
						m = inner
					}
					maps = []map[string]interface{}{m}
				} else {
					continue
				}
			}

			for _, vals := range maps {
				if v := GetFirstString(vals, "hostName", "host_name", "hostname"); v != "" {
					info.Hostname = v
				}
				if v := GetFirstString(vals, "nxosVersion", "nxos_version"); v != "" {
					info.SoftwareVersion = "NX-OS " + v
				} else if info.SoftwareVersion == "" {
					if v := GetFirstString(vals, "biosVer", "biosVersion", "bios_version"); v != "" {
						info.SoftwareVersion = v
					}
				}
				if info.Uptime == "" {
					if upSec := GetNumber(vals, "kernelUptime"); upSec > 0 {
						info.Uptime = formatUptime(upSec)
					} else if upSec := GetNumber(vals, "kernel_uptime"); upSec > 0 {
						info.Uptime = formatUptime(upSec)
					} else if v := GetString(vals, "kernelUptime"); v != "" {
						info.Uptime = v
					}
				}
			}
		}
	}

	return info
}
