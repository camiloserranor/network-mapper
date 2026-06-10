// Package collector — dumper.go provides raw gNMI data export to disk,
// producing the same directory structure that LoadFromDisk expects.
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/platform"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// dataCategory defines a gNMI path to query and its label for file output.
type dataCategory struct {
	Name string
	Path string
}

// categoriesForPlatform returns the data categories to dump based on platform type.
func categoriesForPlatform(p string) []dataCategory {
	switch p {
	case "nxos":
		return []dataCategory{
			{"lldp-nxos", transform.LLDPPathNXOS},
			{"interfaces-nxos", transform.InterfacesPathNXOS},
			{"interfaces-openconfig", transform.InterfacesPathOpenConfig},
			{"interfaces-counters-openconfig", transform.InterfacesCountersPathOpenConfig},
			{"system-nxos", transform.SystemPathNXOS},
			{"mac-table-nxos", transform.MACTablePathNXOS},
			{"arp-nxos", transform.ARPPathNXOS},
			{"vlan-nxos", transform.VLANPathNXOS},
			{"bgp-nxos", transform.BGPNeighborsPathNXOS},
			{"nve-peers-nxos", transform.NVEPeersPathNXOS},
			{"l2rib-nxos", transform.L2RIBPathNXOS},
			{"qos-stats-nxos", transform.QoSStatsPathNXOS},
			{"pfc-config-nxos", transform.PFCConfigPathNXOS},
			{"interface-vlans-nxos", transform.InterfaceVLANPathNXOS},
			{"cpu-nxos", transform.CPUPathNXOS},
			{"memory-nxos", transform.MemoryPathNXOS},
		}
	default: // sonic / openconfig
		return []dataCategory{
			{"lldp-openconfig", transform.LLDPPathOpenConfig},
			{"interfaces-openconfig", transform.InterfacesPathOpenConfig},
			{"interfaces-counters-openconfig", transform.InterfacesCountersPathOpenConfig},
			{"system-openconfig", transform.SystemPathOpenConfig},
		}
	}
}

// DumpRaw connects to all configured switches and dumps raw gNMI responses
// for each data category to a timestamped directory. The output is compatible
// with LoadFromDisk for offline analysis.
//
// Returns the path to the output directory.
func DumpRaw(ctx context.Context, cfg *config.Config, baseDir string) (string, error) {
	ts := time.Now().UTC()
	outDir := filepath.Join(baseDir, ts.Format("2006-01-02T150405Z"))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	var totalFiles int
	for _, sw := range cfg.Switches {
		swDir := filepath.Join(outDir, sw.Name)
		if err := os.MkdirAll(swDir, 0755); err != nil {
			return "", fmt.Errorf("creating switch directory %s: %w", sw.Name, err)
		}

		p := platform.ForPlatform(sw.Platform)

		client, err := gnmi.NewClient(ctx, gnmi.ClientOptions{
			Address:  sw.Address,
			Username: sw.Auth.Username,
			Password: sw.Auth.Password,
			TLS: gnmi.TLSOptions{
				SkipVerify: cfg.TLS.SkipVerify,
				TOFU:       cfg.TLS.TOFU,
				CertDir:    cfg.TLS.CertDir,
				CACert:     cfg.TLS.CACert,
				ClientCert: cfg.TLS.ClientCert,
				ClientKey:  cfg.TLS.ClientKey,
			},
			Encoding: p.Encoding(),
		})
		if err != nil {
			log.Printf("  %s: connection failed: %v", sw.Name, err)
			continue
		}

		categories := categoriesForPlatform(sw.Platform)
		for _, cat := range categories {
			notifs, err := client.GetWithFallback(ctx, cat.Path)
			if err != nil {
				log.Printf("  %s: %s: %v", sw.Name, cat.Name, err)
				continue
			}
			if len(notifs) == 0 {
				continue
			}

			dump := rawDumpFile{
				Switch:            sw.Name,
				Category:          cat.Name,
				Platform:          sw.Platform,
				Path:              cat.Path,
				Timestamp:         ts.Format(time.RFC3339),
				NotificationCount: len(notifs),
				Notifications:     notifs,
			}

			data, err := json.MarshalIndent(dump, "", "  ")
			if err != nil {
				log.Printf("  %s: %s: marshal error: %v", sw.Name, cat.Name, err)
				continue
			}

			outFile := filepath.Join(swDir, cat.Name+".json")
			if err := os.WriteFile(outFile, data, 0644); err != nil {
				return "", fmt.Errorf("writing %s: %w", outFile, err)
			}
			totalFiles++
		}

		client.Close()
		log.Printf("  %s: dumped %d categories", sw.Name, len(categories))
	}

	// Write summary
	summary := map[string]interface{}{
		"timestamp":    ts.Format(time.RFC3339),
		"switch_count": len(cfg.Switches),
		"total_files":  totalFiles,
	}
	summaryData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(filepath.Join(outDir, "_summary.json"), summaryData, 0644)

	return outDir, nil
}
