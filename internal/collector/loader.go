// Package collector — loader.go provides offline data loading from raw gNMI
// dump files, enabling the full pipeline to run without live switch access.
package collector

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/topology"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// rawDumpFile is the on-disk format produced by gnmi-dump. Each file contains
// the raw gNMI notifications for one data category from one switch.
type rawDumpFile struct {
	Switch            string              `json:"switch"`
	Category          string              `json:"category"`
	Platform          string              `json:"platform"`
	Path              string              `json:"path"`
	Timestamp         string              `json:"timestamp"`
	NotificationCount int                 `json:"notification_count"`
	Notifications     []gnmi.Notification `json:"notifications"`
}

// LoadFromDisk reads a directory of raw gNMI dump files (as produced by the
// gnmi-dump tool) and runs them through the same transform/parse pipeline
// used during live collection. Each subdirectory is treated as a switch.
//
// Directory structure:
//
//	<dir>/
//	  ├── TOR-1/
//	  │   ├── lldp-nxos.json
//	  │   ├── interfaces-openconfig.json
//	  │   ├── system-openconfig.json
//	  │   └── ...
//	  ├── TOR-2/
//	  │   └── ...
//	  └── _summary.json   (optional, ignored)
func LoadFromDisk(dir string) (*CollectionResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading raw data directory: %w", err)
	}

	// Collect switch subdirectories (skip files like _summary.json)
	var switchDirs []string
	for _, e := range entries {
		if e.IsDir() {
			switchDirs = append(switchDirs, e.Name())
		}
	}
	sort.Strings(switchDirs)

	if len(switchDirs) == 0 {
		return nil, fmt.Errorf("no switch directories found in %s", dir)
	}

	cr := &CollectionResult{
		CollectedAt: time.Now().UTC(),
		Switches:    make([]SwitchData, 0, len(switchDirs)),
	}

	// Try to extract timestamp from _summary.json
	summaryPath := filepath.Join(dir, "_summary.json")
	if data, err := os.ReadFile(summaryPath); err == nil {
		var summary struct {
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal(data, &summary) == nil && summary.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, summary.Timestamp); err == nil {
				cr.CollectedAt = t
			}
		}
	}

	for _, swName := range switchDirs {
		swDir := filepath.Join(dir, swName)
		sd, err := loadSwitchFromDisk(swDir, swName)
		if err != nil {
			log.Printf("WARNING: loading %s from disk: %v", swName, err)
			sd = SwitchData{
				SwitchName: swName,
				SwitchID:   swName,
				Errors: []topology.PartialError{
					{Switch: swName, Phase: "load", Message: err.Error()},
				},
			}
		}
		cr.Switches = append(cr.Switches, sd)
	}

	return cr, nil
}

func loadSwitchFromDisk(dir, swName string) (SwitchData, error) {
	sd := SwitchData{
		SwitchName: swName,
		SwitchID:   swName,
	}

	// Load all available category files
	files, err := os.ReadDir(dir)
	if err != nil {
		return sd, fmt.Errorf("reading switch directory: %w", err)
	}

	// Group files by category
	categories := make(map[string]*rawDumpFile)
	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		raw, err := loadDumpFile(filepath.Join(dir, f.Name()))
		if err != nil {
			log.Printf("  %s: skipping %s: %v", swName, f.Name(), err)
			sd.Errors = append(sd.Errors, topology.PartialError{
				Switch: swName, Phase: "load-" + f.Name(), Message: err.Error(),
			})
			continue
		}
		categories[raw.Category] = raw
	}

	platform := detectPlatform(categories)
	log.Printf("Loading %s from disk (%d categories, platform=%s)", swName, len(categories), platform)

	// 1. System info
	if raw, ok := categories["system-openconfig"]; ok {
		sysInfo := transform.ParseSystemOpenConfig(raw.Notifications)
		sd.Device = topology.Device{
			ID:                swName,
			Type:              "switch",
			SystemName:        sysInfo.Hostname,
			SoftwareVersion:   sysInfo.SoftwareVersion,
			Uptime:            sysInfo.Uptime,
			ManagementAddress: "",
		}
		if sd.Device.SystemName == "" {
			sd.Device.SystemName = swName
		}
	} else if raw, ok := categories["system-nxos"]; ok {
		sysInfo := transform.ParseSystemNXOS(raw.Notifications)
		sd.Device = topology.Device{
			ID:              swName,
			Type:            "switch",
			SystemName:      swName,
			SoftwareVersion: sysInfo.SoftwareVersion,
			Uptime:          sysInfo.Uptime,
		}
	} else {
		sd.Device = topology.Device{
			ID:         swName,
			Type:       "switch",
			SystemName: swName,
		}
	}

	// 2. LLDP neighbors
	if raw, ok := categories["lldp-nxos"]; ok {
		sd.Neighbors = transform.ParseLLDPNXOS(raw.Notifications)
		log.Printf("  %s: %d LLDP neighbors (nxos)", swName, len(sd.Neighbors))
	} else if raw, ok := categories["lldp-openconfig"]; ok {
		sd.Neighbors = transform.ParseLLDPOpenConfig(raw.Notifications)
		log.Printf("  %s: %d LLDP neighbors (openconfig)", swName, len(sd.Neighbors))
	}

	// 3. Interfaces
	if raw, ok := categories["interfaces-nxos"]; ok {
		sd.Interfaces = transform.ParseInterfacesNXOS(raw.Notifications)
		log.Printf("  %s: %d interfaces (nxos)", swName, len(sd.Interfaces))
	} else if raw, ok := categories["interfaces-openconfig"]; ok {
		sd.Interfaces = transform.ParseInterfacesOpenConfig(raw.Notifications)

		// Merge counters
		if counters, ok := categories["interface-counters-openconfig"]; ok {
			transform.MergeInterfaceCounters(sd.Interfaces, counters.Notifications)
		}

		// Merge VLAN configs (NX-OS)
		if vlanCfg, ok := categories["interface-vlans-nxos"]; ok {
			vlanConfigs := transform.ParseInterfaceVLANsNXOS(vlanCfg.Notifications)
			transform.MergeInterfaceVLANConfigs(sd.Interfaces, vlanConfigs)
			log.Printf("  %s: %d interface VLAN configs", swName, len(vlanConfigs))
		}

		log.Printf("  %s: %d interfaces", swName, len(sd.Interfaces))
	}

	// 4. CPU/memory resources
	var cpuNotifs, memNotifs []gnmi.Notification
	if raw, ok := categories["cpu-nxos"]; ok {
		cpuNotifs = raw.Notifications
	} else if raw, ok := categories["cpu-openconfig"]; ok {
		cpuNotifs = raw.Notifications
	}
	if raw, ok := categories["memory-nxos"]; ok {
		memNotifs = raw.Notifications
	} else if raw, ok := categories["memory-openconfig"]; ok {
		memNotifs = raw.Notifications
	}
	if cpuNotifs != nil || memNotifs != nil {
		stats := transform.ParseResourceStatsNXOS(cpuNotifs, memNotifs)
		sd.Device.CPUUtilization = stats.CPUUtilization
		sd.Device.MemoryUsed = stats.MemoryUsed
		sd.Device.MemoryTotal = stats.MemoryTotal
	}

	// 5. MAC table
	if raw, ok := categories["mac-table-nxos"]; ok {
		sd.MACEntries = transform.ParseMACTableNXOS(raw.Notifications, swName)
		log.Printf("  %s: %d MAC entries", swName, len(sd.MACEntries))
	}

	// 6. ARP table
	if raw, ok := categories["arp-table-nxos"]; ok {
		sd.ARPEntries = transform.ParseARPTableNXOS(raw.Notifications, swName)
		log.Printf("  %s: %d ARP entries", swName, len(sd.ARPEntries))
	}

	// 7. VLANs
	if raw, ok := categories["vlan-config-nxos"]; ok {
		sd.VLANs = transform.ParseVLANsNXOS(raw.Notifications, swName)
		log.Printf("  %s: %d VLANs", swName, len(sd.VLANs))
	}

	// 8. BGP
	if raw, ok := categories["bgp-nxos"]; ok {
		sd.BGPNeighbors = transform.ParseBGPNXOS(raw.Notifications)
		log.Printf("  %s: %d BGP neighbors", swName, len(sd.BGPNeighbors))
	} else if raw, ok := categories["bgp-openconfig"]; ok {
		sd.BGPNeighbors = transform.ParseBGPOpenConfig(raw.Notifications)
		log.Printf("  %s: %d BGP neighbors", swName, len(sd.BGPNeighbors))
	}

	return sd, nil
}

// loadDumpFile reads and deserializes a single raw dump JSON file.
// It supports two formats:
//  1. rawDumpFile wrapper: {"switch":"...", "category":"...", "notifications":[...]}
//  2. gnmic raw output: [{"source":"...", "timestamp":..., "updates":[...]}]
//
// For format 2, the category is inferred from the filename.
func loadDumpFile(path string) (*rawDumpFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Detect format: gnmic raw is a JSON array, rawDumpFile is a JSON object.
	trimmed := trimLeadingWhitespace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return loadGnmicRawFile(data, path)
	}

	// Check for error-only files (single JSON object with "error" field)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var errObj struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &errObj) == nil && errObj.Error != "" {
			return nil, fmt.Errorf("%s: %s", filepath.Base(path), errObj.Error)
		}
	}

	var raw rawDumpFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	if raw.Category == "" {
		return nil, fmt.Errorf("%s: missing 'category' field", filepath.Base(path))
	}
	return &raw, nil
}

// gnmicNotification is the on-wire format produced by gnmic's Get/Subscribe output.
type gnmicNotification struct {
	Source    string        `json:"source"`
	Timestamp int64        `json:"timestamp"`
	Time     string        `json:"time"`
	Updates  []gnmicUpdate `json:"updates"`
}

type gnmicUpdate struct {
	Path   string                 `json:"Path"`
	Values map[string]interface{} `json:"values"`
}

// loadGnmicRawFile parses the gnmic raw JSON array format and converts it into
// a rawDumpFile with proper category and notifications.
func loadGnmicRawFile(data []byte, path string) (*rawDumpFile, error) {
	var gnmicNotifs []gnmicNotification
	if err := json.Unmarshal(data, &gnmicNotifs); err != nil {
		return nil, fmt.Errorf("parsing gnmic format %s: %w", filepath.Base(path), err)
	}
	if len(gnmicNotifs) == 0 {
		return nil, fmt.Errorf("%s: empty notification array", filepath.Base(path))
	}

	category := inferCategoryFromFilename(filepath.Base(path))
	if category == "" {
		return nil, fmt.Errorf("%s: cannot infer category from filename", filepath.Base(path))
	}

	// Convert gnmic notifications to our internal Notification format
	var notifications []gnmi.Notification
	for _, gn := range gnmicNotifs {
		var updates []gnmi.Update
		for _, gu := range gn.Updates {
			// gnmic stores values as {"path": value} — extract the value
			for p, v := range gu.Values {
				updates = append(updates, gnmi.Update{
					Path:  p,
					Value: v,
				})
			}
		}
		notifications = append(notifications, gnmi.Notification{
			Timestamp: gn.Timestamp,
			Updates:   updates,
		})
	}

	return &rawDumpFile{
		Switch:        gnmicNotifs[0].Source,
		Category:      category,
		Platform:      "nxos",
		Notifications: notifications,
	}, nil
}

// inferCategoryFromFilename maps gnmic-style filenames to loader categories.
// Examples:
//
//	System_lldp-items_inst-items_if-items_If-list.json → lldp-nxos
//	System_bd-items.json → vlan-config-nxos
//	openconfig-lldp_lldp_interfaces_interface_neighbors.json → lldp-openconfig
func inferCategoryFromFilename(name string) string {
	// Strip .json extension
	name = strings.TrimSuffix(name, ".json")

	// NX-OS native paths (System_*)
	nxosMap := map[string]string{
		"System_lldp-items_inst-items_if-items_If-list":                                       "lldp-nxos",
		"System_bd-items":                                                                      "vlan-config-nxos",
		"System_intf-items_phys-items_PhysIf-list":                                             "interfaces-nxos",
		"System_intf-items_phys-items_PhysIf-list_dbgIfIn-items":                               "interface-counters-nxos",
		"System_mac-items":                                                                     "mac-table-nxos",
		"System_arp-items_inst-items_dom-items_Dom-list_db-items_Db-list_adj-items_AdjEp-list": "arp-table-nxos",
		"System_bgp-items_inst-items_dom-items_Dom-list_peer-items_Peer-list":                  "bgp-nxos",
		"System_procsys-items_syscpusummary-items":                                             "cpu-nxos",
		"System_procsys-items_sysmem-items":                                                    "memory-nxos",
		"System_showversion-items":                                                             "system-nxos",
	}
	if cat, ok := nxosMap[name]; ok {
		return cat
	}

	// OpenConfig paths
	ocMap := map[string]string{
		"openconfig-lldp_lldp_interfaces_interface_neighbors":                                               "lldp-openconfig",
		"openconfig-interfaces_interfaces_interface_state":                                                   "interfaces-openconfig",
		"openconfig-interfaces_interfaces_interface_state_counters":                                          "interface-counters-openconfig",
		"openconfig-system_system_state":                                                                     "system-openconfig",
		"openconfig-network-instance_network-instances_network-instance_protocols_protocol_bgp_neighbors":    "bgp-openconfig",
		"openconfig-network-instance_network-instances_network-instance_vlans_vlan":                          "vlan-openconfig",
	}
	if cat, ok := ocMap[name]; ok {
		return cat
	}

	return ""
}

func trimLeadingWhitespace(data []byte) []byte {
	for i, b := range data {
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return data[i:]
		}
	}
	return nil
}

// detectPlatform infers the switch platform from the available data files.
func detectPlatform(categories map[string]*rawDumpFile) string {
	for _, raw := range categories {
		if raw.Platform != "" {
			return raw.Platform
		}
	}
	// Infer from category names
	for cat := range categories {
		if len(cat) > 5 && cat[len(cat)-5:] == "-nxos" {
			return "nxos"
		}
	}
	return "unknown"
}
