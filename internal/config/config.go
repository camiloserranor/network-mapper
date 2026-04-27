// Package config handles loading and validating the network-mapper configuration.
package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/camiloserranor/network-mapper/internal/secrets"
	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for network-mapper.
type Config struct {
	Switches []SwitchConfig `yaml:"switches"`
	TLS      TLSConfig      `yaml:"tls"`
	Collect  CollectConfig  `yaml:"collect"`
}

// SwitchConfig defines how to connect to a single TOR switch.
type SwitchConfig struct {
	Address  string     `yaml:"address"`  // host:port (e.g., "10.0.0.1:8080")
	Name     string     `yaml:"name"`     // friendly name (optional, defaults to address)
	Platform string     `yaml:"platform"` // sonic, nxos
	Auth     AuthConfig `yaml:"auth"`
}

// AuthConfig defines authentication credentials for a switch.
type AuthConfig struct {
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`          // supports ${ENV_VAR} syntax
	PasswordKeyvault string `yaml:"password_keyvault"` // Key Vault secret URI (takes precedence over password)
}

// TLSConfig defines TLS settings for gNMI connections.
type TLSConfig struct {
	SkipVerify bool   `yaml:"skip_verify"` // skip TLS certificate verification
	TOFU       bool   `yaml:"tofu"`        // trust-on-first-use cert pinning
	CertDir    string `yaml:"cert_dir"`    // directory to cache switch certificates
	CACert     string `yaml:"ca_cert"`     // path to CA certificate (optional)
	ClientCert string `yaml:"client_cert"` // path to client certificate (optional, for mTLS)
	ClientKey  string `yaml:"client_key"`  // path to client key (optional, for mTLS)
}

// CollectConfig defines collection behavior.
type CollectConfig struct {
	TimeoutSec  int  `yaml:"timeout_sec"`  // per-switch timeout (default 30)
	Parallel    int  `yaml:"parallel"`     // max concurrent switch connections (default 2)
	SkipCounters bool `yaml:"skip_counters"` // skip interface counter collection
}

// Load reads and parses a configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// Resolve environment variables before parsing YAML
	resolved := resolveEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(resolved), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.applyDefaults()

	// Resolve Key Vault secrets for switches that use password_keyvault
	if err := cfg.resolveKeyVaultSecrets(); err != nil {
		return nil, fmt.Errorf("resolving Key Vault secrets: %w", err)
	}

	return &cfg, nil
}

// resolveKeyVaultSecrets fetches passwords from Azure Key Vault for switches that have password_keyvault set.
func (c *Config) resolveKeyVaultSecrets() error {
	var needsKV bool
	for _, sw := range c.Switches {
		if sw.Auth.PasswordKeyvault != "" {
			needsKV = true
			break
		}
	}

	if !needsKV {
		return nil
	}

	resolver, err := secrets.NewResolver()
	if err != nil {
		return fmt.Errorf("initializing Key Vault resolver: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := range c.Switches {
		kvURI := c.Switches[i].Auth.PasswordKeyvault
		if kvURI == "" {
			continue
		}

		log.Printf("[config] Resolving password from Key Vault for switch %q", c.Switches[i].Name)
		secret, err := resolver.Resolve(ctx, kvURI)
		if err != nil {
			return fmt.Errorf("switch %q: %w", c.Switches[i].Name, err)
		}

		c.Switches[i].Auth.Password = secret
	}

	return nil
}

// resolveEnvVars replaces ${VAR_NAME} with the environment variable value.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func resolveEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		varName := match[2 : len(match)-1] // strip ${ and }
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match // leave unresolved if env var not set
	})
}

func (c *Config) validate() error {
	if len(c.Switches) == 0 {
		return fmt.Errorf("at least one switch must be configured")
	}

	for i, sw := range c.Switches {
		if sw.Address == "" {
			return fmt.Errorf("switch[%d]: address is required", i)
		}
		if !strings.Contains(sw.Address, ":") {
			return fmt.Errorf("switch[%d]: address must include port (e.g., 10.0.0.1:8080)", i)
		}

		platform := strings.ToLower(sw.Platform)
		if platform == "" {
			platform = "sonic"
		}
		if platform != "sonic" && platform != "nxos" {
			return fmt.Errorf("switch[%d]: platform must be 'sonic' or 'nxos', got '%s'", i, sw.Platform)
		}
	}

	return nil
}

func (c *Config) applyDefaults() {
	for i := range c.Switches {
		if c.Switches[i].Name == "" {
			c.Switches[i].Name = c.Switches[i].Address
		}
		if c.Switches[i].Platform == "" {
			c.Switches[i].Platform = "sonic"
		}
		c.Switches[i].Platform = strings.ToLower(c.Switches[i].Platform)
	}

	if c.Collect.TimeoutSec <= 0 {
		c.Collect.TimeoutSec = 30
	}
	if c.Collect.Parallel <= 0 {
		c.Collect.Parallel = 2
	}

	if c.TLS.CertDir == "" {
		c.TLS.CertDir = ".certs"
	}
}
