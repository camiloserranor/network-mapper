# VM Deployment Guide

## Prerequisites

- Linux VM (Ubuntu 22.04+ or RHEL 8+ recommended)
- Network access to TOR switches on gNMI port (typically 50051)
- Go 1.23+ (only if building from source)

## Option A: Install Pre-built Binary

```bash
# Copy binary to the VM
scp network-mapper user@vm:/tmp/

# Install
sudo cp /tmp/network-mapper /usr/local/bin/
sudo chmod +x /usr/local/bin/network-mapper

# Create service user
sudo useradd -r -s /usr/sbin/nologin network-mapper
sudo mkdir -p /etc/network-mapper /var/lib/network-mapper
sudo chown network-mapper:network-mapper /var/lib/network-mapper
```

## Option B: Build from Source on VM

```bash
git clone https://github.com/camiloserranor/network-mapper.git
cd network-mapper
go build -o /usr/local/bin/network-mapper ./cmd/network-mapper/
```

## Configuration

```bash
# Copy and edit config
sudo cp examples/config.yaml /etc/network-mapper/config.yaml
sudo nano /etc/network-mapper/config.yaml
```

Example config with Key Vault credentials:

```yaml
auth:
  username_keyvault: "https://myvault.vault.azure.net/secrets/gnmi-username"
  password_keyvault: "https://myvault.vault.azure.net/secrets/gnmi-password"

switches:
  - address: "10.0.0.1:50051"
    name: tor1
    platform: sonic
  - address: "10.0.0.2:50051"
    name: tor2
    platform: sonic

tls:
  skip_verify: true

collect:
  timeout_sec: 30
  parallel: 2
```

## Install Systemd Service

```bash
sudo cp deploy/network-mapper.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable network-mapper
sudo systemctl start network-mapper

# Check status
sudo systemctl status network-mapper
sudo journalctl -u network-mapper -f
```

## Azure Arc Managed Identity (for Key Vault)

In Azure Local deployments, VMs are **Azure Arc-enabled** and automatically get a managed identity.
`DefaultAzureCredential` detects this identity — no client secrets or extra config needed.

### Setup

1. **Verify Arc enrollment** — the VM should already be Arc-enabled as part of the Azure Local cluster:
   ```bash
   azcmagent show   # Should show "Connected" status
   ```

2. **Grant Key Vault access** to the VM's managed identity:
   ```bash
   # Get the VM's managed identity object ID
   OBJECT_ID=$(az connectedmachine show --name <vm-name> --resource-group <rg> --query identity.principalId -o tsv)

   # Grant secret read access
   az keyvault set-policy --name myvault \
     --object-id $OBJECT_ID \
     --secret-permissions get
   ```
   Or use RBAC: assign **Key Vault Secrets User** role on the vault to the VM identity.

3. **Run the service** — credentials are resolved automatically at startup.

### Development / Testing (without Arc)

On a developer machine, `az login` provides the credential:

```bash
az login
network-mapper collect --config /etc/network-mapper/config.yaml --output topology.json
```

### Service Principal (alternative)

For environments without managed identity or `az login`, set environment variables in the service file:

```ini
# In /etc/systemd/system/network-mapper.service [Service] section:
Environment=AZURE_TENANT_ID=your-tenant-id
Environment=AZURE_CLIENT_ID=your-client-id
Environment=AZURE_CLIENT_SECRET=your-client-secret
```

## Firewall Rules

Ensure the VM can reach:
- TOR switches on gNMI port (default: 50051)
- Azure Key Vault (`*.vault.azure.net:443`) if using KV credentials
- Azure Arc endpoints (required for managed identity token refresh)
- Clients on port 8080 (web UI)

## Docker on VM (Alternative)

```bash
# Build and run with Docker
docker compose up -d

# Or run directly
docker run -d \
  -p 8080:8080 \
  -v /etc/network-mapper/config.yaml:/etc/network-mapper/config.yaml:ro \
  network-mapper
```
