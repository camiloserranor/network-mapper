// Package secrets provides credential resolution from Azure Key Vault.
package secrets

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"

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
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	return &Resolver{
		clients: make(map[string]*azsecrets.Client),
		cred:    cred,
	}, nil
}

// Resolve fetches a secret from Azure Key Vault given a secret URI.
// The URI format is: https://<vault-name>.vault.azure.net/secrets/<secret-name>
// An optional version can be appended: https://<vault-name>.vault.azure.net/secrets/<secret-name>/<version>
func (r *Resolver) Resolve(ctx context.Context, secretURI string) (string, error) {
	vaultURL, secretName, version, err := parseSecretURI(secretURI)
	if err != nil {
		return "", err
	}

	client, err := r.getClient(vaultURL)
	if err != nil {
		return "", fmt.Errorf("failed to create Key Vault client for %s: %w", vaultURL, err)
	}

	log.Printf("[keyvault] Fetching secret %q from %s", secretName, vaultURL)

	resp, err := client.GetSecret(ctx, secretName, version, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %q from %s: %w", secretName, vaultURL, err)
	}

	if resp.Value == nil {
		return "", fmt.Errorf("secret %q from %s has nil value", secretName, vaultURL)
	}

	return *resp.Value, nil
}

func (r *Resolver) getClient(vaultURL string) (*azsecrets.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if client, ok := r.clients[vaultURL]; ok {
		return client, nil
	}

	client, err := azsecrets.NewClient(vaultURL, r.cred, nil)
	if err != nil {
		return nil, err
	}

	r.clients[vaultURL] = client
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
