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

Example config with Key Vault:

```yaml
switches:
  - address: "10.0.0.1:50051"
    name: tor1
    platform: nxos
    auth:
      username: admin
      password_keyvault: "https://myvault.vault.azure.net/secrets/tor1-password"
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

## Azure Managed Identity (for Key Vault)

If the VM has a **managed identity** assigned, `DefaultAzureCredential` will
automatically use it to access Key Vault — no client secrets needed.

1. **Assign managed identity** to the VM in Azure Portal → VM → Identity → System assigned → On
2. **Grant Key Vault access**: Key Vault → Access policies → Add → Select the VM's identity → Secret permissions: Get
3. The network-mapper service will automatically authenticate via managed identity

For **service principal** auth instead, set environment variables in the service file:

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
