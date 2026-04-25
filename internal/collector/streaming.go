package collector

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/topology"
)

// StreamingCollector performs periodic gNMI collection and notifies
// listeners when the topology changes.
type StreamingCollector struct {
	cfg      *config.Config
	interval time.Duration

	mu       sync.RWMutex
	current  *topology.Topology
	snapshot []byte // JSON snapshot for change detection

	onChange func(*topology.Topology)
}

// NewStreamingCollector creates a collector that periodically re-collects
// topology and calls onChange when the topology differs from the previous run.
func NewStreamingCollector(cfg *config.Config, interval time.Duration, onChange func(*topology.Topology)) *StreamingCollector {
	return &StreamingCollector{
		cfg:      cfg,
		interval: interval,
		onChange: onChange,
	}
}

// Start performs an initial collection, then re-collects on the configured
// interval until ctx is canceled. It blocks until ctx is done.
func (sc *StreamingCollector) Start(ctx context.Context) error {
	// Initial collection
	sc.recollect(ctx)

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			sc.recollect(ctx)
		}
	}
}

// GetTopology returns the current topology snapshot (thread-safe).
func (sc *StreamingCollector) GetTopology() *topology.Topology {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.current
}

func (sc *StreamingCollector) recollect(ctx context.Context) {
	log.Println("[streaming] Starting collection cycle...")

	topo, err := Collect(ctx, sc.cfg)
	if err != nil {
		log.Printf("[streaming] Collection error: %v", err)
		return
	}

	newSnapshot, err := json.Marshal(topo)
	if err != nil {
		log.Printf("[streaming] Marshal error: %v", err)
		return
	}

	sc.mu.Lock()
	changed := string(newSnapshot) != string(sc.snapshot)
	sc.current = topo
	sc.snapshot = newSnapshot
	sc.mu.Unlock()

	if changed {
		log.Printf("[streaming] Topology changed: %d devices, %d links", len(topo.Devices), len(topo.Links))
		if sc.onChange != nil {
			sc.onChange(topo)
		}
	} else {
		log.Println("[streaming] No topology changes detected")
	}
}
