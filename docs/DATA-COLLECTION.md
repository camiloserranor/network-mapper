# Data Collection Reference

This document describes **what data** network-mapper collects from TOR switches, **how** it is gathered (gNMI paths, encoding, fallback strategies), and **how** each data category is used in the topology model and UI.

---

## Table of Contents

- [Collection Overview](#collection-overview)
- [Pipeline Stages](#pipeline-stages)
- [Data Categories](#data-categories)
  - [1. System Information](#1-system-information)
  - [2. LLDP Neighbors](#2-lldp-neighbors)
  - [3. Interface State](#3-interface-state)
  - [4. Resource Utilization](#4-resource-utilization)
  - [5. MAC Address Table](#5-mac-address-table)
  - [6. ARP Table](#6-arp-table)
  - [7. VLAN Configuration](#7-vlan-configuration)
  - [8. BGP Sessions](#8-bgp-sessions)
- [gNMI Path Reference](#gnmi-path-reference)
- [Platform Differences](#platform-differences)
- [Data Model](#data-model)
- [Data Flow](#data-flow)
- [Privacy & Security Considerations](#privacy--security-considerations)

---

## Collection Overview

Network-mapper connects to each configured TOR switch via **gNMI** (gRPC Network Management Interface) and performs a series of `Get` or `Subscribe ONCE` requests to collect operational state. The data is parsed, correlated, and assembled into a unified topology graph.

**Key principles:**

- **Read-only**: Only `Get`/`Subscribe` RPCs are used. No configuration changes are made.
- **Non-destructive**: Collection does not impact switch forwarding performance.
- **Graceful degradation**: If a data category fails, the error is recorded and collection continues with remaining categories.
- **Parallel execution**: Multiple switches are queried concurrently (configurable).
- **Platform-aware**: Each switch is queried using the correct YANG paths and encoding for its platform (SONiC/OpenConfig or NX-OS native).

---

## Pipeline Stages

Each switch is processed through 8 sequential collection stages:

| Stage | Category | Required | Platforms |
|-------|----------|----------|-----------|
| 1 | System Information | Yes | All |
| 2 | LLDP Neighbors | Yes | All |
| 3 | Interface State | Yes | All |
| 4 | Resource Utilization (CPU/Memory) | No | All |
| 5 | MAC Address Table | No | NX-OS only |
| 6 | ARP Table | No | NX-OS only |
| 7 | VLAN Configuration | No | NX-OS only |
| 8 | BGP Sessions | No | All |

Stages marked "No" under Required are best-effort — failures are logged but do not prevent topology assembly.

---

## Data Categories

### 1. System Information

**Purpose:** Establish the switch's identity (hostname, version, uptime) to anchor the topology graph.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| Hostname | System hostname / FQDN | `tor-1.azure.local` |
| Software Version | Operating system version string | `NX-OS 10.3(4a)` |
| Uptime | Time since last boot | `14 days, 3:42:01` |

**How it is used:**

- The hostname becomes the **device ID** if no config-level name matches.
- Software version and uptime are displayed in the switch detail view.
- The hostname is used to **deduplicate** devices seen via LLDP from other switches.

---

### 2. LLDP Neighbors

**Purpose:** Discover the physical connections between devices — the core of the topology map.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| Local Port | Interface where the neighbor was seen | `Ethernet1/1` |
| Chassis ID | Neighbor's unique identifier (usually MAC) | `00:1a:2b:3c:4d:5e` |
| Port ID | Remote port identifier | `Ethernet1/48` |
| Port Description | Human-readable port description | `To-TOR-2-Eth1/1` |
| System Name | Neighbor's hostname | `hci-node-01.azure.local` |
| System Description | Neighbor's OS/platform string | `Linux 5.15.0` |
| Management Address | Neighbor's management IP | `10.0.0.5` |
| Capabilities | LLDP advertised capabilities | `bridge`, `router`, `station-only` |

**How it is used:**

- Each LLDP neighbor creates a **Link** in the topology.
- The System Name and Capabilities are used to **classify** the remote device (`switch`, `host`, `bmc`, `unknown`).
- Chassis ID provides a stable identifier for devices that may change hostnames.
- Links are enriched with interface status and speed from Stage 3.

---

### 3. Interface State

**Purpose:** Determine operational health of every physical port and enrich link data with speed/status.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| Name | Interface identifier (normalized) | `Ethernet1/1` |
| Oper Status | Operational state | `UP` or `DOWN` |
| Speed | Negotiated link speed | `25G`, `100G`, `1G` |
| MTU | Maximum Transmission Unit | `9216` |
| In Octets | Total bytes received | `1234567890` |
| Out Octets | Total bytes transmitted | `987654321` |
| In Packets | Total packets received | `5000000` |
| Out Packets | Total packets transmitted | `4800000` |
| In Errors | Receive errors | `0` |
| Out Errors | Transmit errors | `12` |
| In Discards | Receive discards | `0` |
| Out Discards | Transmit discards | `5` |

**How it is used:**

- Interface status determines **port health** (% UP in the fabric view).
- Speed and status are attached to Links discovered via LLDP.
- Counters provide traffic statistics for bandwidth visualization.
- Error/discard counters can surface unhealthy links.
- The port list drives the **switch front-panel** SVG in the detail view.

---

### 4. Resource Utilization

**Purpose:** Monitor switch health (CPU/memory) to detect overloaded devices.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| CPU Utilization | Average CPU usage percentage | `23.5` |
| Memory Used | RAM currently in use (bytes) | `4294967296` |
| Memory Total | Total RAM available (bytes) | `8589934592` |

**How it is used:**

- CPU utilization is displayed as a **health indicator** in the switch detail view.
- Memory usage shown alongside CPU for operational awareness.
- Could drive alerting thresholds in future iterations.

---

### 5. MAC Address Table

**Purpose:** Discover virtual machines and containers connected to hosts behind TOR switches.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| MAC Address | Learned Ethernet address | `00:15:5d:a1:b2:c3` |
| VLAN ID | VLAN where the MAC was learned | `100` |
| Port | Switch port where MAC was seen | `Ethernet1/5` |
| Type | Entry type | `dynamic`, `static` |

**How it is used:**

- Dynamic MAC entries on host-facing ports indicate **VM endpoints**.
- Cross-referencing with ARP gives each VM its IP address.
- MAC-to-port mapping enables **endpoint location** tracking.
- Only dynamic entries on non-uplink ports are considered (avoids counting infrastructure MACs).

**Platform support:** NX-OS only. SONiC MAC table collection is planned but not yet implemented.

---

### 6. ARP Table

**Purpose:** Map IP addresses to MAC addresses for endpoint enrichment.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| IP Address | Layer 3 address | `192.168.100.15` |
| MAC Address | Corresponding Layer 2 address | `00:15:5d:a1:b2:c3` |
| Interface | SVI or L3 interface | `Vlan100` |

**How it is used:**

- Joined with MAC table entries to give endpoints their **IP addresses**.
- Enables the UI to show VMs with both MAC and IP identifiers.
- Helps correlate with Azure Local deployment data (which uses IPs as identifiers).

**Platform support:** NX-OS only.

---

### 7. VLAN Configuration

**Purpose:** Understand Layer 2 segmentation and SVI gateway assignments.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| VLAN ID | IEEE 802.1Q identifier (1-4094) | `100` |
| VLAN Name | Administrative name | `HCI-Storage` |
| Status | Operational state | `active` |
| Gateway IP | SVI IP address (if routed) | `192.168.100.1/24` |
| Member Ports | Interfaces assigned to this VLAN | `[Ethernet1/1, Ethernet1/2]` |
| Source Switch | Which switch reported this VLAN | `tor-1` |

**How it is used:**

- VLAN data is displayed in the switch detail view.
- VLAN membership links endpoints to their network segment.
- Gateway IPs help identify the routing topology.
- The VLAN list provides context for understanding traffic segmentation in Azure Local.

**Platform support:** NX-OS only. OpenConfig VLAN collection is planned.

---

### 8. BGP Sessions

**Purpose:** Capture the routing control-plane state to assess fabric convergence health.

**What is collected:**

| Field | Description | Example |
|-------|-------------|---------|
| Neighbor Address | Peer IP address | `10.0.0.1` |
| Peer AS | Remote Autonomous System Number | `65001` |
| Local AS | Local Autonomous System Number | `65000` |
| Peer Type | iBGP or eBGP | `EXTERNAL` |
| Session State | FSM state | `ESTABLISHED` |
| Description | Administrative description | `spine-1` |
| Enabled | Administrative state | `true` |
| VRF Name | Routing context | `default` |
| Established Transitions | Number of state flaps | `3` |
| Last Established | Timestamp of last session up | `1625000000` |
| Messages Received | Total BGP messages in | `15200` |
| Messages Sent | Total BGP messages out | `14800` |
| Prefixes Received | Number of routes learned | `200` |
| Prefixes Sent | Number of routes advertised | `150` |

**How it is used:**

- **Session state** immediately shows if the routing fabric is healthy.
- **Established/Total** ratio displayed as a BGP health summary.
- **Peer AS** identifies iBGP (same AS) vs eBGP (different AS) relationships.
- **Prefixes received/sent** can detect route leaks or missing advertisements.
- **Established transitions** (flap count) flags unstable peerings.
- **VRF** shows which routing domain each session belongs to.
- All data displayed in the **BGP Sessions table** in the switch detail view.

---

## gNMI Path Reference

### OpenConfig (SONiC and OpenConfig-compliant switches)

| Category | gNMI Path | Encoding |
|----------|-----------|----------|
| System | `/openconfig-system:system/state` | JSON_IETF |
| LLDP | `/openconfig-lldp:lldp/interfaces/interface/neighbors` | JSON_IETF |
| Interfaces | `/openconfig-interfaces:interfaces/interface/state` | JSON_IETF |
| Interface Counters | `/openconfig-interfaces:interfaces/interface/state/counters` | JSON_IETF |
| CPU | `/openconfig-system:system/cpus/cpu[index=ALL]/state` | JSON_IETF |
| Memory | `/openconfig-system:system/memory/state` | JSON_IETF |
| BGP | `/openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/neighbors` | JSON_IETF |

### Cisco NX-OS (Native YANG)

| Category | gNMI Path | Encoding |
|----------|-----------|----------|
| LLDP | `/System/lldp-items/inst-items/if-items/If-list` | JSON |
| CPU | `/System/procsys-items/syscpusummary-items` | JSON |
| Memory | `/System/procsys-items/sysmem-items` | JSON |
| MAC Table | `/System/mac-items` | JSON |
| ARP Table | `/System/arp-items/inst-items/dom-items/Dom-list/db-items/Db-list/adj-items/AdjEp-list` | JSON |
| VLANs | `/System/bd-items` | JSON |
| BGP | `/System/bgp-items/inst-items/dom-items/Dom-list/peer-items/Peer-list` | JSON |

---

## Platform Differences

| Aspect | SONiC (OpenConfig) | Cisco NX-OS |
|--------|-------------------|-------------|
| Encoding | `JSON_IETF` | `JSON` |
| Get behavior | May return empty on bulk list paths; uses Subscribe ONCE fallback | Standard Get works |
| Module prefixes | Stripped (e.g., `openconfig-lldp:` removed) | Not applicable |
| Interface naming | `Ethernet0`, `Ethernet4` | `eth1/1`, `Ethernet1/1` |
| MAC/ARP/VLAN | Not yet supported | Fully supported |
| BGP paths | OpenConfig standard | NX-OS native YANG |
| Authentication | gRPC metadata (username/password) | gRPC metadata (username/password) |

---

## Data Model

The collected data is assembled into the **v2 hierarchical topology schema** (served at `/api/topology`). See [`topology-schema.md`](topology-schema.md) for the full field reference.

The schema version is `"2.0"` with top-level sections: `metadata`, `fabric` (switches, interfaces, peer links), `compute` (hosts, endpoints), `vlans`, and `warnings`.

---

## Offline / Mock Mode

You can run the topology pipeline without live switch access by using pre-collected raw gNMI data:

```bash
# Serve from a raw gNMI dump directory
network-mapper serve --from-raw ./gnmi-raw-data/2026-05-11_094644 --port 8080
```

The raw data directory should contain one subdirectory per switch, each with JSON files for the collected gNMI paths. This enables:

- **Development** without switch access
- **Testing** transform/builder changes against known data
- **Demos** with reproducible topology output

---

## Data Flow

```
┌────────────────────────────────────────────────────────┐
│ TOR Switch (gNMI Server)                               │
│  ├─ LLDP daemon → neighbor state                       │
│  ├─ Interface manager → port state/counters            │
│  ├─ BGP daemon → peer state/statistics                 │
│  ├─ MAC/ARP tables → learned addresses                 │
│  └─ System state → hostname, CPU, memory               │
└───────────────────────┬────────────────────────────────┘
                        │ gNMI Get / Subscribe ONCE
                        ▼
┌────────────────────────────────────────────────────────┐
│ Collector (internal/collector/)                         │
│  ├─ Per-switch: 8 collection stages (parallel across   │
│  │   switches, sequential within each switch)          │
│  ├─ Transform: raw JSON → typed Go structs             │
│  └─ Assemble: merge, deduplicate, classify, enrich     │
└───────────────────────┬────────────────────────────────┘
                        │ topology.Topology struct
                        ▼
┌────────────────────────────────────────────────────────┐
│ Persistence                                            │
│  ├─ File mode: write to topology.json (--output)       │
│  └─ Live mode: store in memory, push via WebSocket     │
└───────────────────────┬────────────────────────────────┘
                        │ HTTP / WebSocket
                        ▼
┌────────────────────────────────────────────────────────┐
│ Web UI (cmd/network-mapper/web/)                       │
│  ├─ Fetch: GET /api/topology                           │
│  ├─ Transform: classify switches, build port maps      │
│  ├─ Render: Cytoscape graph (fabric view)              │
│  ├─ Detail views: switch front-panel, BGP table,       │
│  │   host NICs, VM cards                               │
│  └─ Live: WebSocket reconnect for streaming updates    │
└────────────────────────────────────────────────────────┘
```

---

## Privacy & Security Considerations

### What we collect

- **Network topology metadata**: device names, port names, IP addresses, MAC addresses, VLAN IDs, BGP AS numbers.
- **Operational counters**: traffic statistics, message counts, prefix counts.
- **No payload data**: We never inspect or collect any user traffic content.
- **No credentials stored in output**: Switch credentials are in the config file only and are never written to topology JSON.

### Sensitive data in the output

The topology JSON may contain:

| Data | Sensitivity | Mitigation |
|------|-------------|------------|
| Management IPs | Medium | Limit access to topology files |
| MAC addresses | Low-Medium | Standard operational data |
| Hostnames/FQDNs | Low-Medium | May reveal naming conventions |
| BGP AS numbers | Low | Public in peering scenarios |
| VLAN names | Low | May reveal network segmentation strategy |

### Recommendations

1. **Restrict access** to the topology JSON file and web UI (bind to localhost or use authentication).
2. **Use environment variables** for switch credentials (`${SWITCH_PASSWORD}`), never hardcode.
3. **TLS for gNMI**: Use proper CA certificates in production (avoid `skip_verify`).
4. **Network segmentation**: Run the collector from a management network with controlled access to switches.
5. **Audit logging**: The collector logs which switches are queried and when.
