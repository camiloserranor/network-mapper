package transform

import (
	"encoding/json"
	"strings"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// gNMI paths for switch CPU and memory utilization.
const (
	// NX-OS native paths
	CPUPathNXOS    = "/System/procsys-items/syscpusummary-items"
	MemoryPathNXOS = "/System/procsys-items/sysmem-items"

	// OpenConfig paths (SONiC and other OC-compliant platforms)
	CPUPathOpenConfig    = "/openconfig-system:system/cpus/cpu[index=ALL]/state"
	MemoryPathOpenConfig = "/openconfig-system:system/memory/state"
)

// ResourceStats holds CPU and memory utilization for a switch.
type ResourceStats struct {
	CPUUtilization float64 // percentage 0-100
	MemoryUsed     uint64  // bytes
	MemoryTotal    uint64  // bytes
}

// ParseResourceStatsNXOS extracts CPU and memory info from gNMI responses.
func ParseResourceStatsNXOS(cpuNotifs, memNotifs []gnmi.Notification) ResourceStats {
	stats := ResourceStats{}

	// Parse CPU
	for _, notif := range cpuNotifs {
		for _, u := range notif.Updates {
			path := strings.ToLower(u.Path)

			switch {
			case strings.Contains(path, "idle"):
				idle := toFloat(u.Value)
				stats.CPUUtilization = 100.0 - idle
			case strings.Contains(path, "user") || strings.Contains(path, "kernel"):
				if stats.CPUUtilization == 0 {
					stats.CPUUtilization += toFloat(u.Value)
				}
			}
		}
	}

	// Parse memory
	for _, notif := range memNotifs {
		for _, u := range notif.Updates {
			path := strings.ToLower(u.Path)

			switch {
			case strings.Contains(path, "total"):
				stats.MemoryTotal = toUint64(u.Value)
			case strings.Contains(path, "memused") || strings.Contains(path, "used"):
				stats.MemoryUsed = toUint64(u.Value)
			}
		}
	}

	return stats
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case uint64:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}

func toUint64(v interface{}) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case int64:
		return uint64(val)
	case float64:
		return uint64(val)
	case int:
		return uint64(val)
	case json.Number:
		n, _ := val.Int64()
		return uint64(n)
	}
	return 0
}
