# Network Mapper

**Physical topology discovery and visualization for Azure Local deployments using gNMI and LLDP.**

Network Mapper connects to Top-of-Rack (TOR) switches via [gNMI](https://github.com/openconfig/gnmi), retrieves LLDP neighbor data along with interface state and system information, and builds an interactive physical topology map. The tool ships as a single Go binary with an embedded web UI — no external dependencies required.

## Why

Azure Local deployments rely on physical cabling between hosts and TOR switches. Understanding this topology is critical for:

- **Deployment validation** — verify hosts are cabled to the correct switch ports
- **Troubleshooting** — quickly identify which switch port a failing NIC is connected to
- **Inventory** — maintain an up-to-date map of physical connectivity with interface health
- **Drift detection** — compare current topology against a known-good baseline

## How It Works

```
┌─────────────┐     gNMI Get/Subscribe       ┌──────────────┐
│  TOR Switch  │ ◄────────────────────────── │              │
│  (NX-OS)     │  LLDP · Interfaces · Sys    │   Network    │
└─────────────┘                              │   Mapper     │ ──► topology.json
                                             │              │
┌─────────────┐     gNMI Get/Subscribe       │              │ ──► Web UI (localhost)
│  TOR Switch  │ ◄────────────────────────── │              │
│  (NX-OS)     │  LLDP · Interfaces · Sys    │              │
└─────────────┘                              └──────────────┘
```

1. **Connect** to each TOR switch via gNMI (gRPC + TLS)
2. **Query** LLDP neighbor tables, interface state/counters, and system info
3. **Normalize** data across vendor-specific gNMI paths into a unified topology model
4. **Build** a topology graph with devices, interfaces, and physical links
5. **Export** the topology as a versioned JSON document
6. **Visualize** the topology in an interactive web UI with hierarchical layout

## Quick Start

```bash
# Build from source
go build -o network-mapper ./cmd/network-mapper/

# Authenticate (dev/test — production uses Arc managed identity)
az login

# Collect topology from TOR switches
network-mapper collect --config examples/config.yaml --output topology.json

# Launch the interactive web UI
network-mapper serve --topology topology.json --port 8080
```

## Configuration

Network Mapper uses a YAML configuration file. Switch credentials are stored in Azure Key Vault — **never in plaintext config files**.

```yaml
# config.yaml

# Global auth — applies to all switches unless overridden per-switch.
auth:
  username_keyvault: https://myvault.vault.azure.net/secrets/gnmi-username
  password_keyvault: https://myvault.vault.azure.net/secrets/gnmi-password

switches:
  - name: TOR-1
    address: "10.0.0.1:50051"
    platform: nxos             # nxos | sonic | dell-os10

  - name: TOR-2
    address: "10.0.0.2:50051"
    platform: nxos
    # Per-switch override (optional):
    # auth:
    #   username_keyvault: https://myvault.vault.azure.net/secrets/tor2-user
    #   password_keyvault: https://myvault.vault.azure.net/secrets/tor2-pass

tls:
  skip_verify: true            # Or use tofu/ca_cert for production
  # tofu: true                 # Trust-On-First-Use cert pinning
  # cert_dir: ".certs"
  # ca_cert: /path/to/ca.pem  # Explicit CA trust

collect:
  timeout_sec: 30              # Per-switch timeout
  parallel: 2                  # Max concurrent switch connections
  skip_counters: false         # Skip interface counter collection
```

## Authentication & Credentials

Network Mapper uses [Azure Identity `DefaultAzureCredential`](https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication) to authenticate to Key Vault. No client secrets or tokens are stored — the tool relies on the ambient identity of the environment.

### Production (Azure Local VM)

In production, network-mapper runs on a VM inside the Azure Local cluster. These VMs are Azure Arc-enabled, which means they have a **managed identity** automatically available:

1. **Enable system-assigned managed identity** on the Arc-enabled VM (usually enabled by default)
2. **Grant the identity access** to the Key Vault:
   ```bash
   az keyvault set-policy --name myvault \
     --object-id <vm-managed-identity-object-id> \
     --secret-permissions get
   ```
   Or assign the **Key Vault Secrets User** RBAC role on the vault.
3. **Run network-mapper** — `DefaultAzureCredential` detects Arc managed identity automatically. No extra config needed.

### Development / Testing

On a developer workstation, authenticate using the Azure CLI:

```bash
az login
# That's it — DefaultAzureCredential picks up the az CLI token.
network-mapper collect --config config.yaml --output topology.json
```

The credential chain tried by `DefaultAzureCredential` (in order):
1. Environment variables (`AZURE_CLIENT_ID` / `AZURE_CLIENT_SECRET` / `AZURE_TENANT_ID`)
2. Workload Identity (Kubernetes)
3. Managed Identity (Azure VM / Arc-enabled VM)
4. Azure CLI (`az login`)
5. Azure PowerShell (`Connect-AzAccount`)

### Key Vault Setup

Store the gNMI credentials as two separate secrets:

```bash
az keyvault secret set --vault-name myvault --name gnmi-username --value "admin"
az keyvault secret set --vault-name myvault --name gnmi-password --value "your-secure-password"
```

Then reference them in config:
```yaml
auth:
  username_keyvault: https://myvault.vault.azure.net/secrets/gnmi-username
  password_keyvault: https://myvault.vault.azure.net/secrets/gnmi-password
```

## CLI Commands

```bash
# Collect topology from configured switches
network-mapper collect --config config.yaml --output topology.json

# Serve the interactive web UI
network-mapper serve --topology topology.json --port 8080

# Flags
network-mapper collect --help
network-mapper serve --help
```

## Data Collected

The tool queries each TOR switch for 3 categories of data via gNMI:

| Category | OpenConfig Path | Data |
|---|---|---|
| LLDP Neighbors | `/openconfig-lldp:lldp/interfaces/interface/neighbors` | Chassis ID, port ID, system name, management IP |
| Interface State | `/openconfig-interfaces:interfaces/interface` | Oper status, speed, MTU, traffic counters |
| System Info | `/openconfig-system:system/state` | Hostname, software version, uptime |

For Cisco NX-OS switches, native paths are used: `/System/lldp-items/inst-items/if-items/If-list`

## Topology JSON Output

```json
{
  "schema_version": "1.0",
  "collected_at": "2026-04-24T12:00:00Z",
  "source_switches": ["TOR-1", "TOR-2"],
  "partial_failures": [],
  "devices": [
    {
      "id": "TOR-1",
      "type": "switch",
      "system_name": "TOR-1",
      "software_version": "SONiC.4.1.5",
      "management_address": "10.0.0.1",
      "interfaces": [
        { "name": "Ethernet1", "oper_status": "UP", "speed": "25G", "mtu": 9100 }
      ]
    }
  ],
  "links": [
    {
      "local_device": "TOR-1",
      "local_port": "Ethernet1",
      "remote_device": "host-01",
      "remote_port": "NIC1",
      "remote_chassis_id": "11:22:33:44:55:01",
      "source": "lldp",
      "oper_status": "UP",
      "speed": "25G",
      "counters": { "in_octets": 1234567890, "out_octets": 987654321 }
    }
  ]
}
```

## Web UI Features

The embedded web UI provides an interactive topology visualization (Azure portal-inspired dark theme):

- **Tree-based exploration** — initially shows only switches; double-click to expand hosts, then VMs
- **Spine/leaf hierarchy** — spine switches appear at the top, leaf switches below, then hosts, then VMs
- **Hierarchical layout** — tree-aware positioning that avoids node overlaps across subtrees
- **Force-directed layout** — alternative physics-based layout
- **Inventory panel** — collapsible left sidebar listing all discovered physical devices with expand/details buttons
- **VLAN summary view** — groups nodes by VLAN, showing device counts per VLAN instead of individual nodes
- **Hover interactions** — highlights connected neighbors, dims unrelated nodes, shows port labels
- **Click popup card** — floating card with key device/link info near the clicked element
- **Full detail sidebar** — interface list with health indicators, traffic counters, connection list
- **Tooltips** — hover over detail fields for explanations (e.g., what "Source Device" means in LLDP context)
- **Search** — find devices by name, ID, or chassis ID
- **DOWN link highlighting** — red dashed lines for operationally down connections
- **Dual export** — download the topology as PNG image or JSON data
- **Live updates** — WebSocket connection refreshes topology data without page reload
- **Dark theme** — Azure portal-inspired dark UI optimized for NOC environments

## Supported Platforms

| Vendor | Platform Value | Status | Encoding |
|---|---|---|---|
| Cisco | `nxos` | **Tested** | JSON |
| Dell OS10 | `dell-os10` | Experimental | JSON_IETF |

The architecture supports any OpenConfig-compatible platform via the `platform` config field. See [`docs/DATA-COLLECTION.md`](docs/DATA-COLLECTION.md) for details on the multi-vendor collection pipeline.

The tool automatically handles:
- **Vendor-specific gNMI paths** — NX-OS native paths vs. OpenConfig paths for other platforms
- **JSON_IETF prefix stripping** — removes `module-name:` prefixes from response keys (RFC 7951)
- **Interface name normalization** — `eth1/1` → `Eth1/1`, `Ethernet0` unchanged

## Device Identification

The tool classifies every discovered device into one of five types. Classification uses only data obtained from gNMI — no external database is required.

| Type | Meaning | How identified |
|------|---------|---------------|
| **switch** | Network switch (TOR, spine, leaf) | LLDP capabilities (Bridge/Router), or system description keywords: SONiC, NX-OS, Arista, Cumulus, FTOS, Dell EMC, Cisco |
| **host** | Physical server | LLDP capabilities (Station only), or system description keywords: Linux, Ubuntu, Windows, Red Hat, CentOS, SLES. Also promoted from `unknown` via ARP enrichment |
| **bmc** | Baseboard Management Controller | Name or description contains: iDRAC, iLO, BMC, IPMI, Redfish |
| **vm** | Virtual machine / endpoint | MAC address learned on a switch port that does NOT match (or nearly match) the LLDP chassis-id of the neighbor on that port |
| **unknown** | Unclassifiable device | No LLDP capabilities and no matching keywords. Common for bare-metal NICs on NX-OS (empty system-name/capabilities) |

### Spine vs. Leaf Switch Classification

The web UI further classifies switches for layout purposes:

- **Spine** — a switch whose LLDP neighbors are exclusively other switches (no host/BMC connections)
- **Leaf** — a switch with at least one non-switch neighbor (host, BMC, or unknown device)

This classification is computed client-side from the link data and used only for visual hierarchy (spine at top, leaf below). It is not stored in the topology JSON.

### Device Identity Priority

| Priority | Source | Example | When used |
|----------|--------|---------|-----------|
| 1 | Config name (queried switches) | `TOR-1` | Always — user-assigned name from `config.yaml` |
| 2 | LLDP system-name (neighbors) | `rr1-n42-r14-93180hl-8-1a` | When the neighbor reports a hostname |
| 3 | LLDP chassis-id (neighbors) | `d8:94:24:f2:cf:b4` | Fallback when system-name is empty |

For full details on device correlation and MAC offset handling, see [`docs/DEVICE-CORRELATION.md`](docs/DEVICE-CORRELATION.md).

## Architecture

```
network-mapper/
├── cmd/network-mapper/
│   ├── main.go                # CLI entry (cobra): collect + serve commands
│   └── web/                   # Embedded web UI (go:embed)
│       ├── index.html
│       ├── css/app.css        # Dark theme
│       ├── js/
│       │   ├── graph.js       # Cytoscape.js init, tree layout, expand/collapse, VLAN view
│       │   ├── sidebar.js     # Detail panel with interface health and tooltips
│       │   ├── popup.js       # Floating card near clicked elements
│       │   ├── toolbar.js     # Layout, search, export (PNG + JSON) controls
│       │   ├── live.js        # WebSocket connection for live topology updates
│       │   └── app.js         # Main entry, topology transform, inventory panel
│       └── lib/               # Vendored: cytoscape.min.js, dagre.min.js
├── internal/
│   ├── config/                # YAML config loading + env-var resolution
│   ├── gnmi/                  # gNMI client, TLS/TOFU, path parsing
│   ├── transform/             # LLDP, interface, system data parsers
│   ├── collector/             # Orchestrator: connect, collect, build topology
│   ├── topology/              # Core types: Device, Interface, Link, Topology
│   ├── secrets/               # Azure Key Vault credential resolution
│   └── server/                # HTTP server: embedded web + REST API
└── examples/
    ├── config.yaml            # Sample config (NX-OS switches)
    ├── config-sonic.yaml      # Sample config (OpenConfig platforms)
    └── sample-topology.json   # Sample topology output
```

## Related Projects

This project builds on patterns from [arc-switch](../arc-switch), specifically:
- gNMI client with gRPC metadata auth and 64MB max message size
- TLS/TOFU certificate bootstrapping and caching
- Subscribe ONCE fallback for OpenConfig list path quirks
- Multi-vendor LLDP response transformers (NX-OS native + OpenConfig)
- JSON_IETF module prefix stripping (RFC 7951)

## Roadmap

- [x] v0.1 — CLI `collect` command with JSON topology output
- [x] v0.2 — Embedded web UI with interactive graph visualization
- [ ] v0.3 — Topology diff / drift detection



- [ ] v0.4 — Real-time streaming / auto-refresh
- [ ] v1.0 — Production hardening, comprehensive tests, CI/CD




## License

MIT

# Demo

https://github.com/user-attachments/assets/5892ba64-68d3-4ea7-93a3-a4b0e99845b2

