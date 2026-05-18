// Command gnmi-dump connects to switches defined in a config file and saves
// raw gNMI responses for every data category the network-mapper collects.
// This is a diagnostic/analysis tool — not part of the production binary.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/transform"
)

// dataCategory defines a gNMI path to collect and the file name to save it as.
type dataCategory struct {
	Name     string // human-readable label and output file name
	Path     string // gNMI YANG path
	Platform string // "nxos", "openconfig", or "all"
}

// All data categories that network-mapper collects.
var categories = []dataCategory{
	// System info
	{Name: "system-openconfig", Path: transform.SystemPathOpenConfig, Platform: "all"},

	// LLDP
	{Name: "lldp-openconfig", Path: transform.LLDPPathOpenConfig, Platform: "all"},
	{Name: "lldp-nxos", Path: transform.LLDPPathNXOS, Platform: "nxos"},

	// Interfaces
	{Name: "interfaces-openconfig", Path: transform.InterfacesPathOpenConfig, Platform: "all"},
	{Name: "interface-counters-openconfig", Path: transform.InterfacesCountersPathOpenConfig, Platform: "all"},
	{Name: "interface-vlans-nxos", Path: transform.InterfaceVLANPathNXOS, Platform: "nxos"},

	// Resources
	{Name: "cpu-nxos", Path: transform.CPUPathNXOS, Platform: "nxos"},
	{Name: "memory-nxos", Path: transform.MemoryPathNXOS, Platform: "nxos"},
	{Name: "cpu-openconfig", Path: transform.CPUPathOpenConfig, Platform: "all"},
	{Name: "memory-openconfig", Path: transform.MemoryPathOpenConfig, Platform: "all"},

	// MAC table
	{Name: "mac-table-nxos", Path: transform.MACTablePathNXOS, Platform: "nxos"},

	// ARP table
	{Name: "arp-table-nxos", Path: transform.ARPPathNXOS, Platform: "nxos"},

	// VLANs
	{Name: "vlan-config-nxos", Path: transform.VLANPathNXOS, Platform: "nxos"},
	{Name: "vlan-openconfig", Path: transform.VLANPathOpenConfig, Platform: "all"},

	// BGP
	{Name: "bgp-openconfig", Path: transform.BGPNeighborsPathOpenConfig, Platform: "all"},
	{Name: "bgp-nxos", Path: transform.BGPNeighborsPathNXOS, Platform: "nxos"},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: gnmi-dump <config.yaml> [output-dir]\n")
		os.Exit(1)
	}

	configPath := os.Args[1]
	outputDir := "gnmi-raw-data"
	if len(os.Args) >= 3 {
		outputDir = os.Args[2]
	}

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create timestamped output directory
	timestamp := time.Now().Format("2006-01-02_150405")
	outputDir = filepath.Join(outputDir, timestamp)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	log.Printf("Output directory: %s", outputDir)

	// Process each switch
	for _, sw := range cfg.Switches {
		log.Printf("\n========== %s (%s) ==========", sw.Name, sw.Address)

		switchDir := filepath.Join(outputDir, sanitizeFilename(sw.Name))
		if err := os.MkdirAll(switchDir, 0755); err != nil {
			log.Printf("ERROR creating dir for %s: %v", sw.Name, err)
			continue
		}

		// Determine encoding
		encoding := "JSON_IETF"
		if sw.Platform == "nxos" {
			encoding = "JSON"
		}

		// Connect
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
			Encoding: encoding,
		})
		if err != nil {
			log.Printf("ERROR connecting to %s: %v", sw.Name, err)
			// Save the error
			writeFile(filepath.Join(switchDir, "_connection_error.txt"),
				fmt.Sprintf("Switch: %s\nAddress: %s\nError: %v\n", sw.Name, sw.Address, err))
			cancel()
			continue
		}

		// Save capabilities
		caps, capErr := client.Capabilities(ctx)
		if capErr != nil {
			log.Printf("  WARN: capabilities: %v", capErr)
		} else {
			capJSON, _ := json.MarshalIndent(caps, "", "  ")
			writeFile(filepath.Join(switchDir, "capabilities.json"), string(capJSON))
			log.Printf("  Capabilities: gNMI %s, %d models, encodings: %v",
				caps.GNMIVersion, len(caps.Models), caps.Encodings)
		}

		// Collect each data category
		for _, cat := range categories {
			if cat.Platform == "nxos" && sw.Platform != "nxos" {
				continue
			}

			log.Printf("  Collecting: %s  [%s]", cat.Name, cat.Path)
			start := time.Now()

			notifs, err := client.GetWithFallback(ctx, cat.Path)
			elapsed := time.Since(start)

			result := rawDumpResult{
				Switch:    sw.Name,
				Category:  cat.Name,
				Path:      cat.Path,
				Platform:  sw.Platform,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Duration:  elapsed.String(),
			}

			if err != nil {
				result.Error = err.Error()
				log.Printf("    ERROR (%s): %v", elapsed, err)
			} else {
				result.NotificationCount = len(notifs)
				updateCount := 0
				for _, n := range notifs {
					updateCount += len(n.Updates)
				}
				result.UpdateCount = updateCount
				result.Notifications = notifs
				log.Printf("    OK (%s): %d notifications, %d updates", elapsed, len(notifs), updateCount)
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			writeFile(filepath.Join(switchDir, cat.Name+".json"), string(resultJSON))
		}

		client.Close()
		cancel()
		log.Printf("  %s: done", sw.Name)
	}

	// Write summary
	writeSummary(outputDir, cfg)
	log.Printf("\nAll done. Raw data saved to: %s", outputDir)
}

type rawDumpResult struct {
	Switch            string              `json:"switch"`
	Category          string              `json:"category"`
	Path              string              `json:"path"`
	Platform          string              `json:"platform"`
	Timestamp         string              `json:"timestamp"`
	Duration          string              `json:"duration"`
	Error             string              `json:"error,omitempty"`
	NotificationCount int                 `json:"notification_count"`
	UpdateCount       int                 `json:"update_count"`
	Notifications     []gnmi.Notification `json:"notifications,omitempty"`
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("ERROR writing %s: %v", path, err)
	}
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(s)
}

func writeSummary(outputDir string, cfg *config.Config) {
	summary := struct {
		Timestamp string   `json:"timestamp"`
		Switches  []string `json:"switches"`
		Categories []string `json:"categories"`
	}{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	for _, sw := range cfg.Switches {
		summary.Switches = append(summary.Switches, fmt.Sprintf("%s (%s) [%s]", sw.Name, sw.Address, sw.Platform))
	}
	for _, cat := range categories {
		summary.Categories = append(summary.Categories, fmt.Sprintf("%s: %s [%s]", cat.Name, cat.Path, cat.Platform))
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	writeFile(filepath.Join(outputDir, "_summary.json"), string(data))
}
