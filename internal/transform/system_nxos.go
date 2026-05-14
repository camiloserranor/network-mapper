package transform

import (
	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// SystemPathNXOS is the gNMI path for NX-OS system/version info.
const SystemPathNXOS = "/System/showversion-items"

// ParseSystemNXOS extracts system info from NX-OS gNMI responses.
func ParseSystemNXOS(notifs []gnmi.Notification) SystemInfo {
	var info SystemInfo

	for _, n := range notifs {
		for _, u := range n.Updates {
			maps := AsMapSlice(u.Value)
			if maps == nil {
				if m, ok := u.Value.(map[string]interface{}); ok {
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
					info.SoftwareVersion = v
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
					}
				}
			}
		}
	}

	return info
}
