# profile-resource-usage.ps1
# Measures memory and CPU consumption of network-mapper under various conditions.
#
# Usage: .\scripts\profile-resource-usage.ps1
#
# Prerequisites: network-mapper.exe built at repo root
#   go build -o network-mapper.exe ./cmd/network-mapper

param(
    [int]$Port = 9999,
    [int]$DurationSec = 30,
    [int]$ConcurrentClients = 10,
    [string]$TopologyFile = "examples\demo-topology-v3.json"
)

$ErrorActionPreference = "Stop"
$binary = ".\network-mapper.exe"

if (-not (Test-Path $binary)) {
    Write-Host "ERROR: $binary not found. Run 'go build -o network-mapper.exe ./cmd/network-mapper' first." -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $TopologyFile)) {
    Write-Host "ERROR: Topology file '$TopologyFile' not found." -ForegroundColor Red
    exit 1
}

$baseUrl = "http://localhost:$Port"

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Network Mapper Resource Profiling" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Config:" -ForegroundColor Yellow
Write-Host "  Binary:      $binary"
Write-Host "  Topology:    $TopologyFile"
Write-Host "  Port:        $Port"
Write-Host "  Duration:    ${DurationSec}s per phase"
Write-Host "  Clients:     $ConcurrentClients concurrent"
Write-Host ""

# --- Phase 0: Start the server ---
Write-Host "[Phase 0] Starting server with --profile flag..." -ForegroundColor Green
$proc = Start-Process -FilePath $binary -ArgumentList "serve","--topology",$TopologyFile,"--port",$Port,"--profile","--no-open" -PassThru -NoNewWindow
Start-Sleep -Seconds 3

# Verify server is up
try {
    $health = Invoke-RestMethod -Uri "$baseUrl/api/health" -TimeoutSec 5
    Write-Host "  Server is UP (health: $($health.status))" -ForegroundColor Green
} catch {
    Write-Host "  ERROR: Server failed to start" -ForegroundColor Red
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    exit 1
}

function Get-ProcessMetrics($process) {
    $p = Get-Process -Id $process.Id -ErrorAction SilentlyContinue
    if (-not $p) { return $null }
    return @{
        WorkingSetMB = [math]::Round($p.WorkingSet64 / 1MB, 2)
        PrivateMemMB = [math]::Round($p.PrivateMemorySize64 / 1MB, 2)
        CPU_Seconds  = [math]::Round($p.TotalProcessorTime.TotalSeconds, 2)
        Threads      = $p.Threads.Count
        Handles      = $p.HandleCount
    }
}

function Show-Metrics($label, $metrics) {
    Write-Host "  [$label]" -ForegroundColor Yellow
    Write-Host "    Working Set:   $($metrics.WorkingSetMB) MB"
    Write-Host "    Private Mem:   $($metrics.PrivateMemMB) MB"
    Write-Host "    CPU Time:      $($metrics.CPU_Seconds) s"
    Write-Host "    Threads:       $($metrics.Threads)"
    Write-Host "    Handles:       $($metrics.Handles)"
}

# --- Phase 1: Idle baseline ---
Write-Host ""
Write-Host "[Phase 1] Measuring IDLE baseline (5s settle)..." -ForegroundColor Green
Start-Sleep -Seconds 5
$idleMetrics = Get-ProcessMetrics $proc
Show-Metrics "Idle" $idleMetrics

# --- Phase 2: Single request latency ---
Write-Host ""
Write-Host "[Phase 2] Measuring single /api/topology request..." -ForegroundColor Green
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$resp = Invoke-WebRequest -Uri "$baseUrl/api/topology" -UseBasicParsing
$sw.Stop()
Write-Host "  Response size: $([math]::Round($resp.Content.Length / 1KB, 1)) KB"
Write-Host "  Latency:       $($sw.ElapsedMilliseconds) ms"
Write-Host "  Status:        $($resp.StatusCode)"

# --- Phase 3: Sustained load ---
Write-Host ""
Write-Host "[Phase 3] Sustained load: $ConcurrentClients clients, ${DurationSec}s..." -ForegroundColor Green
$cpuBefore = (Get-ProcessMetrics $proc).CPU_Seconds

$runspacePool = [runspacefactory]::CreateRunspacePool(1, $ConcurrentClients)
$runspacePool.Open()

$scriptBlock = {
    param($url, $duration)
    $end = (Get-Date).AddSeconds($duration)
    $count = 0
    while ((Get-Date) -lt $end) {
        try {
            $null = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 10
            $count++
        } catch { }
    }
    return $count
}

$handles = @()
for ($i = 0; $i -lt $ConcurrentClients; $i++) {
    $ps = [powershell]::Create().AddScript($scriptBlock).AddArgument("$baseUrl/api/topology").AddArgument($DurationSec)
    $ps.RunspacePool = $runspacePool
    $handles += @{ PowerShell = $ps; Handle = $ps.BeginInvoke() }
}

# Wait for all to complete
$totalRequests = 0
foreach ($h in $handles) {
    $count = $h.PowerShell.EndInvoke($h.Handle)
    $totalRequests += $count[0]
    $h.PowerShell.Dispose()
}
$runspacePool.Close()

$loadMetrics = Get-ProcessMetrics $proc
$cpuAfter = $loadMetrics.CPU_Seconds
$cpuUsed = [math]::Round($cpuAfter - $cpuBefore, 2)

Show-Metrics "Under Load" $loadMetrics
Write-Host "  Total Requests:  $totalRequests"
Write-Host "  Throughput:      $([math]::Round($totalRequests / $DurationSec, 1)) req/s"
Write-Host "  CPU consumed:    $cpuUsed s (during ${DurationSec}s wall time)"
Write-Host "  CPU utilization: $([math]::Round(($cpuUsed / $DurationSec) * 100, 1))%"

# --- Phase 4: Memory after GC (trigger via pprof) ---
Write-Host ""
Write-Host "[Phase 4] Triggering GC and checking post-load memory..." -ForegroundColor Green
try {
    $null = Invoke-WebRequest -Uri "$baseUrl/debug/pprof/heap?gc=1" -UseBasicParsing -TimeoutSec 10
} catch { }
Start-Sleep -Seconds 2
$postGCMetrics = Get-ProcessMetrics $proc
Show-Metrics "Post-GC" $postGCMetrics

# --- Phase 5: pprof heap summary ---
Write-Host ""
Write-Host "[Phase 5] Go runtime memory stats (from pprof)..." -ForegroundColor Green
try {
    $heapResp = Invoke-WebRequest -Uri "$baseUrl/debug/pprof/heap?debug=1" -UseBasicParsing -TimeoutSec 10
    $heapText = $heapResp.Content
    $memLines = $heapText -split "`n" | Where-Object { $_ -match "(HeapAlloc|HeapSys|HeapInuse|HeapIdle|StackInuse|Sys|NumGC)" } | Select-Object -First 10
    foreach ($line in $memLines) {
        Write-Host "    $($line.Trim())"
    }
    if ($memLines.Count -eq 0) {
        Write-Host "    (pprof debug output available at $baseUrl/debug/pprof/heap?debug=1)" -ForegroundColor Gray
    }
} catch {
    Write-Host "  Could not fetch pprof heap data: $_" -ForegroundColor Yellow
}

# --- Summary ---
Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host " SUMMARY" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Idle Memory:         $($idleMetrics.WorkingSetMB) MB (working set)"
Write-Host "  Load Memory:         $($loadMetrics.WorkingSetMB) MB (working set)"
Write-Host "  Post-GC Memory:      $($postGCMetrics.WorkingSetMB) MB (working set)"
Write-Host "  Memory Delta (load): $([math]::Round($loadMetrics.WorkingSetMB - $idleMetrics.WorkingSetMB, 2)) MB"
Write-Host ""
Write-Host "  Throughput:          $([math]::Round($totalRequests / $DurationSec, 1)) req/s ($ConcurrentClients clients)"
Write-Host "  Avg Latency:        ~$([math]::Round(($DurationSec * 1000 * $ConcurrentClients) / [math]::Max($totalRequests,1), 1)) ms"
Write-Host "  CPU Utilization:     $([math]::Round(($cpuUsed / $DurationSec) * 100, 1))% (single core)"
Write-Host ""
Write-Host "  pprof endpoints:     $baseUrl/debug/pprof/" -ForegroundColor Gray
Write-Host "    - Heap profile:    go tool pprof $baseUrl/debug/pprof/heap"
Write-Host "    - CPU profile:     go tool pprof '$baseUrl/debug/pprof/profile?seconds=30'"
Write-Host "    - Goroutines:      $baseUrl/debug/pprof/goroutine?debug=1"
Write-Host ""

# --- Cleanup ---
Write-Host "Stopping server (PID $($proc.Id))..." -ForegroundColor Gray
Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
Write-Host "Done." -ForegroundColor Green
