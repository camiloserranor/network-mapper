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
│ (SONiC/NX-OS)│  LLDP · Interfaces · Sys    │   Network    │
└─────────────┘                              │   Mapper     │ ──► topology.json
                                             │              │
┌─────────────┐     gNMI Get/Subscribe       │              │ ──► Web UI (localhost)
│  TOR Switch  │ ◄────────────────────────── │              │
│ (SONiC/NX-OS)│  LLDP · Interfaces · Sys    │              │
└─────────────┘                              └──────────────┘
```

1. **Connect** to each TOR switch via gNMI (gRPC + TLS)
2. **Query** LLDP neighbor tables, interface state/counters, and system info
3. **Normalize** data across vendor variations (SONiC OpenConfig, Cisco NX-OS native)
4. **Build** a topology graph with devices, interfaces, and physical links
5. **Export** the topology as a versioned JSON document
6. **Visualize** the topology in an interactive web UI with hierarchical layout

## Quick Start

```bash
# Build from source
go build -o network-mapper ./cmd/network-mapper/

# Collect topology from TOR switches
export SWITCH_PASSWORD="your-password"
network-mapper collect --config examples/config.yaml --output topology.json

# Launch the interactive web UI
network-mapper serve --topology topology.json --port 8080
```

## Configuration

Network Mapper uses a YAML configuration file. Credentials support `${ENV_VAR}` syntax for secure password handling.

```yaml
# config.yaml
switches:
  - name: TOR-1
    address: "10.0.0.1:8080"
    platform: sonic            # sonic | nxos
    auth:
      username: admin
      password: "${SWITCH_PASSWORD}"

  - name: TOR-2
    address: "10.0.0.2:8080"
    platform: sonic
    auth:
      username: admin
      password: "${SWITCH_PASSWORD}"

tls:
  tofu: true                   # Trust-On-First-Use cert pinning
  cert_dir: ".certs"           # Directory to cache switch certificates
  # skip_verify: true          # Skip all TLS verification
  # ca_cert: /path/to/ca.pem  # Explicit CA trust

collect:
  timeout_sec: 30              # Per-switch timeout
  parallel: 2                  # Max concurrent switch connections
  skip_counters: false         # Skip interface counter collection
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

The embedded web UI provides an interactive Obsidian-style graph visualization:

- **Hierarchical layout** — BMCs on top, TOR switches in the middle, hosts at the bottom
- **Force-directed layout** — alternative physics-based layout
- **Group by TOR** — compound nodes grouping hosts under their connected switch
- **Hover interactions** — highlights connected neighbors, dims unrelated nodes, shows port labels
- **Click popup card** — floating card with key device/link info near the clicked element
- **Full detail sidebar** — interface list with health indicators, traffic counters, connection list
- **Search** — find devices by name, ID, or chassis ID
- **Filter by type** — show only switches, hosts, BMCs, or all
- **DOWN link highlighting** — red dashed lines for operationally down connections
- **PNG export** — download the graph as a high-resolution image
- **Dark theme** — Obsidian-inspired dark UI optimized for NOC environments

## Supported Platforms

| Vendor | Platform | LLDP Path | Encoding |
|---|---|---|---|
| SONiC (Dell/MS) | Enterprise SONiC | OpenConfig `/openconfig-lldp:lldp/...` | JSON_IETF |
| Cisco | NX-OS | Native `/System/lldp-items/...` | JSON |

The tool automatically handles:
- **SONiC Get→Subscribe fallback** — SONiC returns empty for bulk Get on list paths; the tool falls back to Subscribe ONCE
- **JSON_IETF prefix stripping** — removes `module-name:` prefixes from response keys (RFC 7951)
- **Interface name normalization** — `eth1/1` → `Eth1/1`, `Ethernet0` unchanged

## Architecture

```
network-mapper/
├── cmd/network-mapper/
│   ├── main.go                # CLI entry (cobra): collect + serve commands
│   └── web/                   # Embedded web UI (go:embed)
│       ├── index.html
│       ├── css/app.css        # Dark theme
│       ├── js/
│       │   ├── graph.js       # Cytoscape.js init, layout, interactions
│       │   ├── sidebar.js     # Detail panel with interface health
│       │   ├── popup.js       # Floating card near clicked elements
│       │   ├── toolbar.js     # Layout, search, filter, export controls
│       │   └── app.js         # Main entry, topology transform, event wiring
│       └── lib/               # Vendored: cytoscape.min.js, dagre.min.js
├── internal/
│   ├── config/                # YAML config loading + env-var resolution
│   ├── gnmi/                  # gNMI client, TLS/TOFU, path parsing
│   ├── transform/             # LLDP, interface, system data parsers
│   ├── collector/             # Orchestrator: connect, collect, build topology
│   ├── topology/              # Core types: Device, Interface, Link, Topology
│   └── server/                # HTTP server: embedded web + REST API
└── examples/
    ├── config.yaml            # Sample config for 2 TOR switches
    └── sample-topology.json   # Sample output with enriched data
```

## Related Projects

This project builds on patterns from [arc-switch](../arc-switch), specifically:
- gNMI client with gRPC metadata auth and 64MB max message size
- TLS/TOFU certificate bootstrapping and caching
- Subscribe ONCE fallback for SONiC list path quirks
- OpenConfig + NX-OS native LLDP response transformers
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

