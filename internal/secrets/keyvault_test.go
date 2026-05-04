package secrets

import (
	"testing"
)

func TestParseSecretURI(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantVault  string
		wantSecret string
		wantVer    string
		wantErr    bool
	}{
		{
			name:       "valid URI without version",
			uri:        "https://myvault.vault.azure.net/secrets/my-secret",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
			wantVer:    "",
		},
		{
			name:       "valid URI with version",
			uri:        "https://myvault.vault.azure.net/secrets/my-secret/abc123",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
			wantVer:    "abc123",
		},
		{
			name:       "trailing slash",
			uri:        "https://myvault.vault.azure.net/secrets/my-secret/",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
		},
		{
			name:    "http scheme rejected",
			uri:     "http://myvault.vault.azure.net/secrets/my-secret",
			wantErr: true,
		},
		{
			name:    "wrong host",
			uri:     "https://myvault.example.com/secrets/my-secret",
			wantErr: true,
		},
		{
			name:    "missing secret path",
			uri:     "https://myvault.vault.azure.net/keys/my-key",
			wantErr: true,
		},
		{
			name:    "empty path",
			uri:     "https://myvault.vault.azure.net/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault, secret, ver, err := parseSecretURI(tt.uri)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if vault != tt.wantVault {
				t.Errorf("vault = %q, want %q", vault, tt.wantVault)
			}
			if secret != tt.wantSecret {
				t.Errorf("secret = %q, want %q", secret, tt.wantSecret)
			}
			if ver != tt.wantVer {
				t.Errorf("version = %q, want %q", ver, tt.wantVer)
			}
		})
	}
}
