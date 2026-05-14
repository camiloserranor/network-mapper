# Data Collection & Multi-Vendor Architecture

Network Mapper uses gNMI (gRPC Network Management Interface) to collect topology data from TOR switches. The collection pipeline supports multiple switch platforms through vendor-specific gNMI path mappings.

---

## Multi-Vendor Support

The tool uses a `platform` field in the switch configuration to select the correct gNMI paths and encoding for each switch. This allows a single topology collection to span switches from different vendors.

### Supported Platform Values

| Platform Value | Vendor | gNMI Encoding | Path Style |
|---|---|---|---|
| `nxos` | Cisco NX-OS | JSON | Native NX-OS paths (`/System/...`) |
| `sonic` | SONiC | JSON_IETF | OpenConfig (`/openconfig-lldp:lldp/...`) |
| `dell-os10` | Dell OS10 | JSON_IETF | OpenConfig (same as SONiC) |

> **Note:** Cisco NX-OS is the primary tested platform. Other OpenConfig-compatible platforms are supported by the architecture but may require additional validation.

### How Platform Selection Works

When the collector processes a switch, it branches on the `platform` field to determine:

1. **gNMI paths** — NX-OS uses native Cisco YANG paths; OpenConfig platforms use standard OpenConfig paths.
2. **Encoding** — NX-OS uses `JSON`; OpenConfig platforms use `JSON_IETF`.
3. **Response parsing** — Each platform has dedicated transform functions (e.g., `ParseLLDPNXOS` vs. `ParseLLDPOpenConfig`).
4. **Get vs. Subscribe** — Some platforms may return empty responses for bulk Get on list paths; the client falls back to Subscribe ONCE automatically.

### gNMI Paths by Platform

| Data Category | NX-OS Path | OpenConfig Path |
|---|---|---|
| LLDP Neighbors | `/System/lldp-items/inst-items/if-items/If-list` | `/openconfig-lldp:lldp/interfaces/interface/neighbors` |
| Interfaces | `/System/intf-items/phys-items/PhysIf-list` | `/openconfig-interfaces:interfaces/interface` |
| System Info | `/System/` | `/openconfig-system:system/state` |

---

## Configuration

Set the `platform` field on each switch entry in your config YAML:

```yaml
switches:
  - name: TOR-1
    address: "10.0.0.1:50051"
    platform: nxos          # Cisco NX-OS
    auth:
      username_keyvault: https://myvault.vault.azure.net/secrets/gnmi-username
      password_keyvault: https://myvault.vault.azure.net/secrets/gnmi-password

  - name: TOR-2
    address: "10.0.0.2:50051"
    platform: dell-os10     # Dell OS10 (OpenConfig)
    auth:
      username_keyvault: https://myvault.vault.azure.net/secrets/gnmi-username
      password_keyvault: https://myvault.vault.azure.net/secrets/gnmi-password
```

### Example Config Files

| File | Description |
|---|---|
| `examples/config.yaml` | NX-OS switches (primary example) |
| `examples/config-sonic.yaml` | OpenConfig platforms (SONiC / Dell OS10) |

---

## Collection Pipeline

For each configured switch (in parallel up to `collect.parallel`):

1. **Connect** — Establish gNMI session (gRPC + TLS with TOFU or skip-verify)
2. **Authenticate** — Username/password sent as gRPC metadata
3. **Collect System Info** — Query hostname, software version
4. **Collect LLDP Neighbors** — Query neighbor tables using platform-specific paths
5. **Collect Interfaces** — Query interface state, speed, counters
6. **Parse & Normalize** — Platform-specific parsers produce unified `topology.Device` / `topology.Link` types
7. **Merge** — Results from all switches are merged into a single topology graph

The normalized output is the same regardless of which platform the data came from — downstream consumers (web UI, JSON export) are vendor-agnostic.
