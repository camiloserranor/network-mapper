# Requirements Specification — Network Mapper v0.2

## 1. Overview

Network Mapper is a Go CLI tool that discovers the physical topology of Azure Local deployments by querying TOR (Top-of-Rack) switches via gNMI to retrieve LLDP neighbor data. It produces a versioned JSON document representing the physical connectivity graph between switches, hosts, and their interfaces.

### 1.1 Goals

- Discover physical topology automatically from LLDP data on TOR switches
- Support multiple switch vendors (SONiC, Cisco NX-OS) through a vendor-abstraction layer
- Produce a machine-readable, versioned topology document (JSON)
- Handle real-world gNMI quirks: encoding differences, empty-response fallbacks, path variations

### 1.2 Non-Goals (v0.1)

- LAG/MLAG/port-channel awareness (follow-up)
- Real-time streaming / continuous monitoring (follow-up)
- Topology diff / drift detection (follow-up)
- Direct Azure Local API integration (follow-up; optional enrichment file supported)

---

## 2. Functional Requirements

### 2.1 CLI Interface

| ID | Requirement |
|---|---|
| FR-CLI-01 | The tool SHALL be a single Go binary with `collect` and `serve` subcommands |
| FR-CLI-02 | The tool SHALL accept a `--config` flag pointing to a YAML configuration file |
| FR-CLI-03 | The tool SHALL accept `--output` flag to override the output file path |
| FR-CLI-04 | The tool SHALL accept `--verbose` flag for detailed logging to stderr |
| FR-CLI-05 | The tool SHALL exit with code 0 on success, non-zero on failure |
| FR-CLI-06 | CLI flags SHALL override corresponding values in the YAML config |

### 2.2 Configuration

| ID | Requirement |
|---|---|
| FR-CFG-01 | The tool SHALL load configuration from a YAML file |
| FR-CFG-02 | Each switch target SHALL specify: name, address, port, vendor type, TLS settings, and credential env var names |
| FR-CFG-03 | Credentials SHALL be resolved from environment variables at runtime — never stored in the config file |
| FR-CFG-04 | The tool SHALL validate the configuration at startup and report clear errors for missing/invalid fields |
| FR-CFG-05 | The vendor field SHALL accept `sonic`, `cisco-nx-os`, or `auto` |
| FR-CFG-06 | Discovery settings (timeout, encoding, retry) SHALL have sensible defaults |

### 2.3 gNMI Connection

| ID | Requirement |
|---|---|
| FR-GNMI-01 | The tool SHALL connect to each TOR switch using gRPC with the `openconfig/gnmi` protobuf API |
| FR-GNMI-02 | The tool SHALL support TLS with trust-on-first-use (TOFU) certificate bootstrapping |
| FR-GNMI-03 | The tool SHALL support TLS with a pinned CA certificate file |
| FR-GNMI-04 | The tool SHALL support username/password authentication via gRPC metadata |
| FR-GNMI-05 | The tool SHALL support certificate-based mutual TLS (mTLS) authentication |
| FR-GNMI-06 | The tool SHALL query gNMI Capabilities on connection to detect supported models and encodings |
| FR-GNMI-07 | The tool SHALL support both JSON and JSON_IETF gNMI encoding |
| FR-GNMI-08 | The tool SHALL use gNMI Get as the primary data retrieval method |
| FR-GNMI-09 | The tool SHALL fall back to gNMI Subscribe ONCE when Get returns empty responses (required for SONiC on list paths) |
| FR-GNMI-10 | The tool SHALL respect the configured timeout for each gNMI request |
| FR-GNMI-11 | The tool SHALL retry failed connections with configurable retry count and delay |

### 2.4 LLDP Discovery

| ID | Requirement |
|---|---|
| FR-LLDP-01 | The tool SHALL query the LLDP neighbor table from each configured TOR switch |
| FR-LLDP-02 | For SONiC switches, the tool SHALL use the OpenConfig path: `/openconfig-lldp:lldp/interfaces/interface/neighbors` |
| FR-LLDP-03 | For Cisco NX-OS switches, the tool SHALL use the native path: `/System/lldp-items/inst-items/if-items/If-list` |
| FR-LLDP-04 | When vendor is `auto`, the tool SHALL probe Capabilities and select the appropriate LLDP path based on supported models |
| FR-LLDP-05 | The tool SHALL extract from each LLDP neighbor entry: chassis-id, port-id, system-name, system-description, port-description, management-address |
| FR-LLDP-06 | The tool SHALL normalize interface names across vendors to a canonical format (e.g., `eth1/1` → `Eth1/1`, `Ethernet0` unchanged) |
| FR-LLDP-07 | The tool SHALL handle both bulk responses (array of neighbors) and per-neighbor subscribe responses |
| FR-LLDP-08 | The tool SHALL strip YANG module prefixes from JSON_IETF keys (e.g., `openconfig-lldp:state` → `state`) |

### 2.5 Topology Model

| ID | Requirement |
|---|---|
| FR-TOPO-01 | The topology model SHALL represent devices as nodes with: id, type (switch/host/unknown), chassis-id, system-name, management-address, and a list of interfaces |
| FR-TOPO-02 | The topology model SHALL represent interfaces as first-class records with: name, normalized name, description |
| FR-TOPO-03 | The topology model SHALL represent physical links with: local-device, local-port, remote-device, remote-port, remote-chassis-id, source (always "lldp" for v0.1), and discovery timestamp |
| FR-TOPO-04 | Devices SHALL be uniquely identified by the composite key: `(source_switch + local_interface + remote_chassis_id + remote_port_id)` — not by system-name alone |
| FR-TOPO-05 | The topology model SHALL deduplicate links seen from multiple switch perspectives (e.g., host connected to both TOR-1 and TOR-2) |
| FR-TOPO-06 | The topology model SHALL classify device types as best-effort based on LLDP capabilities and system-description |

### 2.6 Output

| ID | Requirement |
|---|---|
| FR-OUT-01 | The tool SHALL produce a JSON file as its primary output |
| FR-OUT-02 | The JSON output SHALL include a `schema_version` field (starting at `"1.0"`) |
| FR-OUT-03 | The JSON output SHALL include `collected_at` (RFC3339 timestamp) |
| FR-OUT-04 | The JSON output SHALL include `source_switches` (list of switch names queried) |
| FR-OUT-05 | The JSON output SHALL include `partial_failures` (list of switches/paths that failed, with error messages) — the tool SHALL NOT fail entirely if one switch is unreachable |
| FR-OUT-06 | The JSON output SHALL include `devices` (list of discovered devices) and `links` (list of physical connections) |
| FR-OUT-07 | The JSON output SHALL be deterministically ordered (sorted by device ID, then interface name) for diff-friendliness |

### 2.7 Web UI Visualization

| ID | Requirement |
|---|---|
| FR-UI-01 | The tool SHALL embed a web UI served by the `serve` subcommand via `go:embed` |
| FR-UI-02 | The web UI SHALL display topology as an interactive graph using Cytoscape.js |
| FR-UI-03 | The web UI SHALL support a hierarchical layout (BMC top, switches middle, hosts bottom) |
| FR-UI-04 | The web UI SHALL support a force-directed (physics-based) layout |
| FR-UI-05 | The web UI SHALL highlight neighbors on hover and dim unrelated nodes |
| FR-UI-06 | The web UI SHALL show a floating popup card when a node or edge is clicked |
| FR-UI-07 | The web UI SHALL show a detail sidebar with interface health, counters, and connections |
| FR-UI-08 | The web UI SHALL support searching devices by name, ID, or chassis ID |
| FR-UI-09 | The web UI SHALL support filtering by device type (switch, host, BMC) |
| FR-UI-10 | The web UI SHALL render DOWN links with red dashed lines |
| FR-UI-11 | The web UI SHALL support PNG export of the graph |
| FR-UI-12 | The web UI SHALL support compound node grouping by TOR switch |
| FR-UI-13 | The web UI SHALL use a dark theme suitable for NOC environments |
| FR-UI-14 | The web UI SHALL NOT require any external CDN or build toolchain |

### 2.8 Data Collection

| ID | Requirement |
|---|---|
| FR-DATA-01 | The `collect` command SHALL query LLDP neighbors from each configured TOR switch |
| FR-DATA-02 | The `collect` command SHALL query interface state (oper_status, speed, MTU) from each switch |
| FR-DATA-03 | The `collect` command SHALL query interface traffic counters (in/out octets, packets, errors, discards) |
| FR-DATA-04 | The `collect` command SHALL query system information (hostname, software version, uptime) |
| FR-DATA-05 | The `collect` command SHALL enrich topology links with interface counters from the local switch port |
| FR-DATA-06 | The `collect` command SHALL classify discovered devices as switch, host, bmc, or unknown based on LLDP data |

### 2.9 Enrichment (Optional)

| ID | Requirement |
|---|---|
| FR-ENR-01 | The tool SHALL optionally accept an Azure Local node mapping file to correlate LLDP system-names to Azure Local node names |
| FR-ENR-02 | Enrichment data SHALL be added as annotations on devices — it SHALL NOT replace LLDP-discovered data |
| FR-ENR-03 | NIC purpose detection (mgmt/storage/compute) SHALL be treated as best-effort annotation with a confidence indicator |

---

## 3. Non-Functional Requirements

### 3.1 Performance

| ID | Requirement |
|---|---|
| NFR-PERF-01 | The tool SHALL query all configured switches concurrently |
| NFR-PERF-02 | Full discovery of a typical deployment (2 TORs, 16 hosts) SHALL complete in under 60 seconds |

### 3.2 Security

| ID | Requirement |
|---|---|
| NFR-SEC-01 | Credentials SHALL only be read from environment variables — never from config files, CLI flags, or logs |
| NFR-SEC-02 | TLS SHALL be enabled by default; connecting without TLS SHALL require explicit opt-in |
| NFR-SEC-03 | The tool SHALL NOT log or print credentials, tokens, or certificate private keys |
| NFR-SEC-04 | The tool SHALL validate TLS certificates (TOFU or pinned CA) — InsecureSkipVerify SHALL only be used during the TOFU probe itself |

### 3.3 Reliability

| ID | Requirement |
|---|---|
| NFR-REL-01 | Failure to reach one switch SHALL NOT prevent discovery from other switches |
| NFR-REL-02 | The tool SHALL report partial failures in the output JSON, not silently drop data |
| NFR-REL-03 | The tool SHALL handle gNMI responses with missing or unexpected fields gracefully (log warnings, not crash) |

### 3.4 Maintainability

| ID | Requirement |
|---|---|
| NFR-MAINT-01 | Vendor-specific LLDP parsing SHALL be isolated behind a common interface so new vendors can be added without modifying core logic |
| NFR-MAINT-02 | Unit tests SHALL use captured gNMI response fixtures from real switches, not only synthetic data |
| NFR-MAINT-03 | The gNMI client SHALL be wrapped behind an interface to allow mocking in tests |

---

## 4. Technical Design Decisions

### 4.1 Language and Dependencies

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go | First-class gNMI/protobuf support, single binary distribution, strong concurrency |
| gNMI library | `openconfig/gnmi` proto + `google.golang.org/grpc` | Same stack used by arc-switch; proven in production |
| CLI framework | `cobra` | Standard Go CLI framework, supports subcommands |
| Config parsing | `gopkg.in/yaml.v3` | Same as arc-switch config pattern |
| Testing | `testing` + `testify` | Standard Go testing with assertion helpers |

### 4.2 gNMI YANG Paths

LLDP paths vary by vendor. The tool maintains a registry of paths per vendor:

| Vendor | Path Name | YANG Path |
|---|---|---|
| SONiC (OpenConfig) | `lldp-neighbors` | `/openconfig-lldp:lldp/interfaces/interface/neighbors` |
| Cisco NX-OS (Native) | `nx-lldp` | `/System/lldp-items/inst-items/if-items/If-list` |

When vendor is `auto`, the tool queries `Capabilities()` and checks for:
- `openconfig-lldp` in supported models → use OpenConfig path
- No OpenConfig LLDP → fall back to native vendor path based on system identity

### 4.3 Data Flow

```
Config Load → Validate → For each switch (concurrent):
  │
  ├─ Connect (gRPC + TLS/TOFU)
  ├─ Capabilities probe (detect vendor/encoding)
  ├─ LLDP Get (with Subscribe ONCE fallback)
  ├─ Vendor-specific transform → normalized LLDP records
  │
  └─► Merge all LLDP records
        │
        ├─ Build device list (from switch configs + LLDP neighbors)
        ├─ Build link list (deduplicated)
        ├─ Optional enrichment (Azure Local node names)
        │
        └─► Write topology.json
```

### 4.4 Project Structure

```
network-mapper/
├── cmd/
│   └── network-mapper/
│       ├── main.go             # Entry point (collect + serve commands)
│       └── web/                # Embedded web UI (go:embed)
│           ├── index.html
│           ├── css/app.css     # Dark theme
│           ├── js/
│           │   ├── graph.js    # Cytoscape init, layout, interactions
│           │   ├── sidebar.js  # Detail panel
│           │   ├── popup.js    # Floating card
│           │   ├── toolbar.js  # Controls
│           │   └── app.js      # Main entry
│           └── lib/            # Vendored: cytoscape, dagre
├── internal/
│   ├── config/
│   │   └── config.go           # YAML config loading + validation
│   ├── gnmi/
│   │   ├── client.go           # gNMI client (Get, SubscribeOnce, GetWithFallback)
│   │   └── tls.go              # TLS/TOFU cert bootstrapping
│   ├── transform/
│   │   ├── helpers.go          # JSON parsing helpers
│   │   ├── lldp.go             # LLDP parsers (OpenConfig + NX-OS)
│   │   ├── interfaces.go       # Interface state/counter parser
│   │   └── system.go           # System info parser
│   ├── collector/
│   │   └── collector.go        # Orchestrator: parallel collection + topology assembly
│   ├── topology/
│   │   └── types.go            # Device, Interface, Link, Topology types
│   └── server/
│       └── server.go           # HTTP server: embedded web + REST API
├── examples/
│   ├── config.yaml             # Sample config for 2 TOR switches
│   └── sample-topology.json    # Sample output with enriched data
├── go.mod
├── go.sum
├── README.md
└── REQUIREMENTS.md
```

### 4.5 Vendor Abstraction

Based on the transformer registry pattern from arc-switch:

```go
// discovery/registry.go
type LLDPTransformer interface {
    // YANGPath returns the gNMI path for LLDP neighbor discovery.
    YANGPath() string
    // Transform converts raw gNMI notifications into normalized LLDP records.
    Transform(notifications []gnmi.Notification) ([]LLDPNeighbor, error)
}

// Self-registering transformers (like arc-switch):
// discovery/openconfig.go  → Register("sonic", ...)
// discovery/nxos.go        → Register("cisco-nx-os", ...)
```

### 4.6 Key Type Definitions

```go
// topology/model.go

type Topology struct {
    SchemaVersion   string          `json:"schema_version"`
    CollectedAt     time.Time       `json:"collected_at"`
    SourceSwitches  []string        `json:"source_switches"`
    PartialFailures []PartialError  `json:"partial_failures"`
    Devices         []Device        `json:"devices"`
    Links           []Link          `json:"links"`
}

type Device struct {
    ID                string      `json:"id"`
    Type              string      `json:"type"`     // switch, host, unknown
    ChassisID         string      `json:"chassis_id"`
    SystemName        string      `json:"system_name"`
    SystemDescription string      `json:"system_description,omitempty"`
    ManagementAddress string      `json:"management_address,omitempty"`
    Interfaces        []Interface `json:"interfaces,omitempty"`
    Annotations       map[string]string `json:"annotations,omitempty"`
}

type Interface struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}

type Link struct {
    LocalDevice    string `json:"local_device"`
    LocalPort      string `json:"local_port"`
    RemoteDevice   string `json:"remote_device"`
    RemotePort     string `json:"remote_port"`
    RemoteChassisID string `json:"remote_chassis_id"`
    Source         string `json:"source"`        // "lldp"
    DiscoveredAt   string `json:"discovered_at"` // RFC3339
}

type PartialError struct {
    Switch  string `json:"switch"`
    Error   string `json:"error"`
}
```

---

## 5. Patterns Borrowed from arc-switch

The following proven patterns from the [arc-switch TelemetryClient](../arc-switch/src/TelemetryClient) are directly applicable and should be reused:

| Pattern | Arc-Switch Source | Network Mapper Usage |
|---|---|---|
| gNMI client with auth context | `internal/gnmi/client.go` | Same connection, auth, and Get/SubscribeOnce pattern |
| TLS TOFU bootstrapping | `internal/gnmi/tls.go` | Same FetchServerCert → TOFUCertPool flow |
| YANG path parsing | `internal/gnmi/client.go:parsePath()` | Same path parsing with module prefix stripping |
| JSON_IETF module prefix stripping | `internal/gnmi/client.go:stripModulePrefixes()` | Required for SONiC JSON_IETF responses |
| Subscribe ONCE fallback | `internal/gnmi/client.go:SubscribeOnce()` | SONiC returns empty for bulk Get on list paths |
| OpenConfig LLDP transform | `internal/transform/lldp_neighbor.go` | Adapt for topology builder input |
| Native NX-OS LLDP transform | `internal/transform/native_lldp.go` | Adapt for topology builder input |
| Transformer registry | `internal/transform/registry.go` | Self-registering vendor transformers via `init()` |
| Interface name normalization | `internal/transform/common.go:NormalizeInterfaceName()` | Canonical interface names across vendors |
| Config with env-var credentials | `internal/config/config.go` | Same credential resolution pattern |
| Encoding resolution | `internal/gnmi/client.go:resolveEncoding()` | JSON vs JSON_IETF per vendor |

---

## 6. Acceptance Criteria

### 6.1 Minimum Viable

- [ ] Can connect to a SONiC TOR switch via gNMI with TLS/TOFU and retrieve LLDP neighbors
- [ ] Can connect to a Cisco NX-OS TOR switch via gNMI and retrieve LLDP neighbors
- [ ] Produces a valid `topology.json` with devices and links
- [ ] Handles partial failures (one switch down) without crashing
- [ ] JSON output is deterministically ordered

### 6.2 Testing

- [ ] Unit tests for config loading/validation
- [ ] Unit tests for OpenConfig LLDP transformation (using captured fixtures)
- [ ] Unit tests for NX-OS native LLDP transformation (using captured fixtures)
- [ ] Unit tests for topology builder (deduplication, device classification)
- [ ] Integration test with mocked gNMI server
