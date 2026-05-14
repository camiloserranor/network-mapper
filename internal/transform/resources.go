package transform

import (
	"encoding/json"
	"strconv"
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

// ParseResourceStatsOpenConfig extracts CPU and memory info from OpenConfig gNMI responses.
func ParseResourceStatsOpenConfig(cpuNotifs, memNotifs []gnmi.Notification) ResourceStats {
	stats := ResourceStats{}

	// Parse CPU — collect idle values across notifications
	var idleValues []float64
	var userSum, kernelSum float64
	var coreCount int

	for _, notif := range cpuNotifs {
		for _, u := range notif.Updates {
			var data map[string]interface{}
			raw, err := json.Marshal(u.Value)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(raw, &data); err != nil {
				// Leaf value — check path for the field name
				path := strings.ToLower(u.Path)
				if strings.Contains(path, "idle") && strings.Contains(path, "instant") {
					idleValues = append(idleValues, toFloat(u.Value))
				} else if strings.Contains(path, "user") && strings.Contains(path, "instant") {
					userSum += toFloat(u.Value)
					coreCount++
				} else if strings.Contains(path, "kernel") && strings.Contains(path, "instant") {
					kernelSum += toFloat(u.Value)
				}
				continue
			}

			// Try bulk format: look for cpu[] array
			cpuList := GetSlice(data, "cpu")
			if cpuList != nil {
				for _, entry := range cpuList {
					cpuEntry, ok := entry.(map[string]interface{})
					if !ok {
						continue
					}
					state := GetMap(cpuEntry, "state")
					if state == nil {
						state = cpuEntry
					}
					idle := extractInstant(state, "idle")
					if idle > 0 {
						idleValues = append(idleValues, idle)
					}
				}
				continue
			}

			// Subscribe ONCE format: data contains user/kernel/idle directly
			idle := extractInstant(data, "idle")
			if idle > 0 {
				idleValues = append(idleValues, idle)
			} else {
				user := extractInstant(data, "user")
				kernel := extractInstant(data, "kernel")
				if user > 0 || kernel > 0 {
					userSum += user
					kernelSum += kernel
					coreCount++
				}
			}
		}
	}

	if len(idleValues) > 0 {
		var totalIdle float64
		for _, v := range idleValues {
			totalIdle += v
		}
		avgIdle := totalIdle / float64(len(idleValues))
		stats.CPUUtilization = 100.0 - avgIdle
	} else if coreCount > 0 {
		stats.CPUUtilization = (userSum + kernelSum) / float64(coreCount)
	}

	// Parse memory
	for _, notif := range memNotifs {
		for _, u := range notif.Updates {
			var data map[string]interface{}
			raw, err := json.Marshal(u.Value)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(raw, &data); err != nil {
				continue
			}

			// Try wrapped format: state.physical / state.reserved
			state := GetMap(data, "state")
			if state == nil {
				state = data
			}

			if phys := GetString(state, "physical"); phys != "" {
				stats.MemoryTotal, _ = strconv.ParseUint(phys, 10, 64)
			}
			if reserved := GetString(state, "reserved"); reserved != "" {
				stats.MemoryUsed, _ = strconv.ParseUint(reserved, 10, 64)
			}
		}
	}

	return stats
}

// extractInstant extracts the instant value from a CPU metric field.
// Handles both {"instant": 5.2} map format and direct numeric values.
func extractInstant(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	if sub, ok := v.(map[string]interface{}); ok {
		return toFloat(sub["instant"])
	}
	return toFloat(v)
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
