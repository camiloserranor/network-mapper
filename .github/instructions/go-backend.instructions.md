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
- When adding a new vendor or data category, follow the existing pattern:
  1. Add a `ParseXxxVendor()` function in the appropriate transform file.
  2. Add the collection call in `collector.go`, branching on `platform`.
  3. Update the builder if the new data type affects the v2 schema.
- Helper functions in `internal/transform/helpers.go` handle safe JSON traversal (`GetString`, `GetMap`, `GetSlice`, etc.) — use them instead of raw type assertions.
- Avoid import cycles: `collector` → `topology`/`transform` (OK), `builder` → `collector`/`topology`/`transform` (OK), but `collector` must NOT import `builder`. Use callbacks (like `BuildFunc`) to decouple.
