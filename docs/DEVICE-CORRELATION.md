# Device Correlation and Classification

This document describes how network-mapper identifies, classifies, and correlates devices in an Azure Local deployment topology. Understanding these heuristics is important because the tool works with incomplete and sometimes inconsistent data from multiple sources.

## Data Sources

| Source | What it provides | Required? |
|--------|-----------------|-----------|
| **gNMI LLDP** (from TOR switches) | Neighbor chassis-id, system-name, system-description, capabilities, port-id, management-address | **Yes** — primary discovery mechanism |
| **gNMI System Info** | Switch's own hostname (FQDN), software version | Yes (for queried switches) |
| **gNMI Interfaces** | Port names, oper-status, speed, counters, MTU | Yes (for queried switches) |
| **gNMI MAC Table** | MAC → port → VLAN mappings on each switch | Optional (enables VM endpoint discovery) |
| **gNMI ARP Table** | MAC → IP mappings | Optional (enables IP assignment for VMs) |
| **Deployment JSON** | Hostnames, IP addresses, BMC IPs, NIC MAC addresses per host | **Optional** — enrichment only |
| **Config YAML** | Switch names (user-assigned), addresses, platform type | Yes |

**Key principle:** Everything should work without the deployment JSON. The deployment JSON adds hostnames and groups NIC ports, but the topology is fully discoverable from gNMI alone.

---

## Pipeline Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                        DATA COLLECTION                               │
│  For each configured switch (parallel):                              │
│    1. Connect via gNMI (TLS + username/password auth)                │
│    2. Collect: System Info → LLDP Neighbors → Interfaces →           │
│       MAC Table → ARP Table → VLANs                                  │
└──────────────────────┬───────────────────────────────────────────────┘
                       ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      TOPOLOGY ASSEMBLY                                │
│  1. Add each queried switch as a device (config name as ID)          │
│  2. Build system-name index (FQDN → config ID)                      │
│  3. For each LLDP neighbor:                                          │
│     a. Pick device ID (system-name preferred, chassis-id fallback)   │
│     b. Resolve against system-name index (switch dedup)              │
│     c. Classify device type (capabilities → description → name)      │
│     d. Create/merge device, create link                              │
│  4. Deduplicate VLANs                                                │
│  5. **ARP-Port Correlation** (host enrichment from switch data)      │
│  6. Correlate VM endpoints (MAC table − LLDP chassis IDs)            │
└──────────────────────┬───────────────────────────────────────────────┘
                       ▼
┌──────────────────────────────────────────────────────────────────────┐
│                  DEPLOYMENT ENRICHMENT (optional)                     │
│  Pass 1:   MAC match (exact → offset +2) and hostname match         │
│  Pass 1.5: Merge NIC port devices into single host nodes            │
│  Pass 2:   Rename device IDs to deployment hostnames                 │
│  Pass 3:   Synthesize hosts expected but not discovered              │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Device Identity

### How a device gets its ID

Every device in the topology needs a stable, unique ID. The ID is determined by the following priority:

| Priority | Source | Example | When used |
|----------|--------|---------|-----------|
| 1 | Config name (for queried switches) | `TOR-1` | Always — this is the user-assigned name from `config.yaml` |
| 2 | LLDP system-name (for neighbors) | `rr1-n42-r14-93180hl-8-1a` | When the neighbor reports a hostname |
| 3 | LLDP chassis-id (for neighbors) | `d894.24f2.cfb4` | Fallback when system-name is empty (common for bare-metal NICs) |
| 4 | Deployment hostname (post-enrichment) | `ASRR1N42R14U01` | Replaces MAC-based IDs after deployment matching |

### Switch deduplication

A physical switch can appear twice in the topology: once as a queried device (with its config name like `TOR-1`) and once as an LLDP neighbor reported by another switch (with its FQDN like `rr1-n42-r14-93180hl-8-1a`).

**How we prevent duplicates:**

1. When collecting system info from a queried switch, we record its FQDN (SystemName).
2. We build a `systemNameToID` index mapping FQDN → config name (e.g., `rr1-n42-r14-93180hl-8-1a` → `TOR-1`).
3. When processing LLDP neighbors, we check this index before creating a new device.
4. Non-queried switches (spines, management switches, MLAG peers not in config) keep their FQDN as-is.

**Assumption:** A switch's gNMI-reported SystemName matches the system-name it advertises via LLDP to its peers.

**Code:** `collector.go` → `buildSystemNameIndex()`, `resolveDeviceID()`

---

## Device Classification

Devices are classified into types: `switch`, `host`, `bmc`, `vm`, or `unknown`. Classification happens in multiple stages.

### Device types

| Type | Meaning | Discovery source |
|------|---------|-----------------|
| `switch` | Network switch (TOR, spine, leaf) | LLDP capabilities or system description keywords |
| `host` | Physical server | LLDP capabilities, system description keywords, ARP enrichment, or deployment JSON |
| `bmc` | Baseboard Management Controller | Name/description keywords (iDRAC, iLO, BMC, IPMI, Redfish) |
| `vm` | Virtual machine / endpoint | MAC table entries that do not match the LLDP chassis-id on a port |
| `unknown` | Unclassifiable device | No capabilities and no matching keywords |

### Stage 1: LLDP-based classification (no deployment JSON needed)

The function `ClassifyDevice(description, name, capabilities)` uses the following decision tree:

```
LLDP capabilities present?
├── YES
│   ├── Bridge or Router capability → "switch"
│   │   └── BUT name/description contains BMC hint? → "bmc"
│   ├── Station capability only → "host"
│   │   └── BUT name/description contains BMC hint? → "bmc"
│   └── Other capability → fall through to heuristics
│
└── NO (capabilities field empty or absent)
    ├── Name/description contains BMC hint? → "bmc"
    │   (hints: iDRAC, iLO, BMC, IPMI, Redfish)
    │
    ├── Description contains switch OS hint? → "switch"
    │   (hints: SONiC, NX-OS, Arista, Cumulus, FTOS, Dell EMC, Cisco)
    │
    ├── Description contains server OS hint? → "host"
    │   (hints: Linux, Ubuntu, Windows, Red Hat, CentOS, SLES)
    │
    └── None of the above → "unknown"
```

**Assumptions:**
- LLDP capabilities (IEEE 802.1AB) are the most reliable signal when present.
- Bridge+Router is the standard capability set for L2/L3 switches.
- Station-only indicates an end host.
- BMC keywords in the name or description override capabilities-based classification.

**Limitation:** Many NX-OS devices report empty system-name and empty capabilities for directly-connected NICs, leaving only the chassis-id (MAC address). These end up as `unknown` devices with MAC-based IDs.

**Code:** `transform/lldp.go` → `ClassifyDevice()`

### Stage 2: Deployment-based reclassification (requires deployment JSON — experimental)

> ⚠️ **Deployment JSON enrichment is experimental.** It has been tested against a single Azure Local deployment layout. The JSON schema may vary across versions and regions. Use for additional context, not as sole source of truth.

If a deployment JSON is provided and an `unknown` device matches a deployment host (via MAC or hostname), it is reclassified to `host`.

Devices already classified as `switch` or `bmc` are **not** reclassified by deployment matching — only `unknown` types are promoted.

### Spine vs. Leaf Switch Classification (UI only)

The web UI further classifies switches to determine their position in the visual hierarchy:

| Role | Rule | Tree position |
|------|------|--------------|
| **Spine** | All LLDP neighbors of this switch are other switches (no host/BMC/unknown neighbors) | Top row (tier 0) |
| **Leaf** | At least one LLDP neighbor is a non-switch device (host, BMC, unknown) | Second row (tier 1) |

This classification is computed **client-side** from the topology link data by `classifySwitches()` in `app.js`. It is **not** stored in the topology JSON output and is used only for layout positioning in the web UI.

**Assumption:** A switch that only connects to other switches is an aggregation/spine switch. This heuristic may misclassify management switches or out-of-band devices that happen to only connect to other switches.

**Code:** `cmd/network-mapper/web/js/app.js` → `classifySwitches()`

---

## MAC Address Correlation

### The +2 Offset Problem

**Observation:** Cisco NX-OS switches report the LLDP chassis-id for directly-connected NICs as the NIC's port MAC address **plus 2**.

| Deployment JSON NIC MAC | LLDP Chassis-ID (hex) | Difference |
|--------------------------|-----------------------|------------|
| `D8:94:24:F2:CF:B2` | `d8:94:24:f2:cf:b4` | +2 |
| `D8:94:24:F2:CF:B3` | `d8:94:24:f2:cf:b5` | +2 |
| `D8:94:24:83:A5:CE` | `d8:94:24:83:a5:d0` | +2 |

This offset was verified against 100% of NIC card 1 (ethernet + ethernet 2) connections across all hosts in a real 64-node deployment. The cause is likely that NX-OS reports the base MAC of the NIC module rather than the individual port MAC.

**Assumptions:**
- The +2 offset is consistent across all Cisco NX-OS switches.
- We have NOT verified this on SONiC or other platforms (they may report exact MACs).
- We only apply the +2 offset — not +1, +3, or other values.
- Exact MAC match always takes precedence over offset match.

**Code:** `deployment.go` → `macAddOffset()`, offset lookup in `EnrichTopology()`

### MAC Normalization

MAC addresses arrive in many formats depending on the source:

| Source | Format example |
|--------|---------------|
| Deployment JSON | `D8-94-24-F2-CF-B2` (dash-separated, uppercase) |
| LLDP chassis-id (NX-OS) | `d894.24f2.cfb4` (dot-separated Cisco notation) |
| LLDP chassis-id (SONiC) | `aa:bb:cc:dd:ee:ff` (colon-separated) |
| Internal representation | `d8:94:24:f2:cf:b2` (colon-separated, lowercase) |

All MACs are normalized to lowercase colon-separated format before comparison.

**Code:** `deployment.go` → `normalizeMAC()`, `transform/lldp.go` → `normalizeMACAddress()`

---

## NIC Port Grouping (Deployment Enrichment)

### The Problem

Each physical server in an Azure Local deployment has 4 NIC ports (2 per NIC card). Since NX-OS reports each NIC port with its own chassis-id and no system-name, each port appears as a separate `unknown` device in the topology. A 64-host deployment creates ~252 `unknown` MAC-only devices instead of 64 named hosts.

### The Solution (requires deployment JSON)

After MAC matching (Pass 1), we group devices that matched the same deployment host:

1. **Group by deployment_name**: All devices whose `deployment_name` annotation points to the same host are candidates for merging.
2. **Only MAC-based matches merge**: Devices matched by hostname alone are NOT merged (too weak a signal — could be coincidence).
3. **Pick a primary**: The device with the most links wins. Ties broken by lexically smallest ID (deterministic).
4. **Rewrite references**: All links and endpoint `HostDevice` references pointing to secondary devices are rewritten to the primary's ID.
5. **Remove secondaries**: Merged devices are deleted from the topology.

**Assumptions:**
- Two NIC ports from the same host will match the same deployment host via MAC+2 offset.
- It is safe to collapse them because they represent the same physical machine.
- The primary device's metadata (chassis-id, etc.) is sufficient — we don't union interface lists from secondaries.

**Code:** `deployment.go` → `mergeDevicesByHost()`, `pickPrimary()`

### What happens WITHOUT deployment JSON

Without the deployment JSON:
- Each NIC port remains as a separate `unknown` device with a MAC-based ID.
- The topology is still valid — all links are correct, just not human-friendly.
- You get 252 MAC-only nodes instead of 64 named hosts.
- The web UI still shows the correct physical connections.

---

## VM Endpoint Discovery

VMs are discovered by comparing the MAC table against LLDP neighbor data on each switch port:

```
For each MAC table entry on port P:
  1. Is this MAC the LLDP chassis-id of the neighbor on port P?
     → Skip (it's the physical host NIC)
  2. Is this MAC within ±2 of the LLDP chassis-id?
     → Skip (NX-OS offset — still the physical host)
  3. Is this a well-known infrastructure MAC?
     (multicast, broadcast, VRRP, HSRP, STP)
     → Skip
  4. Otherwise → This is a VM/endpoint behind the host
```

**Assumptions:**
- A MAC on a switch port that doesn't match (or nearly match) the LLDP chassis-id is a VM.
- The ±2 range around the chassis-id covers the NX-OS offset for all NIC ports.
- Infrastructure MACs are identifiable by well-known prefixes.

**Code:** `transform/endpoint.go` → `CorrelateEndpoints()`, `isHostMAC()`

---

## ARP-Port Correlation (Host Enrichment)

This enrichment pass runs **before** both deployment enrichment and endpoint correlation, giving it the ability to assign IPs to unknown devices using only data from the switches themselves (no deployment JSON needed).

### Algorithm

```
For each switch port with an LLDP neighbor:
  1. Identify the neighbor device by chassis-id
  2. Skip if device is already a switch or has a ManagementAddress
  3. Find MAC table entries on that same port
  4. Filter to "host MACs" — those within ±2 of the LLDP chassis-id
     (VM MACs on the same port are explicitly excluded)
  5. Look up filtered MACs in the ARP table to find IPs
  6. Filter out link-local (169.254.x.x), loopback, multicast, and special IPs
  7. If multiple valid IPs remain, pick the lowest numerically (deterministic)
  8. Assign the IP as ManagementAddress; reclassify device as "host"
  9. Optionally perform reverse DNS lookup (opt-in via config)
```

### Why ±2 MAC offset?

NX-OS reports LLDP chassis-id as the NIC's "base MAC" which is often +2 from the traffic MAC observed in the MAC table. The ±2 window handles:
- Offset 0: chassis-id equals the traffic MAC (some NIC types, SONiC)
- Offset +2: NX-OS NIC card 1 behavior (verified across 64+ real hosts)
- Offset -2: covers the reverse relationship

### Key constraints

| Constraint | Reason |
|-----------|--------|
| Only host MACs used | VMs behind a host share the port but shouldn't get their IP assigned to the host device |
| Link-local IPs excluded | 169.254.x.x addresses are auto-configured and not useful for identification |
| Existing ManagementAddress not overwritten | Preserves deployment-authoritative data |
| Switches never reclassified | `looksLikeSwitch()` prevents known switches from being changed to hosts |
| Deterministic IP selection | Lowest IP chosen when multiple valid IPs exist |

### Configuration

```yaml
collect:
  reverse_dns: true  # Optional: attempt rDNS lookup for assigned IPs
```

**Code:** `transform/host_enrichment.go` → `EnrichDevicesFromSwitchData()`

---

## Deployment Enrichment Passes (Experimental)

> ⚠️ **This entire section describes experimental functionality.** The deployment JSON enrichment has been tested against a limited set of Azure Local deployments. The schema and matching behavior may require adjustments for different deployment versions.

When a deployment JSON is provided, enrichment runs in 4 passes:

### Pass 1: Device matching

For each device in the topology:

| Priority | Match method | Annotation | Merge-eligible? |
|----------|-------------|------------|-----------------|
| 1 | Exact MAC match (chassis-id == NIC MAC) | `deployment_match=mac` | ✅ Yes |
| 2 | Offset MAC match (chassis-id == NIC MAC + 2) | `deployment_match=mac_offset_2` | ✅ Yes |
| 3 | Hostname match (system-name == deployment name, case-insensitive) | `deployment_match=hostname` | ❌ No |

When matched, the device receives:
- `deployment_name` annotation (the authoritative hostname)
- `deployment_source=true` annotation
- `ManagementAddress` filled from IPv4Address or BMCIPAddress (only if empty)
- `BMCIPAddress` filled (only if empty)
- Reclassification from `unknown` → `host` (only if currently `unknown`)

### Pass 1.5: NIC port merge

Groups devices by `deployment_name` where match type is `mac` or `mac_offset_2`. Merges multi-NIC-port devices into a single node. See [NIC Port Grouping](#nic-port-grouping-deployment-enrichment) above.

### Pass 2: ID rename

Devices matched to a deployment host are renamed from their MAC-based ID to the deployment hostname. All references (links, endpoints) are rewritten.

Renames are skipped if the target ID already exists (collision avoidance).

### Pass 3: Synthesize missing hosts

Deployment hosts that were not matched to any discovered device are synthesized as new devices with:
- Type: `host`
- `deployment_synthesized=true` annotation
- All known metadata from the deployment JSON
- No links (since they weren't seen on any switch)

This ensures the topology includes all expected hosts, even if some weren't reachable via LLDP.

---

## Known Limitations and Edge Cases

### NIC card 2 connections

In the real deployment analyzed, NIC card 2 (ethernet 3 + ethernet 4) connects to TOR switches NOT in the config. Their chassis-ids appear in the MAC table but not as LLDP neighbors of the queried switches. These hosts will only show 2 links (from NIC card 1) instead of 4.

**Mitigation:** Add all TOR switches to the config, including MLAG peers.

### Devices with no LLDP data

Some devices may be connected to switches but have LLDP disabled. These are invisible to the tool. The deployment JSON synthesis (Pass 3) partially addresses this for known hosts.

### Shared chassis-id

If multiple devices share the same chassis-id (e.g., virtual switches behind a NIC team), they will appear as a single device. This is inherent to LLDP's device identity model.

### MAC offset on non-Cisco platforms

The +2 offset is only verified on Cisco NX-OS. SONiC and other platforms may report exact MACs. The code tries exact match first, so this is safe — offset matching only kicks in when exact fails.

### Hostname collisions

If two different physical devices report the same system-name via LLDP, they will be merged into one device (the second one's data overwrites the first). This is unlikely in practice but possible with misconfigured devices.

---

## Annotation Reference

Annotations are key-value metadata attached to devices after enrichment:

| Annotation | Value | Meaning |
|-----------|-------|---------|
| `deployment_source` | `"true"` | Device was matched to a deployment host |
| `deployment_name` | hostname string | The authoritative name from the deployment JSON |
| `deployment_match` | `"mac"`, `"mac_offset_2"`, `"hostname"` | How the match was made |
| `deployment_synthesized` | `"true"` | Device was created from deployment data, not discovered via LLDP |
| `deployment_bmc_ip` | IP string | BMC IP address from deployment data |
