# Performance Benchmarks

## Overview

This document captures performance measurements of the network-mapper tool, both for the agent process itself and for the impact on target switches.

## How to Profile

### Agent-Side Profiling

Start the server with the `--profile` flag to enable Go pprof endpoints:

```bash
network-mapper serve --config config.yaml --profile --port 8080
```

Available endpoints:
- `http://localhost:8080/debug/pprof/` — Index page
- `http://localhost:8080/debug/pprof/heap` — Heap profile
- `http://localhost:8080/debug/pprof/goroutine` — Goroutine stacks
- `http://localhost:8080/debug/pprof/profile?seconds=30` — 30-second CPU profile

Use `go tool pprof` to analyze:

```bash
# CPU profile
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:8080/debug/pprof/heap
```

### Runtime Metrics

The `/api/metrics` endpoint returns runtime stats:

```bash
curl -s http://localhost:8080/api/metrics | jq
```

Fields returned:
| Field | Description |
|---|---|
| `uptime_seconds` | Time since server start |
| `goroutines` | Active goroutine count |
| `heap_alloc_bytes` | Current heap allocation |
| `heap_sys_bytes` | Total heap memory obtained from OS |
| `heap_objects` | Number of live heap objects |
| `gc_cycles` | Total garbage collection cycles |
| `gc_pause_total_ns` | Cumulative GC pause time |
| `ws_clients` | Connected WebSocket clients |

### Switch-Side Metrics

When collecting from switches, the tool queries CPU and memory utilization via gNMI:
- **NX-OS**: `/System/procsys-items/syscpusummary-items` and `/System/procsys-items/sysmem-items`
- **OpenConfig (SONiC)**: `/openconfig-system:system/cpus/cpu` and `/openconfig-system:system/memory/state`

These metrics are included in the topology output as `cpu_utilization`, `memory_used`, and `memory_total` per switch device.

### Collection Timing

Each collection cycle logs per-switch and total duration:

```
[streaming] Starting collection cycle...
  tor1: collection completed in 2.3s
  tor2: collection completed in 1.8s
[streaming] Collection cycle completed in 2.5s
```

## Benchmark Procedure

To run a reproducible benchmark:

1. **Start the server in live mode with profiling:**
   ```bash
   network-mapper serve --config config.yaml --profile --interval 30
   ```

2. **Collect baseline metrics:**
   ```bash
   curl -s http://localhost:8080/api/metrics | jq > baseline.json
   ```

3. **Wait for several collection cycles** (e.g., 10 minutes with 30s interval = ~20 cycles)

4. **Collect post-run metrics:**
   ```bash
   curl -s http://localhost:8080/api/metrics | jq > after.json
   ```

5. **Capture CPU profile:**
   ```bash
   go tool pprof -http=:9090 http://localhost:8080/debug/pprof/profile?seconds=60
   ```

6. **Check switch CPU/memory from topology:**
   ```bash
   curl -s http://localhost:8080/api/topology | jq '.devices[] | select(.type=="switch") | {id, cpu_utilization, memory_used, memory_total}'
   ```

## Results

> Run your benchmarks and record results here.

### Agent Resource Consumption

| Metric | Value | Notes |
|---|---|---|
| Binary size | TBD | `ls -la network-mapper` |
| Idle heap | TBD | No active collection |
| Active heap (N switches) | TBD | During collection cycle |
| Goroutines (idle) | TBD | Between cycles |
| Goroutines (collecting) | TBD | During parallel collection |
| GC pause per cycle | TBD | |
| Collection cycle time | TBD | Avg over 20 cycles |

### Switch Impact

| Metric | Baseline | During Collection | Delta |
|---|---|---|---|
| CPU utilization | TBD | TBD | TBD |
| Memory used | TBD | TBD | TBD |

### Docker Container Overhead

| Metric | Native | Docker | Difference |
|---|---|---|---|
| Collection time | TBD | TBD | TBD |
| Memory usage | TBD | TBD | TBD |
| Binary size | TBD | TBD | TBD |
