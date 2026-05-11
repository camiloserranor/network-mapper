package collector

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/storage"
	"github.com/camiloserranor/network-mapper/internal/subscribe"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

const (
	// debounceInterval prevents multiple rapid ON_CHANGE notifications from
	// triggering back-to-back re-collections.
	debounceInterval = 5 * time.Second
)

// HybridCollector combines gNMI ON_CHANGE subscriptions with periodic polling.
// ON_CHANGE detects LLDP/interface changes in near real-time; periodic polling
// catches everything else (MAC, ARP, counters, VLANs, BGP).
type HybridCollector struct {
	cfg          *config.Config
	pollInterval time.Duration
	snapshots    *storage.SnapshotStore

	mu       sync.RWMutex
	current  *topology.Topology
	snapshot []byte // JSON for change detection

	onChange func(*topology.Topology)
}

// NewHybridCollector creates a collector that reacts to ON_CHANGE events
// and also polls periodically as a fallback.
func NewHybridCollector(cfg *config.Config, pollInterval time.Duration, snapshots *storage.SnapshotStore, onChange func(*topology.Topology)) *HybridCollector {
	return &HybridCollector{
		cfg:          cfg,
		pollInterval: pollInterval,
		snapshots:    snapshots,
		onChange:     onChange,
	}
}

// Start performs an initial collection, then listens for ON_CHANGE notifications
// and polls periodically. Blocks until ctx is canceled.
func (hc *HybridCollector) Start(ctx context.Context) error {
	// Initial full collection
	hc.collectAndSave(ctx)

	// Start subscribe manager in background
	subMgr := subscribe.NewManager(hc.cfg)
	go subMgr.Start(ctx)

	// Periodic poll ticker
	pollTicker := time.NewTicker(hc.pollInterval)
	defer pollTicker.Stop()

	// Debounce timer for ON_CHANGE events
	var debounceTimer *time.Timer
	var debounceCh <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return ctx.Err()

		case switchName := <-subMgr.Changes:
			log.Printf("[hybrid] ON_CHANGE from %s — scheduling re-collect", switchName)
			// Reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(debounceInterval)
			debounceCh = debounceTimer.C

		case <-debounceCh:
			log.Printf("[hybrid] Debounce expired — running full re-collect")
			hc.collectAndSave(ctx)
			debounceTimer = nil
			debounceCh = nil

		case <-pollTicker.C:
			log.Printf("[hybrid] Periodic poll — running full re-collect")
			hc.collectAndSave(ctx)
		}
	}
}

// GetTopology returns the current topology snapshot (thread-safe).
func (hc *HybridCollector) GetTopology() *topology.Topology {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.current
}

func (hc *HybridCollector) collectAndSave(ctx context.Context) {
	start := time.Now()
	log.Println("[hybrid] Starting collection cycle...")

	topo, err := Collect(ctx, hc.cfg)
	if err != nil {
		log.Printf("[hybrid] Collection error: %v", err)
		return
	}

	elapsed := time.Since(start)
	log.Printf("[hybrid] Collection cycle completed in %s", elapsed)

	newSnapshot, err := json.Marshal(topo)
	if err != nil {
		log.Printf("[hybrid] Marshal error: %v", err)
		return
	}

	hc.mu.Lock()
	changed := string(newSnapshot) != string(hc.snapshot)
	hc.current = topo
	hc.snapshot = newSnapshot
	hc.mu.Unlock()

	if changed {
		log.Printf("[hybrid] Topology changed: %d devices, %d links (collected in %s)", len(topo.Devices), len(topo.Links), elapsed)

		// Save snapshot to disk
		if hc.snapshots != nil {
			if _, err := hc.snapshots.Save(topo); err != nil {
				log.Printf("[hybrid] Failed to save snapshot: %v", err)
			}
		}

		if hc.onChange != nil {
			hc.onChange(topo)
		}
	} else {
		log.Printf("[hybrid] No topology changes detected (collected in %s)", elapsed)
	}
}
