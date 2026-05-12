# Topology V2 Schema

The topology JSON file is the primary output of the network-mapper collection pipeline. It provides a hierarchical, human-readable representation of the network fabric that can be consumed by:

- **Operators** inspecting switch configurations and LLDP neighbors
- **AI agents** analyzing network topology and diagnosing issues
- **The network-mapper web UI** for interactive visualization

## Schema Version

```json
{ "schema_version": "2.0" }
```

## Top-Level Structure

```
topology-v2.json
├── schema_version    — Always "2.0"
├── metadata          — Collection context and summary statistics
├── fabric            — Network fabric: TOR switches, interfaces, BGP, inter-switch links
├── compute           — Compute layer: hosts, VMs, and unattributed endpoints
├── vlans             — Cross-fabric VLAN view with per-switch port assignments
├── unknown_devices   — LLDP-discovered devices that couldn't be classified
└── warnings          — Non-fatal errors from the collection process
```

---

## `metadata`

| Field | Type | Description |
|-------|------|-------------|
| `collected_at` | string (ISO 8601) | When this collection was performed |
| `tool` | string | Always `"network-mapper"` |
| `tool_version` | string | Version of the tool (e.g., `"0.3.0"`) |
| `source_switches` | string[] | Config-level IDs of switches that were queried |
| `summary` | object | At-a-glance counts (see below) |

### `metadata.summary`

| Field | Type | Description |
|-------|------|-------------|
| `switch_count` | int | Number of fabric switches |
| `host_count` | int | Number of classified hosts |
| `endpoint_count` | int | Total VM/container endpoints |
| `unknown_device_count` | int | Unclassified LLDP neighbors |
| `total_links` | int | Total inter-device links |
| `inter_switch_links` | int | Switch-to-switch links |
| `host_links` | int | Switch-to-host links |
| `vlan_count` | int | Number of VLANs discovered |
| `partial_failures` | int | Non-fatal errors |
| `attributed_endpoints` | int | VMs mapped to a specific host |
| `unattributed_endpoints` | int | VMs that couldn't be mapped to a host |

---

## `fabric`

Contains an array of `switches`, each representing a TOR or spine switch that was directly queried via gNMI or discovered via LLDP.

### `fabric.switches[]`

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier (config name or FQDN) |
| `name` | string | System hostname |
| `chassis_id` | string | LLDP chassis ID (MAC address) |
| `management_address` | string | Management IP address |
| `software_version` | string | NX-OS/SONiC version string |
| `system_description` | string | Full system description from LLDP/gNMI |
| `uptime` | string | System uptime |
| `health` | object? | CPU/memory utilization (if available) |
| `interfaces` | Interface[] | All interfaces on this switch |
| `bgp_sessions` | BGPSession[] | BGP peering sessions |
| `peer_links` | PeerLink[] | Inter-switch connections (switch↔switch) |
| `connected_hosts` | ConnectedHost[] | Hosts attached to this switch |
| `annotations` | map<string,string> | Arbitrary metadata |

### `health`

| Field | Type | Description |
|-------|------|-------------|
| `cpu_utilization_pct` | float | CPU usage percentage |
| `memory_used_bytes` | int | Memory in use |
| `memory_total_bytes` | int | Total memory |

### `interfaces[]` (on switch)

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Interface name (e.g., `Ethernet1/1`) |
| `oper_status` | string | `UP` or `DOWN` |
| `speed` | string | `1G`, `10G`, `25G`, `100G`, etc. |
| `mtu` | int | MTU in bytes |
| `mode` | string | `access`, `trunk`, or `routed` |
| `access_vlan` | int | Configured access VLAN |
| `native_vlan` | int | Configured native VLAN (trunk) |
| `trunk_vlans` | int[] | Allowed trunk VLANs |
| `observed_vlans` | int[] | VLANs with active MAC traffic |

### `bgp_sessions[]`

| Field | Type | Description |
|-------|------|-------------|
| `neighbor_address` | string | BGP peer IP |
| `peer_as` | int | Remote AS number |
| `local_as` | int | Local AS number |
| `peer_type` | string | `INTERNAL` or `EXTERNAL` |
| `session_state` | string | `ESTABLISHED`, `IDLE`, `ACTIVE`, etc. |
| `enabled` | bool | Whether the session is admin-enabled |
| `vrf_name` | string | VRF name |

### `peer_links[]`

| Field | Type | Description |
|-------|------|-------------|
| `local_port` | string | Local interface name |
| `remote_switch` | string | Remote switch ID |
| `remote_port` | string | Remote interface name |
| `oper_status` | string | `UP` or `DOWN` |
| `speed` | string | Link speed |
| `mtu` | string | MTU |

### `connected_hosts[]`

| Field | Type | Description |
|-------|------|-------------|
| `port` | string | Switch port the host is on |
| `host_id` | string | Host device ID |
| `host_mgmt_ip` | string | Host management IP |
| `oper_status` | string | Port operational status |
| `mtu` | string | Port MTU |

---

## `compute`

### `compute.hosts[]`

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique host identifier |
| `chassis_id` | string | LLDP chassis ID (MAC) |
| `name` | string | Hostname (from LLDP or rDNS) |
| `management_address` | string | Management IP |
| `classification_source` | string | How the host was identified |
| `connections` | HostConnection[] | Switch connections |
| `endpoints` | HostEndpoint[] | VMs/containers on this host |
| `annotations` | map<string,string> | Arbitrary metadata |

### `connections[]` (on host)

| Field | Type | Description |
|-------|------|-------------|
| `switch_name` | string | Connected switch name |
| `switch_id` | string | Connected switch ID |
| `switch_port` | string | Port on the switch |
| `oper_status` | string | Port status |
| `speed` | string | Link speed |
| `mtu` | string | MTU |
| `vlan_mode` | string | `access`, `trunk`, or `routed` |
| `access_vlan` | int | Access VLAN ID |
| `native_vlan` | int | Native VLAN ID |
| `trunk_vlans` | int[] | Trunk allowed VLANs |

### `compute.unattributed_endpoints`

Endpoints (VMs) discovered via MAC/ARP tables that could not be mapped to a specific physical host. This is common for NVE-learned entries.

| Field | Type | Description |
|-------|------|-------------|
| `count` | int | Number of unattributed endpoints |
| `items` | HostEndpoint[] | The endpoint records |

### `HostEndpoint` (used in both hosts and unattributed)

| Field | Type | Description |
|-------|------|-------------|
| `mac` | string | MAC address |
| `ips` | string[] | IP addresses (from ARP) |
| `vlans` | int[] | VLAN memberships |
| `type` | string | `vm`, `container`, `floating`, `unknown` |
| `learned_on_switch` | string | Switch that reported this MAC |
| `learned_on_port` | string | Port where MAC was learned |

---

## `vlans`

Network-wide VLAN cross-reference. For each VLAN, shows which switches carry it and on which ports.

### `vlans.items[]`

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | VLAN ID |
| `switches` | VLANSwitch[] | Switches carrying this VLAN |
| `hosts` | VLANHost[] | Hosts in this VLAN |

### `VLANSwitch`

| Field | Type | Description |
|-------|------|-------------|
| `switch_name` | string | Switch ID |
| `access_ports` | string[] | Ports in access mode for this VLAN |
| `trunk_ports` | string[] | Ports trunking this VLAN |

### `VLANHost`

| Field | Type | Description |
|-------|------|-------------|
| `chassis_id` | string | Host chassis ID |
| `management_ip` | string | Host management IP |
| `switch_port` | string | Switch port connecting the host |

---

## `unknown_devices`

Devices discovered via LLDP that could not be classified as switch or host.

### `unknown_devices.items[]`

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Device identifier |
| `chassis_id` | string | LLDP chassis ID |
| `management_address` | string | Management IP (if available) |
| `system_description` | string | LLDP system description |
| `connected_to` | DeviceAttachment[] | Switch ports this device is seen on |

### `DeviceAttachment`

| Field | Type | Description |
|-------|------|-------------|
| `switch` | string | Switch ID |
| `port` | string | Switch port |
| `oper_status` | string | Port status |
| `mtu` | string | Port MTU |

---

## `warnings`

Array of non-fatal errors encountered during collection.

| Field | Type | Description |
|-------|------|-------------|
| `switch` | string | Switch that produced the error |
| `phase` | string | Collection phase (`connect`, `lldp`, `interfaces`, `mac-table`, etc.) |
| `message` | string | Error message |

---

## Example

See [`examples/topology-v2-sample.json`](../examples/topology-v2-sample.json) for a full example generated from a real 6-switch Azure Local deployment.

## Data Pipeline

```
gNMI Switches → collector.CollectRaw() → CollectionResult
                                              ↓
                                      builder.Build() → TopologyV2 (this schema)
                                              ↓
                                      server.Serve() → Web UI / API consumers
```

Each stage can be independently tested with saved data. The `collect` CLI command writes the v2 JSON directly to disk.
