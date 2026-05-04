# TOR Switch Setup Guide

Prerequisites and configuration required on TOR switches before Network Mapper can collect topology data.

---

## Table of Contents

- [Overview](#overview)
- [Cisco NX-OS Setup](#cisco-nx-os-setup)
  - [1. Create a gNMI Service Account](#1-create-a-gnmi-service-account)
  - [2. Enable gNMI](#2-enable-gnmi)
  - [3. Enable LLDP](#3-enable-lldp)
  - [4. Verify gNMI is Operational](#4-verify-gnmi-is-operational)
- [Store Credentials in Azure Key Vault](#store-credentials-in-azure-key-vault)
- [Network Mapper Configuration](#network-mapper-configuration)
- [Connectivity Checklist](#connectivity-checklist)
- [Troubleshooting](#troubleshooting)

---

## Overview

Network Mapper connects to each TOR switch via **gNMI** (gRPC Network Management Interface) and queries:

| Data Category | NX-OS gNMI Path | Purpose |
|---|---|---|
| LLDP neighbors | `/System/lldp-items/...` | Physical link discovery |
| Interfaces | `/System/intf-items/...` | Port status, speed, counters |
| System info | `/System/` | Hostname, software version, uptime |
| MAC table | `/System/mac-items/...` | VM endpoint discovery |
| ARP table | `/System/arp-items/...` | IP-to-MAC mapping |
| VLANs | `/System/bd-items/...` | VLAN topology |
| CPU / Memory | `/System/procsys-items/...` | Resource utilization |

**Requirements summary:**

1. A gNMI service account with **network-admin** role
2. gNMI enabled on the **default VRF** (not management VRF)
3. LLDP enabled globally
4. Credentials stored in **Azure Key Vault** (two separate secrets)

---

## Cisco NX-OS Setup

> **NX-OS version:** gNMI is supported on NX-OS 9.3(1) and later. Version 10.2+ is recommended for full OpenConfig model support.

### 1. Create a gNMI Service Account

Network Mapper authenticates to gNMI using username/password passed as gRPC metadata. The account needs **network-admin** privileges to read LLDP, interface, MAC, ARP, and VLAN data.

```
! Create a dedicated service account for gNMI
username gnmi-network-mapper password <strong-password> role network-admin

! (Optional) Restrict to read-only if a custom role exists
! role name gnmi-readonly
!   rule 10 permit read
```

> **Security note:** The `network-admin` role is required because NX-OS gNMI enforces role-based access on YANG paths. A `network-operator` role cannot make requests to gNMI server.

### 2. Enable gNMI

gNMI must be enabled on the **default VRF**, not the management VRF. This is critical — the Network Mapper VM runs inside the Azure Local cluster on the data network, so it reaches the switch through the default VRF.

```
! Enable the gNMI feature
feature grpc

! Configure gNMI server on the default VRF
grpc use-vrf default
```

> **Why default VRF?** In Azure Local deployments, the Network Mapper VM sits on the cluster data network — the same network that hosts and switches share. The management VRF is typically only accessible from out-of-band management networks. By binding gNMI to the default VRF, the tool can reach the switch over the data-plane interfaces.

#### TLS Certificate (Optional)

By default, NX-OS generates a self-signed certificate for gRPC. Network Mapper supports this via the **TOFU (Trust-On-First-Use)** or **skip-verify** TLS modes. For production hardening, you can install a CA-signed certificate:

```
! Install a custom certificate (optional)
grpc certificate <cert-label>
```

For testing purposes, `skip_verify: true` or `tofu: true` in the Network Mapper config is sufficient.


### 3. Verify gNMI is Operational

After configuration, verify the gNMI server is running:

```
show feature | include grpc

! Expected output:
! grpc                   1          enabled

show grpc gnmi service statistics

! Check for active sessions and successful RPCs
```

Test gNMI connectivity from a Linux machine with `gnmic` (optional):

```bash
# Install gnmic (one-time)
# https://gnmic.openconfig.net/install/

# Test capabilities
gnmic -a <switch-ip>:50051 \
  --username <gnmi-user> \
  --password <password> \
  --insecure \
  capabilities

# Test LLDP query
gnmic -a <switch-ip>:50051 \
  --username <gnmi-user> \
  --password <password> \
  --insecure \
  get --path "/System/lldp-items/inst-items/if-items/If-list" \
  --encoding json
```

Or use Network Mapper's built-in connectivity test:

```bash
network-mapper test-connection --config config.yaml
```

---

## Store Credentials in Azure Key Vault

Switch credentials must be stored in Azure Key Vault — **never in plaintext config files**. Create two separate secrets: one for the username and one for the password.

### Create the secrets

```bash
# Set the vault name
VAULT_NAME="your-keyvault-name"

# Store the gNMI username
az keyvault secret set \
  --vault-name $VAULT_NAME \
  --name gnmi-username \
  --value "gnmi-network-mapper"

# Store the gNMI password
az keyvault secret set \
  --vault-name $VAULT_NAME \
  --name gnmi-password \
  --value "<the-strong-password-you-set-on-the-switch>"
```

## Network Mapper Configuration

Reference the Key Vault secrets in the config file using `_keyvault` suffixed fields:

```yaml
# /etc/network-mapper/config.yaml

# Global auth — applies to all switches unless overridden
auth:
  username_keyvault: https://<vault>.vault.azure.net/secrets/gnmi-username
  password_keyvault: https://<vault>.vault.azure.net/secrets/gnmi-password

switches:
  - name: TOR-1
    address: "10.0.1.1:50051"
    platform: nxos

  - name: TOR-2
    address: "10.0.1.2:50051"
    platform: nxos
    # Per-switch override (if this switch uses different credentials):
    # auth:
    #   username_keyvault: https://<vault>.vault.azure.net/secrets/tor2-user
    #   password_keyvault: https://<vault>.vault.azure.net/secrets/tor2-pass

tls:
  skip_verify: true   # Accept the switch's self-signed cert

collect:
  timeout_sec: 30
  parallel: 2
```

> **Port:** The default gNMI port on NX-OS is `50051`. Adjust if you configured a different `listen-port`.

---

## Connectivity Checklist

Run through this checklist before your first collection:

| # | Check | How to verify |
|---|---|---|
| 1 | gNMI feature enabled | `show feature \| grep grpc` → `enabled` |
| 2 | gNMI listening on correct VRF | `show grpc internal service-status` → `default` VRF, port `50051` |
| 3 | LLDP enabled | `show lldp neighbors` → neighbors visible |
| 4 | Service account exists | `show user-account <gnmi-user>` |
| 5 | Firewall allows gRPC | No ACL blocking TCP 50051 from the VM subnet |
| 6 | Key Vault secrets created | `az keyvault secret show --vault-name <vault> --name gnmi-username` |
| 7 | Network Mapper test | `network-mapper test-connection --config config.yaml` → ✓ |

---

## Troubleshooting

### gNMI connection refused

```
dial tcp 10.0.1.1:50051: connection refused
```

- Verify `feature grpc` is enabled
- Check the listening port: `show grpc internal service-status`
- Confirm the VRF — if gNMI is on `management` VRF but the VM is on the data network, change to `vrf default`

### Authentication failed

```
rpc error: code = Unauthenticated desc = ...
```

- Verify the username and password match exactly (case-sensitive)
- Check the Key Vault secret values: `az keyvault secret show --vault-name <vault> --name gnmi-username --query value`
- Verify the account has `network-admin` role: `show user-account <username>`

### LLDP data is empty

```
switch collected 0 LLDP neighbors
```

- Verify LLDP is enabled: `show feature | grep lldp` → `enabled`
- Check that neighbors are visible: `show lldp neighbors`
- Some interfaces may have LLDP disabled per-port: `show lldp interface`
- Wait 30 seconds after enabling LLDP for neighbor advertisements to propagate

### MAC / ARP table queries fail

```
rpc error: code = Internal desc = could not read MAC table
```

- These paths require `network-admin` role on some NX-OS versions
- Verify the account role: `show user-account <username>`
- Upgrade the role from `network-operator` to `network-admin` if needed

### TLS handshake errors

```
transport: authentication handshake failed: tls: ...
```

- Use `skip_verify: true` in the Network Mapper config for self-signed certs
- Or use `tofu: true` to trust the cert on first connection and pin it for subsequent connections
- For production hardening, export the switch certificate and set `ca_cert:` in the config

### Key Vault access denied

```
keyvault: failed to fetch secret: 403 Forbidden
```

- Verify the identity has the **Key Vault Secrets User** role on the vault
- For Arc VMs, verify the managed identity is enabled: `azcmagent show`
- For dev machines, verify you're logged in: `az login` → `az account show`
