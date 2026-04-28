// Package secrets provides credential resolution from Azure Key Vault.
package secrets

import (
"context"
"fmt"
"log"
"net/url"
"strings"
"sync"
"time"

"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// Resolver fetches secrets from Azure Key Vault using DefaultAzureCredential.
// It caches clients per vault URL to avoid creating multiple connections.
type Resolver struct {
mu      sync.Mutex
clients map[string]*azsecrets.Client
cred    *azidentity.DefaultAzureCredential
}

// NewResolver creates a new Key Vault resolver.
// It initializes DefaultAzureCredential which supports managed identity,
// az CLI login, environment variables, and other Azure auth methods.
func NewResolver() (*Resolver, error) {
log.Printf("[keyvault] Initializing DefaultAzureCredential (auth chain: env vars -> workload identity -> managed identity -> Azure CLI)")
cred, err := azidentity.NewDefaultAzureCredential(nil)
if err != nil {
log.Printf("[keyvault] ERROR: Failed to initialize Azure credential: %v", err)
log.Printf("[keyvault] Ensure you are logged in (az login) or running on an Azure/Arc-enabled VM with managed identity")
return nil, fmt.Errorf("failed to create Azure credential: %w", err)
}
log.Printf("[keyvault] Azure credential initialized successfully")

return &Resolver{
clients: make(map[string]*azsecrets.Client),
cred:    cred,
}, nil
}

// Resolve fetches a secret from Azure Key Vault given a secret URI.
// The URI format is: https://<vault-name>.vault.azure.net/secrets/<secret-name>
// An optional version can be appended: https://<vault-name>.vault.azure.net/secrets/<secret-name>/<version>
func (r *Resolver) Resolve(ctx context.Context, secretURI string) (string, error) {
log.Printf("[keyvault] Parsing secret URI: %s", redactURI(secretURI))

vaultURL, secretName, version, err := parseSecretURI(secretURI)
if err != nil {
log.Printf("[keyvault] ERROR: Invalid secret URI %q: %v", redactURI(secretURI), err)
return "", err
}
log.Printf("[keyvault] Parsed URI - vault: %s, secret: %q, version: %s", vaultURL, secretName, versionLabel(version))

client, err := r.getClient(vaultURL)
if err != nil {
log.Printf("[keyvault] ERROR: Failed to create client for vault %s: %v", vaultURL, err)
return "", fmt.Errorf("failed to create Key Vault client for %s: %w", vaultURL, err)
}

log.Printf("[keyvault] Fetching secret %q from %s ...", secretName, vaultURL)
start := time.Now()

resp, err := client.GetSecret(ctx, secretName, version, nil)
elapsed := time.Since(start)
if err != nil {
log.Printf("[keyvault] ERROR: Failed to fetch secret %q from %s after %v: %v", secretName, vaultURL, elapsed, err)
log.Printf("[keyvault] Verify the identity has 'Key Vault Secrets User' role or Get secret permission on %s", vaultURL)
return "", fmt.Errorf("failed to get secret %q from %s: %w", secretName, vaultURL, err)
}

if resp.Value == nil {
log.Printf("[keyvault] ERROR: Secret %q from %s returned nil value (secret may be disabled)", secretName, vaultURL)
return "", fmt.Errorf("secret %q from %s has nil value", secretName, vaultURL)
}

log.Printf("[keyvault] Successfully fetched secret %q from %s (%d chars, %v)", secretName, vaultURL, len(*resp.Value), elapsed)
return *resp.Value, nil
}

func (r *Resolver) getClient(vaultURL string) (*azsecrets.Client, error) {
r.mu.Lock()
defer r.mu.Unlock()

if client, ok := r.clients[vaultURL]; ok {
log.Printf("[keyvault] Reusing cached client for %s", vaultURL)
return client, nil
}

log.Printf("[keyvault] Creating new client for vault %s", vaultURL)
client, err := azsecrets.NewClient(vaultURL, r.cred, nil)
if err != nil {
return nil, err
}

r.clients[vaultURL] = client
log.Printf("[keyvault] Client created and cached for %s", vaultURL)
return client, nil
}

// parseSecretURI parses a Key Vault secret URI into vault URL, secret name, and version.
func parseSecretURI(uri string) (vaultURL, secretName, version string, err error) {
u, err := url.Parse(uri)
if err != nil {
return "", "", "", fmt.Errorf("invalid secret URI: %w", err)
}

if u.Scheme != "https" {
return "", "", "", fmt.Errorf("secret URI must use https scheme: %s", uri)
}

if !strings.HasSuffix(u.Host, ".vault.azure.net") {
return "", "", "", fmt.Errorf("secret URI must be a Key Vault URL (*.vault.azure.net): %s", uri)
}

vaultURL = fmt.Sprintf("https://%s", u.Host)

// Path should be /secrets/<name> or /secrets/<name>/<version>
parts := strings.Split(strings.Trim(u.Path, "/"), "/")
if len(parts) < 2 || parts[0] != "secrets" {
return "", "", "", fmt.Errorf("secret URI path must be /secrets/<name>[/<version>]: %s", uri)
}

secretName = parts[1]
if len(parts) >= 3 {
version = parts[2]
}

return vaultURL, secretName, version, nil
}

// redactURI strips query parameters and fragments from a URI for safe logging.
func redactURI(uri string) string {
u, err := url.Parse(uri)
if err != nil {
return "<invalid-uri>"
}
u.RawQuery = ""
u.Fragment = ""
return u.String()
}

// versionLabel returns a human-readable version label for logging.
func versionLabel(version string) string {
if version == "" {
return "latest"
}
return version
}