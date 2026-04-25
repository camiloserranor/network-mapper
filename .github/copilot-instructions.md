# Copilot Instructions for network-mapper

## Project Overview

network-mapper is a Go CLI tool that discovers the physical topology of Azure Local deployments by querying TOR (Top-of-Rack) switches via gNMI to extract LLDP neighbor data. It produces a topology graph (JSON) and serves an interactive web UI for visualization.

## Architecture

- **CLI**: Built with `cobra`. Entry point at `cmd/network-mapper/main.go`.
- **Web UI**: Vanilla HTML/CSS/JS with vendored Cytoscape.js, embedded into the Go binary via `go:embed`. No frontend build toolchain.
- **gNMI collection pipeline**: Connects to TOR switches, collects LLDP + interface + system data, and assembles a unified topology.
- **Config**: YAML-based with `${ENV_VAR}` resolution for secrets. See `examples/config.yaml`.

### Key packages

| Package | Path | Purpose |
|---------|------|---------|
| main | `cmd/network-mapper/` | CLI commands: `serve`, `collect`, `test-connection` |
| server | `internal/server/` | HTTP server, REST API (`/api/topology`, `/api/health`) |
| collector | `internal/collector/` | Orchestrates parallel gNMI collection across switches |
| gnmi | `internal/gnmi/` | gNMI client, TLS/TOFU, auth via gRPC metadata |
| transform | `internal/transform/` | Parses gNMI responses (LLDP, interfaces, system) into topology types |
| config | `internal/config/` | YAML config loading, validation, env-var substitution |
| topology | `internal/topology/` | Core data types: Device, Interface, Link, Topology |

## Coding Conventions

### Go

- Use Go 1.26+.
- Follow standard Go project layout: `cmd/` for binaries, `internal/` for private packages.
- Error handling: return `error`, never panic in library code. Use `fmt.Errorf` with `%w` for wrapping.
- Use `log.Printf` for operational logging (not a logging framework).
- Prefer table-driven tests.
- Exported types and functions must have doc comments.

### Web UI (JavaScript)

- Vanilla JS only — no frameworks, no npm, no build step.
- Libraries are vendored in `cmd/network-mapper/web/lib/`.
- JS is organized into modules: `graph.js`, `sidebar.js`, `toolbar.js`, `popup.js`, `app.js`.
- Follow the existing dark theme aesthetic (bg `#1a1a2e`, accent `#e94560`).

## Multi-Vendor gNMI Handling

The tool supports both **SONiC** and **Cisco NX-OS** switches. Key differences:

| Aspect | SONiC | NX-OS |
|--------|-------|-------|
| Encoding | `JSON_IETF` | `JSON` |
| LLDP path | `/openconfig-lldp:lldp/interfaces/interface/neighbors` | `/System/lldp-items/inst-items/if-items/If-list` |
| Interface path | `/openconfig-interfaces:interfaces/interface` | (native paths, TBD) |
| Get behavior | Bulk Get on list paths may return empty → use Subscribe ONCE fallback | Standard Get works |

When adding support for new data categories or vendors:
1. Add a parser function in `internal/transform/` (e.g., `ParseLLDPNXOS`).
2. Update the collector to call the right parser based on `platform` config.
3. Use `GetWithFallback` in the gNMI client when SONiC may need Subscribe ONCE.

## gNMI Connection Details

- Auth: username/password via gRPC metadata (`metadata.Pairs("username", ..., "password", ...)`).
- TLS: supports TOFU (Trust-On-First-Use), skip-verify, or explicit CA certificate.
- gRPC settings: 64 MB max message size, 30s keepalive.
- SONiC JSON_IETF responses require stripping the module prefix (e.g., `openconfig-lldp:` → removed).

## go:embed Constraint

The `//go:embed` directive in `cmd/network-mapper/main.go` only works with paths relative to that file's directory. The `web/` folder **must** live under `cmd/network-mapper/`. Do not move it elsewhere.

## Configuration

Config files are YAML. Environment variables are referenced as `${VAR_NAME}` and resolved at load time. Example:

```yaml
switches:
  - name: tor-1
    address: ${TOR_SWITCH_ADDRESS}
    platform: nxos
    auth:
      username: ${SWITCH_USER}
      password: ${SWITCH_PASS}
    tls:
      skip_verify: true
```

## Building and Running

```bash
# Build
go build -o network-mapper.exe ./cmd/network-mapper

# Test gNMI connectivity
./network-mapper test-connection --config examples/config.yaml

# Collect topology from switches
./network-mapper collect --config examples/config.yaml --output topology.json

# Serve the web UI
./network-mapper serve --data topology.json --port 8080
```

## Testing

- Run tests with `go test ./...`.
- Test data files go in `testdata/` at the repo root.
- When writing tests for gNMI transform functions, use raw JSON samples from real switch output.
