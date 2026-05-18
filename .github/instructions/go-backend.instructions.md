---
applyTo: "internal/**/*.go"
---

# Go Backend Instructions

- All packages under `internal/` are private to this module.
- Follow the existing pattern: each package has a clear single responsibility.
- The data pipeline has 3 stages:
  1. **Collector** (`internal/collector/`) — gNMI calls → `CollectionResult` (raw per-switch data)
  2. **Builder** (`internal/builder/`) — `CollectionResult` → `TopologyV2` (hierarchical topology)
  3. **Server** (`internal/server/`) — serves `TopologyV2` JSON to the web UI
- Use `topology.TopologyV2` types for the v2 hierarchical output schema. Legacy `topology.Topology` (v1) types are still used internally.
- Transform functions (in `internal/transform/`) should be pure: take raw gNMI JSON data, return typed structs. No side effects or network calls.
- The builder (`internal/builder/`) must also be pure: no I/O, no config, no DNS — just data transformation.
- The collector (`internal/collector/`) is the only package that orchestrates gNMI calls. Keep network I/O there.

## Platform Strategy Pattern

Vendor-specific gNMI behavior is encapsulated in `internal/platform/`:

```
internal/platform/
├── platform.go        # Platform interface + ForPlatform() registry
├── nxos.go            # Cisco NX-OS (native /System/ paths)
├── sonic.go           # Dell Enterprise SONiC (OpenConfig + flat-leaf quirks)
└── openconfig.go      # Generic OpenConfig fallback (future vendors)
```

The `Platform` interface defines one method per data category (CollectSystem, CollectLLDP, CollectInterfaces, etc.). Each implementation encapsulates:
- **Path selection** — which gNMI YANG paths to query
- **Fetch strategy** — Get vs GetWithFallback (Subscribe ONCE)
- **Parser dispatch** — which transform function to call
- **Vendor quirks** — skip counters for SONiC, NX-OS native paths, etc.

### Adding a New Vendor

1. Create `internal/platform/newvendor.go` implementing the `Platform` interface.
2. Register it in `ForPlatform()` in `platform.go`.
3. Add any new parsers needed in `internal/transform/`.
4. No changes to the collector are needed.

### Key design principles:
- The collector calls `platform.ForPlatform(sw.Platform)` and uses the interface — no platform `if/switch` statements.
- Transform functions remain pure and testable (no I/O).
- Platform implementations are the only place that knows both "which path" and "which parser".

## Other Conventions

- Helper functions in `internal/transform/helpers.go` handle safe JSON traversal (`GetString`, `GetMap`, `GetSlice`, etc.) — use them instead of raw type assertions.
- Avoid import cycles: `collector` → `platform`/`topology`/`transform` (OK), `platform` → `gnmi`/`topology`/`transform` (OK), `builder` → `collector`/`topology`/`transform` (OK), but `collector` must NOT import `builder`.
- Error handling: return `error`, never panic in library code. Use `fmt.Errorf` with `%w` for wrapping.
- Prefer table-driven tests.
- Exported types and functions must have doc comments.
