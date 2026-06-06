# Known Weaknesses in Topology Heuristics

This document tracks known limitations and weaknesses in the network-mapper's
topology discovery algorithm. These are areas where the current heuristics
produce incorrect, incomplete, or misleading results.

## Status Legend

| Status | Meaning |
|--------|---------|
| 🔴 Open | Not yet addressed |
| 🟡 Partial | Workaround exists but not fully solved |
| 🟢 Fixed | Resolved |

---

## W-001: Port-Channel → LLDP Mismatch (VM Attribution)

**Status:** 🔴 Open  
**Impact:** High — 31 VMs unattributed in Env2  
**Affected stage:** `correlateEndpoints()`

**Problem:** The MAC table reports traffic on the logical port-channel name
(e.g., `po102`) but LLDP neighbors are indexed by their physical member ports
(e.g., `Eth1/49`, `Eth1/50`). The lookup `portToHostDevice["switchID:po102"]`
returns empty because no LLDP neighbor was registered under the aggregate name.

**Consequence:** All MACs learned on port-channel interfaces have no host
attribution and land in unattributed endpoints.

**Fix approach:** Query NX-OS port-channel membership data
(`/System/intf-items/aggr-items`) to build a `port-channel → member ports`
mapping. When a MAC is learned on a port-channel, resolve it to the member
ports and inherit the LLDP neighbor from those members.

---

## W-002: Switch Self-Traffic in Endpoint List (supeth1)

**Status:** 🔴 Open  
**Impact:** Low — cosmetic, 1 false endpoint per switch  
**Affected stage:** `correlateEndpoints()`

**Problem:** The switch's own supervisor interface (`supeth1`) appears in the
MAC table. Its MAC uses a regular vendor prefix, so `isInfraMAC()` doesn't
filter it. The resulting "VM" has multiple IPs (the switch's own management
addresses) and no host.

**Consequence:** A fake endpoint with 4+ IPs appears in unattributed, confusing
operators who think it's a VM.

**Fix approach:** Add `supeth` (and similar supervisor ports like `sup-eth1`,
`mgmt0`) to an infrastructure port exclusion list in `correlateEndpoints()`.
Skip any MAC entries learned on these ports.

---

## W-003: NVE-Learned MACs Without L2RIB Resolution

**Status:** 🟡 Partial — VTEP groups exist but host mapping is incomplete  
**Impact:** Medium — 67 VMs in Env1 only attributed to VTEP, not to host  
**Affected stage:** `correlateEndpoints()` second pass

**Problem:** MACs learned on the NVE (VXLAN tunnel) interface can be mapped to
a VTEP IP via L2RIB, but the VTEP IP often doesn't match any known host's
management address. The hosts advertise their LLDP mgmt-address (e.g.,
`10.x.x.x`) but the VTEP uses a different IP (e.g., `100.x.x.x` from a
loopback).

**Consequence:** VMs are grouped by VTEP IP in `vtep_groups` but never
attributed to a specific host in `compute.hosts[].endpoints`.

**Fix approach:** Collect loopback interface IPs from switches/hosts via gNMI
and build a `loopback-IP → host` lookup. Alternatively, use the ARP table to
correlate VTEP IPs back to chassis MACs.

---

## W-004: Dual-Homed Host Deduplication Fragility

**Status:** 🟡 Partial — MAC adjacency ±2 + port-number match implemented  
**Impact:** Medium — could produce false merges or missed merges  
**Affected stage:** `mergeDualHomedHosts()`

**Problem:** Azure Local servers with 2 NICs (one per TOR) appear as two
separate devices. The merge uses MAC adjacency (±2 offset) and port-number
confirmation. This works for the current hardware (Mellanox dual-port NICs with
sequential MACs) but assumes:

1. NICs always have adjacent MACs (vendor-specific, not guaranteed)
2. Port numbers match across TORs (cabling convention, not enforced)

**Consequence:** If assumptions break (e.g., different NIC vendor, re-cabled
rack), hosts may not merge or may merge incorrectly.

**Fix approach:** A more robust signal would be the host's LLDP system-name
(both NICs of the same server advertise the same hostname). Currently this
works when LLDP is present (Env1) but not when hosts are silent (Env2 ports
without LLDP where we rely on the heuristic).

---

## W-005: "Switched-Compute" / "Switched-Storage" Grouping

**Status:** 🟡 Partial — ports are now discovered but grouped under a shared ID  
**Impact:** Low-Medium — misleading single "host" for multiple physical servers  
**Affected stage:** `discoverHostsFromDescriptions()`

**Problem:** When multiple ports share the same description (e.g.,
"Switched-Compute") and have no LLDP data, `discoverHostsFromDescriptions()`
uses the description as the device ID. All such ports get grouped under one
logical "host" named "Switched-Compute" rather than individual servers.

**Consequence:** The UI shows one host called "Switched-Compute" connected to
15 ports, instead of 15 individual hosts (one per port). This is technically
correct (we can't distinguish them) but visually misleading.

**Fix approach:** When the description is shared across many ports, create
per-port synthetic IDs (e.g., `Switched-Compute@Eth1/1`) to represent each
as a distinct unnamed device. Alternatively, display these as "N unidentified
hosts on ports X-Y" rather than a single named device.

---

## W-006: \u0002 Control Character in Host IDs

**Status:** 🟢 Fixed  
**Impact:** Medium — caused duplicate host entries in Env2  
**Affected stage:** `ingestLLDPNeighbors()` (LLDP system-name parsing)

**Problem:** One TOR appends a `\x02` (STX) control character to LLDP
system-name values. This creates two deviceMap keys for the same host
(e.g., `asrr1n22r04u12` vs `asrr1n22r04u12\x02`), bypassing dedup.

**Fix:** Added `SanitizeIdentifier()` to strip bytes < 0x20 and 0x7F.
Applied at:
- Parser level: all three LLDP parser paths (OpenConfig, flat-leaf, NX-OS)
  sanitize SystemName and PortID (`transform/lldp.go`)
- Builder level: sanitizes `remoteID` (system-name), ChassisID fallback,
  device struct fields, and linkInfo fields (`builder/builder.go` lines
  231-280). This was the critical fix — the parser-level sanitization alone
  was insufficient because when SystemName is empty, the builder falls back
  to unsanitized ChassisID.

---

## W-007: VLAN "Traffic Observed" Sparsity

**Status:** 🔴 Open (documented in UI, not a code bug)  
**Impact:** Low — confusing but not incorrect  
**Affected stage:** `buildVLANs()`

**Problem:** The `observed_vlans` field (populated from MAC table) is very
sparse on some switches. In Env2, only 2 out of 27 VLANs show any MAC
activity. This makes the "Traffic Observed" column appear empty for most ports,
even though traffic is definitely flowing.

**Consequence:** Users may think ports are misconfigured when they see "no
traffic observed" despite the port being in trunk mode with 20+ allowed VLANs.

**Root cause:** The NX-OS MAC table query may return a limited snapshot (e.g.,
only dynamic entries, aged-out MACs not included). The MAC table is also per-
VLAN on NX-OS and our current query may not enumerate all VLAN instances.

**Fix approach:** Query the MAC table per-VLAN if the global query returns
sparse results, or collect `show mac address-table count` for a more accurate
picture. Alternatively, clearly label the column as "recently active" in the UI.

---

## W-008: Physical Port Heuristic Over-Promotion

**Status:** 🟡 Partial — heuristic only runs on Ethernet ports with UP status  
**Impact:** Low — could promote infrastructure devices to "host"  
**Affected stage:** `promoteUnknownOnPhysicalPorts()`

**Problem:** The last-resort heuristic promotes any "unknown" device connected
to an UP physical Ethernet port to "host". This assumes that anything on a
physical port that isn't a switch or BMC must be a server. Could be wrong for:

- Firewalls, load balancers, or other appliances connected to leaf ports
- Console servers or out-of-band management gear
- Lab equipment during testing

**Consequence:** Non-host devices get green "host" icons in the UI.

**Mitigation:** The heuristic adds `classification_source: physical_port_heuristic`
annotation so operators can identify which hosts were inferred vs. confirmed.

---

## Discovery Pipeline Stages Reference

For context, the build pipeline runs these stages in order:

1. `ingestSwitches()` — config-defined switches
2. `ingestLLDPNeighbors()` — LLDP → devices + links
3. `discoverHostsFromDescriptions()` — UP ports without LLDP
4. `enrichHosts()` — MAC+ARP → IP assignment
5. `mergeDualHomedHosts()` — combine dual-NIC servers
6. `promoteUnknownOnPhysicalPorts()` — last-resort classification
7. `correlateEndpoints()` — VM discovery from MAC table
8. `buildVLANs()` — VLAN membership from MAC entries
9. `assemble()` — build final TopologyV2 output

---

## Architectural Exploration: NIC-Centric vs. Host-Centric Model

### Current model: Host-centric

The current algorithm groups multiple LLDP neighbors into a single "host" device.
This grouping relies on `mergeDualHomedHosts()` which uses MAC adjacency (±2) and
port-number confirmation to decide that two separate LLDP entries represent the
same physical server.

**The host construct is our invention** — LLDP only tells us about individual NICs.
We *infer* that NIC `58:a2:e1:9d:8a:86` on TOR-A port 15 and NIC `58:a2:e1:9d:8a:87`
on TOR-B port 15 belong to the same server because:
1. Their MACs differ by 1 (adjacent Mellanox ports on the same card)
2. They're on the same port number across two different TORs
3. Both NICs advertise the same LLDP system-name (when available)

### Alternative model: NIC-centric (flat)

Instead of merging, we could treat **each LLDP neighbor as a separate NIC entity**:

```
Current (host-centric):
  Host "asrr1n22r04u15" → 4 connections (2 NICs × 2 TORs)
    ├── NIC A port 1 → TOR-A Eth1/15
    ├── NIC A port 2 → TOR-A Eth1/35
    ├── NIC B port 1 → TOR-B Eth1/15
    └── NIC B port 2 → TOR-B Eth1/35

Alternative (NIC-centric):
  NIC "58:a2:e1:9d:8a:86" → TOR-A Eth1/15
  NIC "58:a2:e1:9d:8a:87" → TOR-A Eth1/35
  NIC "a0:88:c2:9a:a6:ba" → TOR-B Eth1/15
  NIC "a0:88:c2:9a:a6:bb" → TOR-B Eth1/35
```

### Tradeoff Analysis

| Dimension | Host-centric (current) | NIC-centric (alternative) |
|-----------|----------------------|--------------------------|
| **Accuracy** | Can produce false merges (W-004) or false splits. Relies on vendor-specific MAC patterns. | 100% accurate — what LLDP reports is what we show. Zero inference. |
| **Human readability** | Operators think in servers, not NICs. "Host X has 4 links" is intuitive. | Operators must mentally group NICs → servers. 56 NICs is harder to scan than 14 hosts. |
| **VM attribution** | VMs are attributed to a host (the server running them). Port-channel → member port resolution needed once. | VMs would be attributed to a specific NIC (or port). More accurate per-path visibility. |
| **Dual-homed visibility** | Explicitly shows all paths. A host with <4 connections signals a cabling issue. | Dual-homing is implicit — operator must notice matching port numbers across TORs. |
| **Scalability** | 14 entities in Env2 (14 hosts). | 56 entities in Env2 (56 NICs). Quadruples inventory list. |
| **Correctness under edge cases** | Breaks when MAC adjacency assumption fails, when NICs are from different vendors, or when cabling is asymmetric (W-004). | Never breaks — no assumptions about hardware. |
| **Host identity for ops** | Can label "asrr1n22r04u15" from LLDP system-name. Useful for rack operations. | Still shows system-name per NIC (all NICs of the same host share it). Could be grouped client-side. |

### Impact on specific weaknesses

| Weakness | Effect of NIC-centric model |
|----------|---------------------------|
| **W-001** (Port-channel mismatch) | Unchanged — still need LAG→member mapping for VM attribution. |
| **W-004** (Dual-homed fragility) | **Eliminated entirely** — no merging, no fragility. |
| **W-005** (Switched-Compute grouping) | **Eliminated** — each port is its own NIC entity. No over-grouping. |
| **W-008** (Over-promotion) | Reduced — NICs don't need classification, they just exist as LLDP neighbors. |

### Recommendation: Hybrid approach

A purely NIC-centric model sacrifices operator usability for correctness. A better
path may be a **two-layer model**:

1. **Data layer (NIC-centric):** Every LLDP neighbor is stored as an individual NIC
   entity with its MAC, IP, switch port, and system-name. No merging, no inference.
   This is the ground truth.

2. **Presentation layer (host grouping):** NICs are grouped into hosts **client-side**
   using the most reliable signal available:
   - LLDP system-name match (deterministic, hardware-independent)
   - Manual override via annotations when system-name is absent

   If system-name is empty and no manual grouping is provided, the NIC stays
   ungrouped — displayed as an individual entity rather than incorrectly merged.

This approach eliminates W-004 and W-005 entirely, preserves the operator-friendly
host view for the common case (LLDP system-name is present in most Azure Local
deployments), and degrades gracefully (ungrouped NICs) rather than incorrectly
(false merges).

### Implementation complexity

Moving to the hybrid model would require:
- Removing `mergeDualHomedHosts()` from the build pipeline
- Changing `ComputeHost.Connections` to be the primary data (each NIC = one connection)
- Adding a `system_name` field to each connection for client-side grouping
- UI change: group NIC cards by system-name header when system-name is non-empty
- Backward-compatible JSON: hosts array becomes NICs array, grouped by system-name

**Risk:** Low — the current `HostConnection` already carries per-NIC data (MAC,
switch port, speed, MTU). The merge step is purely a builder-level aggregation
that could be moved to the UI layer without data loss.

**Effort:** Medium — mostly UI refactoring. Backend change is removing code (simpler).
