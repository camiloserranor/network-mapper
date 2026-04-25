package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/camiloserranor/network-mapper/internal/collector"
	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/gnmi"
	"github.com/camiloserranor/network-mapper/internal/server"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

//go:embed web
var embeddedWeb embed.FS

func main() {
	rootCmd := &cobra.Command{
		Use:   "network-mapper",
		Short: "Physical topology discovery for Azure Local deployments",
		Long:  "Network Mapper discovers the physical topology of Azure Local deployments by querying TOR switches via gNMI to retrieve LLDP neighbor data.",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web UI server",
		Long:  "Start a local HTTP server that serves an interactive graph visualization of the network topology.",
		RunE:  runServe,
	}

	serveCmd.Flags().StringP("topology", "t", "topology.json", "Path to topology JSON file")
	serveCmd.Flags().IntP("port", "p", 8080, "HTTP server port")
	serveCmd.Flags().Bool("no-open", false, "Don't auto-open browser")
	serveCmd.Flags().StringP("config", "c", "", "Config file for live mode (enables periodic gNMI collection)")
	serveCmd.Flags().Int("interval", 30, "Collection interval in seconds for live mode")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(collectCmd())
	rootCmd.AddCommand(testConnectionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	topologyPath, _ := cmd.Flags().GetString("topology")
	port, _ := cmd.Flags().GetInt("port")
	noOpen, _ := cmd.Flags().GetBool("no-open")
	cfgPath, _ := cmd.Flags().GetString("config")
	intervalSec, _ := cmd.Flags().GetInt("interval")

	srv := server.New(topologyPath, port, webFS())

	addr := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Network Mapper UI starting at %s\n", addr)

	if cfgPath != "" {
		// Live mode: periodic gNMI collection + WebSocket push
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		interval := time.Duration(intervalSec) * time.Second
		fmt.Printf("Live mode: collecting from %d switch(es) every %s\n", len(cfg.Switches), interval)

		srv.SetLiveMode(nil)

		sc := collector.NewStreamingCollector(cfg, interval, func(topo *topology.Topology) {
			srv.UpdateTopology(topo)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			if err := sc.Start(ctx); err != nil && err != context.Canceled {
				fmt.Fprintf(os.Stderr, "Streaming collector error: %v\n", err)
			}
		}()
	} else {
		fmt.Printf("Static mode: serving %s\n", topologyPath)
	}

	if !noOpen {
		go server.OpenBrowser(addr)
	}

	return srv.Start()
}

func webFS() fs.FS {
	sub, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		panic(fmt.Sprintf("failed to get embedded web filesystem: %v", err))
	}
	return sub
}

func collectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect network topology from TOR switches via gNMI",
		Long:  "Connect to configured TOR switches via gNMI, retrieve LLDP neighbor data, interface state, and system info, then output a topology JSON file.",
		RunE:  runCollect,
	}

	cmd.Flags().StringP("config", "c", "config.yaml", "Path to configuration file")
	cmd.Flags().StringP("output", "o", "topology.json", "Path to write topology JSON output")

	return cmd
}

func runCollect(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")
	outputPath, _ := cmd.Flags().GetString("output")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("Collecting topology from %d switch(es)...\n", len(cfg.Switches))

	topo, err := collector.Collect(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("collection failed: %w", err)
	}

	data, err := json.MarshalIndent(topo, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling topology: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Printf("Topology written to %s\n", outputPath)
	fmt.Printf("  Devices: %d\n", len(topo.Devices))
	fmt.Printf("  Links:   %d\n", len(topo.Links))
	if len(topo.PartialFailures) > 0 {
		fmt.Printf("  Warnings: %d partial failures\n", len(topo.PartialFailures))
	}

	return nil
}

func testConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test-connection",
		Short: "Test gNMI connectivity to a switch",
		Long: `Test gNMI connectivity to a single switch. Verifies TCP reachability,
TLS handshake, authentication, and gNMI Capabilities response.

Use this to verify your devbox can reach a switch before running a full collect.`,
		RunE: runTestConnection,
	}

	cmd.Flags().StringP("address", "a", "", "Switch address (host:port, e.g., 10.0.0.1:50051)")
	cmd.Flags().StringP("username", "u", "", "Username for authentication")
	cmd.Flags().StringP("password", "p", "", "Password (or set SWITCH_PASSWORD env var)")
	cmd.Flags().String("platform", "nxos", "Platform type: sonic or nxos")
	cmd.Flags().Bool("skip-verify", true, "Skip TLS certificate verification")
	cmd.Flags().Int("timeout", 10, "Connection timeout in seconds")

	_ = cmd.MarkFlagRequired("address")

	return cmd
}

func runTestConnection(cmd *cobra.Command, args []string) error {
	address, _ := cmd.Flags().GetString("address")
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	platform, _ := cmd.Flags().GetString("platform")
	skipVerify, _ := cmd.Flags().GetBool("skip-verify")
	timeoutSec, _ := cmd.Flags().GetInt("timeout")

	// Fall back to env var for password
	if password == "" {
		password = os.Getenv("SWITCH_PASSWORD")
	}

	encoding := "JSON_IETF"
	if platform == "nxos" {
		encoding = "JSON"
	}

	fmt.Printf("Testing gNMI connection to %s...\n", address)
	fmt.Printf("  Platform:    %s\n", platform)
	fmt.Printf("  Encoding:    %s\n", encoding)
	fmt.Printf("  TLS skip:    %v\n", skipVerify)
	fmt.Printf("  Auth:        %s / %s\n", username, maskPassword(password))
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Step 1: TCP + TLS + gRPC connect
	fmt.Print("① Connecting (gRPC + TLS)... ")
	client, err := gnmi.NewClient(ctx, gnmi.ClientOptions{
		Address:  address,
		Username: username,
		Password: password,
		TLS: gnmi.TLSOptions{
			SkipVerify: skipVerify,
		},
		Encoding: encoding,
	})
	if err != nil {
		fmt.Printf("FAIL\n   %v\n", err)
		fmt.Println("\nTroubleshooting:")
		fmt.Println("  - Is the switch IP reachable? Try: ping " + address[:findColon(address)])
		fmt.Println("  - Is gNMI enabled? SSH in and check: show feature | grep grpc")
		fmt.Println("  - Is the port correct? Common ports: 50051, 50052, 8080, 9339")
		return fmt.Errorf("connection failed")
	}
	defer client.Close()
	fmt.Println("OK ✓")

	// Step 2: gNMI Capabilities
	fmt.Print("② gNMI Capabilities RPC... ")
	caps, err := client.Capabilities(ctx)
	if err != nil {
		fmt.Printf("FAIL\n   %v\n", err)
		fmt.Println("\nThe gRPC connection succeeded but the Capabilities RPC failed.")
		fmt.Println("This usually means authentication failed. Check username/password.")
		return fmt.Errorf("capabilities failed")
	}
	fmt.Println("OK ✓")

	fmt.Printf("\n  gNMI version: %s\n", caps.GNMIVersion)
	fmt.Printf("  Encodings:    %v\n", caps.Encodings)
	fmt.Printf("  YANG models:  %d supported\n", len(caps.Models))

	// Check for LLDP model support
	hasLLDP := false
	for _, m := range caps.Models {
		if m.Name == "openconfig-lldp" {
			hasLLDP = true
			fmt.Printf("  LLDP model:   %s (v%s) ✓\n", m.Name, m.Version)
			break
		}
	}
	if !hasLLDP && platform == "sonic" {
		fmt.Println("  LLDP model:   openconfig-lldp NOT found ⚠")
	}

	// Step 3: Quick LLDP test
	fmt.Print("\n③ Test LLDP query... ")
	var lldpPath string
	if platform == "nxos" {
		lldpPath = "/System/lldp-items/inst-items/if-items/If-list"
	} else {
		lldpPath = "/openconfig-lldp:lldp/interfaces/interface/neighbors"
	}

	notifs, err := client.GetWithFallback(ctx, lldpPath)
	if err != nil {
		fmt.Printf("FAIL\n   %v\n", err)
		fmt.Println("\nConnection works but LLDP query failed. The switch may not have LLDP enabled.")
		if platform == "nxos" {
			fmt.Println("  Enable LLDP: feature lldp")
		}
		return fmt.Errorf("LLDP query failed")
	}

	neighborCount := 0
	for _, n := range notifs {
		neighborCount += len(n.Updates)
	}
	fmt.Printf("OK ✓ (%d notification updates)\n", neighborCount)

	fmt.Println("\n✅ All checks passed! You can now run:")
	fmt.Printf("   network-mapper collect --config config.yaml\n")

	return nil
}

func maskPassword(p string) string {
	if p == "" {
		return "(empty)"
	}
	if len(p) <= 4 {
		return "****"
	}
	return p[:2] + "****"
}

func findColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return len(s)
}
