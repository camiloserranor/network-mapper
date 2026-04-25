---
applyTo: "internal/**/*.go"
---

# Go Backend Instructions

- All packages under `internal/` are private to this module.
- Follow the existing pattern: each package has a clear single responsibility.
- Use the `topology.Device`, `topology.Interface`, and `topology.Link` types as the canonical data model — do not create parallel structs.
- Transform functions (in `internal/transform/`) should be pure: take raw gNMI JSON data, return typed structs. No side effects or network calls.
- The collector (`internal/collector/`) is the only package that orchestrates gNMI calls. Keep network I/O there.
- When adding a new vendor or data category, follow the existing pattern:
  1. Add a `ParseXxxVendor()` function in the appropriate transform file.
  2. Add the collection call in `collector.go`, branching on `platform`.
- Helper functions in `internal/transform/helpers.go` handle safe JSON traversal (`GetString`, `GetMap`, `GetSlice`, etc.) — use them instead of raw type assertions.
