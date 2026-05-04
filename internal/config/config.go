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
Auth           AuthConfig     `yaml:"auth"`            // global auth (inherited by all switches unless overridden)
Switches       []SwitchConfig `yaml:"switches"`
TLS            TLSConfig      `yaml:"tls"`
Collect        CollectConfig  `yaml:"collect"`
DeploymentJSON string         `yaml:"deployment_json"` // optional path to Azure Local deployment design JSON
}

// SwitchConfig defines how to connect to a single TOR switch.
type SwitchConfig struct {
Address  string     `yaml:"address"`  // host:port (e.g., "10.0.0.1:8080")
Name     string     `yaml:"name"`     // friendly name (optional, defaults to address)
Platform string     `yaml:"platform"` // sonic, nxos
Auth     AuthConfig `yaml:"auth"`     // per-switch override (empty fields fall back to global)
}

// AuthConfig defines authentication credentials.
// Resolution priority for each field: keyvault URI > plaintext/env var.
// Per-switch auth overrides global auth for any non-empty field.
type AuthConfig struct {
Username         string `yaml:"username"`          // supports ${ENV_VAR} syntax
UsernameKeyvault string `yaml:"username_keyvault"` // Key Vault secret URI for username
Password         string `yaml:"password"`          // supports ${ENV_VAR} syntax
PasswordKeyvault string `yaml:"password_keyvault"` // Key Vault secret URI for password
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
TimeoutSec   int  `yaml:"timeout_sec"`   // per-switch timeout (default 30)
Parallel     int  `yaml:"parallel"`      // max concurrent switch connections (default 2)
SkipCounters bool `yaml:"skip_counters"` // skip interface counter collection
ReverseDNS   bool `yaml:"reverse_dns"`   // attempt reverse DNS for host IP→hostname (default false)
}

// Load reads and parses a configuration file.
func Load(path string) (*Config, error) {
log.Printf("[config] Loading configuration from %s", path)
data, err := os.ReadFile(path)
if err != nil {
return nil, fmt.Errorf("reading config: %w", err)
}
log.Printf("[config] Read %d bytes from config file", len(data))

// Resolve environment variables before parsing YAML
resolved := resolveEnvVars(string(data))

var cfg Config
if err := yaml.Unmarshal([]byte(resolved), &cfg); err != nil {
return nil, fmt.Errorf("parsing config: %w", err)
}

log.Printf("[config] Parsed config: %d switch(es) configured", len(cfg.Switches))

if err := cfg.validate(); err != nil {
return nil, fmt.Errorf("invalid config: %w", err)
}

cfg.applyDefaults()

// Resolve Key Vault secrets for switches that use password_keyvault
if err := cfg.resolveKeyVaultSecrets(); err != nil {
return nil, fmt.Errorf("resolving Key Vault secrets: %w", err)
}

log.Printf("[config] Configuration loaded successfully")
return &cfg, nil
}

// resolveKeyVaultSecrets resolves Key Vault URIs for username and password.
// It applies global auth inheritance first, then resolves any KV URIs.
func (c *Config) resolveKeyVaultSecrets() error {
log.Printf("[config] Applying global auth inheritance to %d switch(es)", len(c.Switches))
for i := range c.Switches {
name := c.Switches[i].Name
if name == "" {
name = c.Switches[i].Address
}
before := c.Switches[i].Auth
c.Switches[i].Auth = mergeAuth(c.Auth, c.Switches[i].Auth)
logAuthMerge(name, before, c.Auth)
}

// Check if any switch needs Key Vault resolution
var kvSwitches []string
for _, sw := range c.Switches {
if sw.Auth.UsernameKeyvault != "" || sw.Auth.PasswordKeyvault != "" {
name := sw.Name
if name == "" {
name = sw.Address
}
kvSwitches = append(kvSwitches, name)
}
}
if len(kvSwitches) == 0 {
log.Printf("[config] No Key Vault URIs configured - using plaintext/env credentials for all switches")
return nil
}

log.Printf("[config] Key Vault resolution needed for %d switch(es): %v", len(kvSwitches), kvSwitches)

resolver, err := secrets.NewResolver()
if err != nil {
log.Printf("[config] ERROR: Key Vault resolver initialization failed: %v", err)
return fmt.Errorf("initializing Key Vault resolver: %w", err)
}

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
log.Printf("[config] Resolving secrets (timeout: 30s) ...")

var resolved int
for i := range c.Switches {
sw := &c.Switches[i]
name := sw.Name
if name == "" {
name = sw.Address
}

if sw.Auth.UsernameKeyvault != "" {
log.Printf("[config] [%s] Resolving username from Key Vault: %s", name, redactSecretURI(sw.Auth.UsernameKeyvault))
secret, err := resolver.Resolve(ctx, sw.Auth.UsernameKeyvault)
if err != nil {
log.Printf("[config] [%s] ERROR: Failed to resolve username: %v", name, err)
return fmt.Errorf("switch %q username: %w", name, err)
}
sw.Auth.Username = secret
log.Printf("[config] [%s] Username resolved successfully (%d chars)", name, len(secret))
resolved++
} else if sw.Auth.Username != "" {
log.Printf("[config] [%s] Username source: plaintext/env var", name)
} else {
log.Printf("[config] [%s] WARNING: No username configured", name)
}

if sw.Auth.PasswordKeyvault != "" {
log.Printf("[config] [%s] Resolving password from Key Vault: %s", name, redactSecretURI(sw.Auth.PasswordKeyvault))
secret, err := resolver.Resolve(ctx, sw.Auth.PasswordKeyvault)
if err != nil {
log.Printf("[config] [%s] ERROR: Failed to resolve password: %v", name, err)
return fmt.Errorf("switch %q password: %w", name, err)
}
sw.Auth.Password = secret
log.Printf("[config] [%s] Password resolved successfully (%d chars)", name, len(secret))
resolved++
} else if sw.Auth.Password != "" {
log.Printf("[config] [%s] Password source: plaintext/env var", name)
} else {
log.Printf("[config] [%s] WARNING: No password configured", name)
}
}

log.Printf("[config] Key Vault resolution complete - %d secret(s) resolved for %d switch(es)", resolved, len(kvSwitches))
return nil
}

// mergeAuth merges global auth with per-switch auth.
// Per-switch values take precedence over global for any non-empty field.
func mergeAuth(global, perSwitch AuthConfig) AuthConfig {
result := global
if perSwitch.Username != "" {
result.Username = perSwitch.Username
}
if perSwitch.UsernameKeyvault != "" {
result.UsernameKeyvault = perSwitch.UsernameKeyvault
}
if perSwitch.Password != "" {
result.Password = perSwitch.Password
}
if perSwitch.PasswordKeyvault != "" {
result.PasswordKeyvault = perSwitch.PasswordKeyvault
}
return result
}

// logAuthMerge logs how auth was resolved for a switch after global->per-switch merge.
func logAuthMerge(switchName string, perSwitch, global AuthConfig) {
if perSwitch.UsernameKeyvault != "" {
log.Printf("[config] [%s] username_keyvault: per-switch override", switchName)
} else if perSwitch.Username != "" {
log.Printf("[config] [%s] username: per-switch override", switchName)
} else if global.UsernameKeyvault != "" {
log.Printf("[config] [%s] username_keyvault: inherited from global auth", switchName)
} else if global.Username != "" {
log.Printf("[config] [%s] username: inherited from global auth", switchName)
}

if perSwitch.PasswordKeyvault != "" {
log.Printf("[config] [%s] password_keyvault: per-switch override", switchName)
} else if perSwitch.Password != "" {
log.Printf("[config] [%s] password: per-switch override", switchName)
} else if global.PasswordKeyvault != "" {
log.Printf("[config] [%s] password_keyvault: inherited from global auth", switchName)
} else if global.Password != "" {
log.Printf("[config] [%s] password: inherited from global auth", switchName)
}
}

// redactSecretURI strips query parameters from a Key Vault URI for safe logging.
func redactSecretURI(uri string) string {
parts := strings.SplitN(uri, "?", 2)
return parts[0]
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