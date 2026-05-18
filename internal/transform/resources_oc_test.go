package transform

import (
	"math"
	"testing"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

func TestParseResourceStatsOpenConfig_CPUMultipleCores(t *testing.T) {
	// Bulk format: cpu[] array with idle values per core
	cpuNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-system:system/cpus/cpu",
					Value: map[string]interface{}{
						"cpu": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"idle": map[string]interface{}{"instant": float64(80)},
								},
							},
							map[string]interface{}{
								"state": map[string]interface{}{
									"idle": map[string]interface{}{"instant": float64(60)},
								},
							},
						},
					},
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(cpuNotifs, nil)

	// Average idle = (80+60)/2 = 70, utilization = 30
	if math.Abs(stats.CPUUtilization-30.0) > 0.1 {
		t.Errorf("CPUUtilization = %.2f, want ~30.0", stats.CPUUtilization)
	}
}

func TestParseResourceStatsOpenConfig_MemoryPhysicalReserved(t *testing.T) {
	memNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-system:system/memory/state",
					Value: map[string]interface{}{
						"state": map[string]interface{}{
							"physical": "8589934592",
							"reserved": "4294967296",
						},
					},
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(nil, memNotifs)

	if stats.MemoryTotal != 8589934592 {
		t.Errorf("MemoryTotal = %d, want 8589934592", stats.MemoryTotal)
	}
	if stats.MemoryUsed != 4294967296 {
		t.Errorf("MemoryUsed = %d, want 4294967296", stats.MemoryUsed)
	}
}

func TestParseResourceStatsOpenConfig_MemoryDirectState(t *testing.T) {
	// Memory data directly in the value without nested "state" wrapper
	memNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/openconfig-system:system/memory/state",
					Value: map[string]interface{}{
						"physical": "16000000000",
						"reserved": "8000000000",
					},
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(nil, memNotifs)

	if stats.MemoryTotal != 16000000000 {
		t.Errorf("MemoryTotal = %d, want 16000000000", stats.MemoryTotal)
	}
	if stats.MemoryUsed != 8000000000 {
		t.Errorf("MemoryUsed = %d, want 8000000000", stats.MemoryUsed)
	}
}

func TestParseResourceStatsOpenConfig_OnlyCPU(t *testing.T) {
	cpuNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/cpus/cpu",
					Value: map[string]interface{}{
						"cpu": []interface{}{
							map[string]interface{}{
								"state": map[string]interface{}{
									"idle": map[string]interface{}{"instant": float64(90)},
								},
							},
						},
					},
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(cpuNotifs, nil)

	if math.Abs(stats.CPUUtilization-10.0) > 0.1 {
		t.Errorf("CPUUtilization = %.2f, want ~10.0", stats.CPUUtilization)
	}
	if stats.MemoryTotal != 0 {
		t.Errorf("MemoryTotal = %d, want 0 (no memory data)", stats.MemoryTotal)
	}
	if stats.MemoryUsed != 0 {
		t.Errorf("MemoryUsed = %d, want 0 (no memory data)", stats.MemoryUsed)
	}
}

func TestParseResourceStatsOpenConfig_OnlyMemory(t *testing.T) {
	memNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/memory/state",
					Value: map[string]interface{}{
						"physical": "4000000000",
						"reserved": "2000000000",
					},
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(nil, memNotifs)

	if stats.CPUUtilization != 0 {
		t.Errorf("CPUUtilization = %.2f, want 0 (no CPU data)", stats.CPUUtilization)
	}
	if stats.MemoryTotal != 4000000000 {
		t.Errorf("MemoryTotal = %d, want 4000000000", stats.MemoryTotal)
	}
}

func TestParseResourceStatsOpenConfig_EmptyInput(t *testing.T) {
	stats := ParseResourceStatsOpenConfig(nil, nil)

	if stats.CPUUtilization != 0 {
		t.Errorf("CPUUtilization = %.2f, want 0", stats.CPUUtilization)
	}
	if stats.MemoryTotal != 0 {
		t.Errorf("MemoryTotal = %d, want 0", stats.MemoryTotal)
	}
	if stats.MemoryUsed != 0 {
		t.Errorf("MemoryUsed = %d, want 0", stats.MemoryUsed)
	}
}

func TestParseResourceStatsOpenConfig_SubscribeONCECPU(t *testing.T) {
	// Subscribe ONCE: each CPU core delivered as separate notification with idle at top level
	cpuNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path: "/system/cpus/cpu[index=0]/state",
					Value: map[string]interface{}{
						"idle": map[string]interface{}{"instant": float64(75)},
					},
				},
			},
		},
		{
			Updates: []gnmi.Update{
				{
					Path: "/system/cpus/cpu[index=1]/state",
					Value: map[string]interface{}{
						"idle": map[string]interface{}{"instant": float64(85)},
					},
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(cpuNotifs, nil)

	// Average idle = (75+85)/2 = 80, utilization = 20
	if math.Abs(stats.CPUUtilization-20.0) > 0.1 {
		t.Errorf("CPUUtilization = %.2f, want ~20.0", stats.CPUUtilization)
	}
}

func TestParseResourceStatsOpenConfig_CPULeafValues(t *testing.T) {
	// Leaf values delivered per path — each update is a single numeric
	cpuNotifs := []gnmi.Notification{
		{
			Updates: []gnmi.Update{
				{
					Path:  "/system/cpus/cpu[index=0]/state/idle/instant",
					Value: float64(50),
				},
			},
		},
		{
			Updates: []gnmi.Update{
				{
					Path:  "/system/cpus/cpu[index=1]/state/idle/instant",
					Value: float64(70),
				},
			},
		},
	}

	stats := ParseResourceStatsOpenConfig(cpuNotifs, nil)

	// Average idle = (50+70)/2 = 60, utilization = 40
	if math.Abs(stats.CPUUtilization-40.0) > 0.1 {
		t.Errorf("CPUUtilization = %.2f, want ~40.0", stats.CPUUtilization)
	}
}
