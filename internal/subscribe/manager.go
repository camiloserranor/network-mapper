// Package subscribe manages persistent gNMI ON_CHANGE subscriptions
// to detect topology changes in real time.
package subscribe

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/camiloserranor/network-mapper/internal/config"
	"github.com/camiloserranor/network-mapper/internal/gnmi"
)

// OnChangePaths defines which gNMI paths support ON_CHANGE subscriptions per platform.
var OnChangePaths = map[string][]string{
	"nxos": {
		"/System/lldp-items/inst-items/if-items/If-list",
		"/openconfig-interfaces:interfaces/interface/state",
	},
	"sonic": {
		"/openconfig-lldp:lldp/interfaces/interface/neighbors",
		"/openconfig-interfaces:interfaces/interface/state",
	},
}

// Manager maintains persistent ON_CHANGE subscriptions to all configured switches.
// When any switch reports a change, it signals via the Changes channel.
type Manager struct {
	cfg     *config.Config
	Changes chan string // receives switch name on change
	wg      sync.WaitGroup
}

// NewManager creates a subscribe manager for the given configuration.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:     cfg,
		Changes: make(chan string, 64),
	}
}

// Start opens ON_CHANGE subscriptions to all configured switches.
// Each switch runs in its own goroutine with automatic reconnection.
// Blocks until ctx is canceled.
func (m *Manager) Start(ctx context.Context) {
	for _, sw := range m.cfg.Switches {
		paths := OnChangePaths[sw.Platform]
		if len(paths) == 0 {
			log.Printf("[subscribe] No ON_CHANGE paths defined for platform %q (%s), skipping", sw.Platform, sw.Name)
			continue
		}

		m.wg.Add(1)
		go func(sw config.SwitchConfig, paths []string) {
			defer m.wg.Done()
			m.watchSwitch(ctx, sw, paths)
		}(sw, paths)
	}

	m.wg.Wait()
}

func (m *Manager) watchSwitch(ctx context.Context, sw config.SwitchConfig, paths []string) {
	backoff := time.Second
	maxBackoff := 2 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("[subscribe] Connecting to %s for ON_CHANGE subscription (%d paths)", sw.Name, len(paths))

		err := m.subscribeOnce(ctx, sw, paths)
		if ctx.Err() != nil {
			return // context canceled, clean exit
		}

		if err != nil {
			log.Printf("[subscribe] Subscription to %s failed: %v (reconnecting in %s)", sw.Name, err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff with cap
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *Manager) subscribeOnce(ctx context.Context, sw config.SwitchConfig, paths []string) error {
	clientOpts := gnmi.ClientOptions{
		Address:  sw.Address,
		Username: sw.Auth.Username,
		Password: sw.Auth.Password,
		TLS: gnmi.TLSOptions{
			SkipVerify: m.cfg.TLS.SkipVerify,
			TOFU:       m.cfg.TLS.TOFU,
			CertDir:    m.cfg.TLS.CertDir,
			CACert:     m.cfg.TLS.CACert,
		},
		Encoding:   "JSON",
		TimeoutSec: m.cfg.Collect.TimeoutSec,
	}
	if sw.Platform == "sonic" {
		clientOpts.Encoding = "JSON_IETF"
	}

	connCtx, connCancel := context.WithTimeout(ctx, time.Duration(m.cfg.Collect.TimeoutSec)*time.Second)
	client, err := gnmi.NewClient(connCtx, clientOpts)
	connCancel()
	if err != nil {
		return err
	}
	defer client.Close()

	// Reset backoff on successful connection
	return client.SubscribeStream(ctx, paths, func(path string) {
		log.Printf("[subscribe] ON_CHANGE from %s: %s", sw.Name, path)
		// Non-blocking send — if channel is full, skip (collector will catch up)
		select {
		case m.Changes <- sw.Name:
		default:
		}
	})
}
