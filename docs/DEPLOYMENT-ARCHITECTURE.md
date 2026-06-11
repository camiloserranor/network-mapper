# Deployment Architecture

How to deploy Network Mapper in Azure Local environments — physical topology, network connectivity, and deployment scenarios.

---

## Table of Contents

- [Overview](#overview)
- [Network Topology Context](#network-topology-context)
- [Deployment Scenarios](#deployment-scenarios)
  - [Scenario 1: Host Node VM (Production)](#scenario-1-host-node-vm-production)
  - [Scenario 2: Jumpbox VM (Isolated Management Network)](#scenario-2-jumpbox-vm-isolated-management-network)
- [Communication Paths](#communication-paths)
  - [gNMI Collection (Switch → Mapper)](#gnmi-collection-switch--mapper)
  - [Web UI Access (User → Mapper)](#web-ui-access-user--mapper)
- [Prerequisites by Scenario](#prerequisites-by-scenario)
- [Choosing a Deployment Scenario](#choosing-a-deployment-scenario)

---

## Overview

Network Mapper is a single Go binary that:

1. **Collects** topology data from TOR switches via gNMI (port 50051)
2. **Serves** an interactive web UI over HTTP (port 8080)

The binary must run on a machine that has **network reachability to the TOR switch gNMI ports**. Where you run it depends on how your management network is configured.

For switch-side setup (enabling gNMI, credentials, VRF), see [SWITCH-SETUP.md](SWITCH-SETUP.md).  
For details on what data is collected, see [DATA-COLLECTION.md](DATA-COLLECTION.md).

---

## Network Topology Context

An Azure Local rack typically includes:

| Component | Quantity | Network Connectivity |
|-----------|----------|---------------------|
| TOR switches | 2 per rack | Data network + management network (separate VRFs) |
| Host nodes | 2–16 per rack | Connected to both TOR switches (dual-homed) |
| Management network | 1 | Out-of-band; may be isolated from data plane |

Each host node is **dual-homed** — physically cabled to both TOR switches for redundancy. The TOR switches expose gNMI on a configurable VRF (see [SWITCH-SETUP.md](SWITCH-SETUP.md#2-enable-gnmi) for VRF selection).

<!-- TODO: Add physical topology diagram showing rack layout with dual-homed hosts -->

---

## Deployment Scenarios

### Scenario 1: Host Node VM (Production)

**When to use:** Standard Azure Local deployments where the data network provides reachability between VMs and TOR switch management interfaces.

```
┌─────────────────────────────────────────────────────────────────┐
│ Azure Local Rack                                                │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ TOR Switch A                    TOR Switch B              │  │
│  │ gNMI :50051                     gNMI :50051              │  │
│  └───────┬───────────────────────────────┬───────────────────┘  │
│          │ data network (default VRF)    │                      │
│          │                               │                      │
│  ┌───────┴───────────────────────────────┴───────────────────┐  │
│  │                                                           │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │  │
│  │  │  Host Node  │  │  Host Node  │  │  Host Node  │       │  │
│  │  │             │  │             │  │             │       │  │
│  │  │ ┌─────────┐ │  │             │  │             │       │  │
│  │  │ │ Network │ │  │             │  │             │       │  │
│  │  │ │ Mapper  │ │  │             │  │             │       │  │
│  │  │ │ VM      │ │  │             │  │             │       │  │
│  │  │ │ :8080   │ │  │             │  │             │       │  │
│  │  │ └─────────┘ │  │             │  │             │       │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘       │  │
│  │                                                           │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**How it works:**

1. Network Mapper VM runs on one of the cluster's host nodes (Arc-enabled VM)
2. gNMI is configured on the TOR switches' **default VRF** — same network the hosts use
3. The VM connects to each TOR switch on port 50051 over the data network
4. Users access the web UI at `http://<vm-ip>:8080` from any machine on the same network

**Key characteristics:**

- No special networking required — the VM is on the same subnet as the switch SVIs
- Azure Arc managed identity provides Key Vault access for credentials
- The VM is managed alongside the cluster workloads

---

### Scenario 2: Jumpbox VM (Isolated Management Network)

**When to use:** Lab environments or deployments where the management network is **completely isolated** from the data plane. In this setup, gNMI is bound to the management VRF, and only machines with direct access to the management network can reach the switches.

```
┌─────────────────────────────────────────────────────────────────┐
│ Azure Local Rack                                                │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ TOR Switch A                    TOR Switch B              │  │
│  │ gNMI :50051 (mgmt VRF)         gNMI :50051 (mgmt VRF)   │  │
│  └───┬───────────────────────────────────┬───────────────────┘  │
│      │ management network (isolated)     │                      │
│      │                                   │                      │
│  ┌───┴───────────────────────────────────┴───────────────────┐  │
│  │ Management Switch / VLAN                                  │  │
│  └───────────────────────────┬───────────────────────────────┘  │
│                              │                                  │
│  ┌───────────────────────────┴───────────────────────────────┐  │
│  │ Jumpbox VM                                                │  │
│  │ (has NIC on management network)                           │  │
│  │                                                           │  │
│  │  ┌────────────────────────────────────┐                   │  │
│  │  │ Network Mapper                     │                   │  │
│  │  │ gNMI → TOR-A:50051, TOR-B:50051   │                   │  │
│  │  │ Web UI → :8080                     │                   │  │
│  │  └────────────────────────────────────┘                   │  │
│  │                                                           │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
             │
             │ RDP / SSH / HTTP :8080
             ▼
        ┌──────────┐
        │ Operator │
        │ laptop   │
        └──────────┘
```

**How it works:**

1. The management network is isolated — only specific VMs have interfaces on it
2. A jumpbox VM is provisioned with a NIC on the management network
3. gNMI is configured on the TOR switches' **management VRF**
4. Network Mapper runs on the jumpbox, connects to switches via their management IPs
5. Operators access the web UI by RDP-ing into the jumpbox or tunneling port 8080

**Key characteristics:**

- Required when the data network cannot route to switch management interfaces
- The jumpbox needs Azure identity (CLI login or managed identity) for Key Vault access
- Typically used in lab/test environments with restrictive network segmentation
- Users access the UI through the jumpbox (RDP + browser, or SSH tunnel)

---

## Communication Paths

### gNMI Collection (Switch → Mapper)

| Aspect | Detail |
|--------|--------|
| Protocol | gRPC (HTTP/2) |
| Port | 50051 (default, configurable) |
| Authentication | Username/password via gRPC metadata |
| TLS | Self-signed (TOFU or skip-verify) or CA-signed |
| Direction | Network Mapper → TOR switch (client-initiated) |
| VRF | Must match where gNMI is listening (default or management) |

The Network Mapper initiates gRPC connections to each configured switch. It performs read-only `Get` and `Subscribe ONCE` requests. No configuration changes are made to the switch.

For credential management details, see [README.md — Authentication & Credentials](../README.md#authentication--credentials).

<!-- TODO: Add diagram showing gNMI request/response flow between mapper and switches -->

### Web UI Access (User → Mapper)

| Aspect | Detail |
|--------|--------|
| Protocol | HTTP (plain) |
| Port | 8080 (configurable via `--port`) |
| Endpoints | `/` (UI), `/api/topology` (JSON), `/api/health` |
| WebSocket | `/ws` for live topology updates |
| Direction | Browser → Network Mapper (client-initiated) |

The web UI is embedded in the binary — no external CDN or package manager required. It serves a single-page application that fetches topology data from the REST API.

**Security note:** The HTTP server has no authentication. In production, restrict access via network segmentation or a reverse proxy with auth. Bind to `127.0.0.1` if only local access is needed.

<!-- TODO: Add diagram showing browser-to-mapper communication and UI rendering -->

---

## Prerequisites by Scenario

### Scenario 1: Host Node VM

| Prerequisite | How to verify |
|---|---|
| VM has IP on data network subnet | `ping <switch-data-ip>` from VM |
| gNMI on switches bound to **default VRF** | `show grpc internal service-status` → VRF `default` |
| Port 50051 reachable from VM | `Test-NetConnection <switch-ip> -Port 50051` |
| Arc managed identity enabled | `azcmagent show` |
| Key Vault access granted to identity | `az keyvault secret show --vault-name <vault> --name <secret>` |
| Port 8080 not blocked by firewall | Verify NSG rules allow inbound 8080 |

### Scenario 2: Jumpbox VM

| Prerequisite | How to verify |
|---|---|
| VM has NIC on management network | `ipconfig` shows management subnet IP |
| gNMI on switches bound to **management VRF** | `show grpc internal service-status` → VRF `management` |
| Port 50051 reachable from jumpbox | `Test-NetConnection <switch-mgmt-ip> -Port 50051` |
| Azure CLI logged in (or managed identity) | `az account show` |
| Key Vault access granted | `az keyvault secret show --vault-name <vault> --name <secret>` |
| RDP or tunnel access to jumpbox | Operator can reach jumpbox port 3389 or set up SSH tunnel for 8080 |

---

## Choosing a Deployment Scenario

| Factor | Host Node VM | Jumpbox VM |
|--------|---|---|
| Management network isolated? | No — switches reachable from data network | Yes — only mgmt-network hosts can reach switches |
| gNMI VRF | `default` | `management` |
| Arc managed identity | ✓ Available automatically | May require manual `az login` |
| UI access | Direct — any network user can browse `http://<vm>:8080` | Indirect — through RDP or SSH tunnel |
| Typical environment | Production Azure Local clusters | Lab / test environments with strict segmentation |
| Operational complexity | Low — standard VM deployment | Medium — requires jumpbox provisioning + network plumbing |

**Recommendation:** Use **Scenario 1** (Host Node VM) for production. Use **Scenario 2** (Jumpbox) only when network topology constraints prevent direct switch access from the data plane.
