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
  Stage 1: Collect               Stage 2: Build               Stage 3: Render
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  gNMI Collector  │         │ Topology Builder │         │   Web UI / CLI  │
│                  │ ──────► │                  │ ──────► │                 │
│ Raw switch data  │ Result  │ V2 hierarchical  │  JSON   │ Interactive     │
│ per switch       │         │ topology JSON    │         │ visualization   │
└─────────────────┘         └─────────────────┘         └─────────────────┘
     ▲                                                        ▲
     │ gNMI Get/Subscribe                                     │ HTTP + WS
┌─────────────┐                                          ┌──────────┐
│ TOR Switches │                                          │ Browser  │
│ (SONiC/NX-OS)│                                          └──────────┘
└─────────────┘
```

The project is organized as a 3-stage data pipeline. Each stage is independently testable and mockable:

1. **gNMI Collector** (`internal/collector/`) — Connects to TOR switches via gNMI, collects raw LLDP, interface, system, VLAN, MAC, ARP, and BGP data. Returns a `CollectionResult` with per-switch raw data.
2. **Topology Builder** (`internal/builder/`) — Pure function that transforms the raw `CollectionResult` into a hierarchical v2 topology JSON. Handles device classification, link correlation, VLAN cross-referencing, and endpoint attribution. No I/O, no side effects.
3. **Network Mapper UI** (`cmd/network-mapper/web/`) — Embedded web UI that consumes the v2 topology JSON and renders an interactive visualization with fabric, switch, host, and VM views.

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
    platform: sonic            # sonic | nxos

  - name: TOR-2
    address: "10.0.0.2:50051"
    platform: sonic
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

# Storage & retention (for live mode with --config)
storage:
  data_dir: "./data"           # Base directory for snapshots + logs
  retention_days: 7            # Delete snapshots/logs older than this
  max_snapshots: 1000          # Hard cap on snapshot count (safety valve)
  log_to_file: true            # Enable file-based logging with rotation
  log_max_size_mb: 50          # Max single log file size before rotation
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

# Serve the interactive web UI (static mode — serves a single JSON file)
network-mapper serve --topology topology.json --port 8080

# Serve with live collection (hybrid mode — ON_CHANGE + periodic polling)
network-mapper serve --config config.yaml --port 8080 --interval 300

# Flags
network-mapper collect --help
network-mapper serve --help
```

## Historical Snapshots & Retention

When running in live mode (`serve --config`), the tool automatically:

- **Saves topology snapshots** to `data/snapshots/` whenever the network topology changes
- **Subscribes to gNMI ON_CHANGE** for LLDP and interface state — changes are detected in near real-time
- **Polls periodically** (default 5 min) as fallback for data that doesn't support ON_CHANGE
- **Prunes old data** — snapshots and log files older than `retention_days` are automatically deleted

The web UI displays a **timeline slider** when snapshots are available, letting you browse historical topology states and compare them with the current live view.

### Data Directory Structure

```
data/
├── snapshots/
│   ├── topology-2026-05-11T10-30-00Z.json
│   ├── topology-2026-05-11T11-45-22Z.json
│   └── ...
└── logs/
    ├── network-mapper-2026-05-11.log
    └── ...
```

## Data Collected

The tool queries each TOR switch for 3 categories of data via gNMI:

| Category | OpenConfig Path | Data |
|---|---|---|
| LLDP Neighbors | `/openconfig-lldp:lldp/interfaces/interface/neighbors` | Chassis ID, port ID, system name, management IP |
| Interface State | `/openconfig-interfaces:interfaces/interface` | Oper status, speed, MTU, traffic counters |
| System Info | `/openconfig-system:system/state` | Hostname, software version, uptime |

For Cisco NX-OS switches, native paths are used: `/System/lldp-items/inst-items/if-items/If-list`

## Topology JSON Output (v2 Schema)

The topology JSON uses a hierarchical v2 schema designed to be both machine-processable and human-readable. It serves as the primary artifact of the tool — a complete summary of the physical network topology.

```json
{
  "schema_version": "2.0",
  "metadata": {
    "collected_at": "2026-05-18T10:30:00Z",
    "source_switches": ["TOR-1", "TOR-2"],
    "collector_version": "0.4.0"
  },
  "summary": {
    "total_switches": 2,
    "total_hosts": 4,
    "total_peer_links": 8,
    "total_host_links": 16,
    "total_vlans": 3,
    "total_unknown_devices": 0
  },
  "fabric": {
    "switches": [
      {
        "id": "TOR-1",
        "name": "TOR-1",
        "chassis_id": "aa:bb:cc:dd:ee:01",
        "management_address": "10.0.0.1",
        "software_version": "SONiC.4.1.5",
        "interfaces": [ ... ],
        "peer_links": [ ... ],
        "connected_hosts": [ ... ],
        "bgp_sessions": [ ... ]
      }
    ]
  },
  "compute": {
    "hosts": [
      {
        "id": "host-01",
        "name": "ASRR1N42R14U01",
        "connections": [ ... ],
        "endpoints": [ ... ]
      }
    ]
  },
  "vlans": { "items": [ ... ] },
  "warnings": []
}
```

For the full schema reference, see [`docs/topology-schema.md`](docs/topology-schema.md).

Use `--schema v1` with the `collect` command to output the legacy flat schema if needed.

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

| Vendor | Platform | LLDP Path | Encoding |
|---|---|---|---|
| SONiC (Dell/MS) | Enterprise SONiC | OpenConfig `/openconfig-lldp:lldp/...` | JSON_IETF |
| Cisco | NX-OS | Native `/System/lldp-items/...` | JSON |

The tool automatically handles:
- **SONiC Get→Subscribe fallback** — SONiC returns empty for bulk Get on list paths; the tool falls back to Subscribe ONCE
- **JSON_IETF prefix stripping** — removes `module-name:` prefixes from response keys (RFC 7951)
- **Interface name normalization** — `eth1/1` → `Eth1/1`, `Ethernet0` unchanged

## Device Identification

The tool classifies every discovered device into one of five types. Classification uses only data obtained from gNMI — no external database is required.

| Type | Meaning | How identified |
|------|---------|---------------|
| **switch** | Network switch (TOR, spine, leaf) | LLDP capabilities (Bridge/Router), or system description keywords: SONiC, NX-OS, Arista, Cumulus, FTOS, Dell EMC, Cisco |
| **host** | Physical server | LLDP capabilities (Station only), or system description keywords: Linux, Ubuntu, Windows, Red Hat, CentOS, SLES. Also promoted from `unknown` via ARP enrichment or deployment JSON matching |
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
| 4 | Deployment hostname (enrichment) | `ASRR1N42R14U01` | Replaces MAC-based IDs after deployment matching |

For full details on device correlation, MAC offset handling, and enrichment passes, see [`docs/DEVICE-CORRELATION.md`](docs/DEVICE-CORRELATION.md).

## Deployment JSON Enrichment (Experimental)

> ⚠️ **This feature is experimental.** It has been tested against a single Azure Local deployment layout. The deployment JSON schema may vary across versions and regions. Use this feature for additional context, but do not rely on it as the sole source of truth.

When provided with a deployment plan JSON file (`--deployment` flag), the tool can enrich the topology with authoritative host metadata:

- **MAC matching** — correlates LLDP chassis-ids to deployment NIC MACs (exact and +2 offset)
- **NIC port grouping** — merges multi-NIC-port devices into single host nodes
- **ID rename** — replaces MAC-based IDs with deployment hostnames
- **Missing host synthesis** — adds expected hosts that were not discovered via LLDP

The deployment JSON is read from files produced by Azure Local deployment tooling. See [`docs/DEVICE-CORRELATION.md`](docs/DEVICE-CORRELATION.md) for the expected schema and matching algorithm.

## Architecture

```
network-mapper/
├── cmd/network-mapper/
│   ├── main.go                # CLI entry (cobra): collect + serve commands
│   └── web/                   # Embedded web UI (go:embed)
│       ├── index.html
│       ├── css/app.css        # Azure portal-inspired theme (light + dark)
│       ├── js/
│       │   ├── app.js         # Entry point, v2 adapter, WebSocket
│       │   ├── graph.js       # Cytoscape.js init, layouts, expand/collapse
│       │   ├── data/topology.js # V2→V1 adapter + data helpers
│       │   ├── views/         # Fabric, switch, host, VM views
│       │   └── ui/            # Sidebar, popup, toolbar, timeline, inventory
│       └── lib/               # Vendored: cytoscape.min.js, dagre.min.js
├── internal/
│   ├── config/                # YAML config loading + env-var resolution
│   ├── gnmi/                  # gNMI client, TLS/TOFU, path parsing
│   ├── transform/             # LLDP, interface, system data parsers
│   ├── collector/             # Stage 1: gNMI collection → CollectionResult
│   ├── builder/               # Stage 2: CollectionResult → TopologyV2
│   ├── topology/              # Core types: v1 (Topology) + v2 (TopologyV2)
│   ├── deployment/            # Deployment JSON enrichment (experimental)
│   ├── secrets/               # Azure Key Vault credential resolution
│   ├── server/                # HTTP server: embedded web + REST API
│   └── storage/               # Snapshot persistence + retention pruning
├── docs/
│   ├── topology-schema.md     # V2 schema reference documentation
│   ├── DATA-COLLECTION.md     # gNMI paths and data categories
│   ├── DEVICE-CORRELATION.md  # Dedup, MAC matching, enrichment
│   └── SWITCH-SETUP.md        # gNMI setup on TOR switches
└── examples/
    ├── config.yaml            # Sample config for 2 TOR switches
    └── topology-v2-sample.json # Sample v2 output from real switches
```

## Related Projects

This project builds on patterns from [arc-switch](../arc-switch), specifically:
- gNMI client with gRPC metadata auth and 64MB max message size
- TLS/TOFU certificate bootstrapping and caching
- Subscribe ONCE fallback for SONiC list path quirks
- OpenConfig + NX-OS native LLDP response transformers
- JSON_IETF module prefix stripping (RFC 7951)

## Documentation

| Document | Description |
|----------|-------------|
| [topology-schema.md](docs/topology-schema.md) | V2 topology JSON schema — field-by-field reference |
| [DATA-COLLECTION.md](docs/DATA-COLLECTION.md) | What data is collected, gNMI paths, how each category is used |
| [DEVICE-CORRELATION.md](docs/DEVICE-CORRELATION.md) | Device deduplication, MAC offset handling, enrichment passes |
| [SWITCH-SETUP.md](docs/SWITCH-SETUP.md) | How to enable gNMI on TOR switches |

## Roadmap

- [x] v0.1 — CLI `collect` command with JSON topology output
- [x] v0.2 — Embedded web UI with interactive graph visualization
- [x] v0.3 — Historical snapshots with timeline UI and retention policy
- [x] v0.4 — V2 hierarchical topology schema with 3-stage pipeline
- [ ] v0.5 — Topology diff / drift detection
- [ ] v1.0 — Production hardening, comprehensive tests, CI/CD




## License

MIT

# Demo

https://github.com/user-attachments/assets/5892ba64-68d3-4ea7-93a3-a4b0e99845b2

