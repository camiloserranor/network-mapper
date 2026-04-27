# Network Mapper — Cheat Sheet

## Build

```powershell
go build -o network-mapper.exe ./cmd/network-mapper/
```

## Serve (Static Mode)

Serve a pre-collected topology JSON file:

```powershell
.\network-mapper.exe serve -t topology-collected.json -p 8085
```

| Flag | Description | Default |
|------|-------------|---------|
| `-t` | Path to topology JSON file | `topology.json` |
| `-p` | HTTP port | `8080` |
| `--no-open` | Don't auto-open browser | `false` |

## Serve (Live Mode)

Collect from switches periodically and push updates via WebSocket:

```powershell
.\network-mapper.exe serve -c examples/config.test.yaml --interval 15 -p 8085 --no-open
```

| Flag | Description | Default |
|------|-------------|---------|
| `-c` | Config file (enables live mode) | *(none)* |
| `--interval` | Collection interval in seconds | `30` |

## Collect (One-Shot)

Collect topology once and write to a JSON file:

```powershell
.\network-mapper.exe collect -c examples/config.test.yaml -o topology-collected.json
```

| Flag | Description | Default |
|------|-------------|---------|
| `-c` | Config file path | `config.yaml` |
| `-o` | Output JSON file | `topology.json` |

## Test Connection

Verify gNMI connectivity to a single switch:

```powershell
.\network-mapper.exe test-connection `
  -a $env:CISCO_SWITCH_2:50051 `
  -u $env:CISCO_SWITCH_USER `
  -p $env:CISCO_SWITCH_PASS `
  --platform nxos
```

| Flag | Description | Default |
|------|-------------|---------|
| `-a` | Switch address (`host:port`) | *(required)* |
| `-u` | Username | *(empty)* |
| `-p` | Password (or `$env:SWITCH_PASSWORD`) | *(empty)* |
| `--platform` | `sonic` or `nxos` | `nxos` |
| `--skip-verify` | Skip TLS cert verification | `true` |
| `--timeout` | Connection timeout (seconds) | `10` |

## Demo Topology

Serve the rich demo file (19 devices, 44 links, DOWN links, counters):

```powershell
.\network-mapper.exe serve -t examples/demo-topology.json -p 8085
```

## Stop the Server

Press `Ctrl+C` in the terminal running the server.

## Profiling

Start with profiling endpoints enabled:

```powershell
.\network-mapper.exe serve -c examples/config.test.yaml --profile -p 8085 --no-open
```

| Endpoint | Description |
|---|---|
| `/api/metrics` | Runtime stats (heap, goroutines, GC, uptime) |
| `/debug/pprof/` | pprof index (requires `--profile`) |
| `/debug/pprof/heap` | Heap profile |
| `/debug/pprof/profile?seconds=30` | CPU profile |

```powershell
# Quick metrics check
curl -s http://localhost:8085/api/metrics | jq

# CPU profile analysis
go tool pprof http://localhost:8085/debug/pprof/profile?seconds=30
```

## Docker

```powershell
# Build image
docker build -t network-mapper .

# Run with config
docker run -p 8080:8080 -v ${PWD}/config.yaml:/etc/network-mapper/config.yaml:ro network-mapper

# Or use docker-compose
docker compose up
```

## Environment Variables

```powershell
$env:CISCO_SWITCH_2 = "10.218.191.143"
$env:CISCO_SWITCH_USER = "your-username"
$env:CISCO_SWITCH_PASS = "your-password"
```

## Quick Workflows

### Full rebuild + serve live

```powershell
go build -o network-mapper.exe ./cmd/network-mapper/ && `
.\network-mapper.exe serve -c examples/config.test.yaml --interval 15 -p 8085 --no-open
```

### Collect + serve static

```powershell
.\network-mapper.exe collect -c examples/config.test.yaml -o topology-collected.json && `
.\network-mapper.exe serve -t topology-collected.json -p 8085
```

### Simulate topology change on NX-OS switch

```
! SSH into switch, then:
configure terminal
interface Ethernet1/48
  shutdown          ! link disappears from UI within --interval seconds
  no shutdown       ! link reappears
end
```
