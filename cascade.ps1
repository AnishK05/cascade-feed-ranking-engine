#Requires -Version 5.1
<#
.SYNOPSIS
  Windows entry point for Cascade (Docker Desktop + PowerShell / cmd).

.EXAMPLE
  .\cascade.cmd up
  .\cascade.cmd smoke
  .\cascade.cmd down
#>
[CmdletBinding()]
param(
  [Parameter(Position = 0)]
  [string]$Command = "help",

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$Rest
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
$ComposeFile = Join-Path $Root "deploy\docker-compose.yml"
$Cluster = if ($env:KIND_CLUSTER_NAME) { $env:KIND_CLUSTER_NAME } else { "cascade" }

function Write-Usage {
  @"
Cascade on Windows (Docker Desktop + PowerShell)

  cascade.cmd up              Build and start the full Compose stack
  cascade.cmd down            Stop Compose (keep volumes)
  cascade.cmd down-v          Stop Compose and delete Postgres/Redis/Kafka data
  cascade.cmd status          docker compose ps
  cascade.cmd logs [svc]      Tail logs (default: all app services)
  cascade.cmd smoke           Create users -> follow -> post -> follower feed
  cascade.cmd kafka-topics    Create/verify Kafka topics
  cascade.cmd seed [ci|full]  COPY a follow graph into Postgres (default ci)
  cascade.cmd warm-cache      Rebuild Redis timelines from Postgres
  cascade.cmd loadtest        Headless Locust (override with -Users / env)
  cascade.cmd kind-up         Local kind cluster (stop Compose first)
  cascade.cmd kind-down       Delete the kind cluster
  cascade.cmd kind-smoke      Same smoke test against kind's NodePort
  cascade.cmd kind-hpa        Show Feed Service HPA
  cascade.cmd kind-chaos      Delete a Feed pod; wait for /api/ping

Open http://localhost:3000 (UI) and http://localhost:8080/api/ping after 'up'.
Do not run Compose and kind at the same time (both bind 8080 and 3000).
"@ | Write-Host
}

function Require-Command([string]$Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Missing '$Name' on PATH. See docs/running-on-windows.md"
  }
}

function Require-Docker {
  Require-Command docker
  docker info 2>$null | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "Docker Desktop is not running. Start it from the Start menu and wait until the whale icon is idle."
  }
}

function Invoke-Compose {
  param([Parameter(ValueFromRemainingArguments = $true)][string[]]$ComposeArgs)
  Require-Docker
  & docker compose -f $ComposeFile @ComposeArgs
  if ($LASTEXITCODE -ne 0) {
    throw "docker compose failed (exit $LASTEXITCODE)"
  }
}

function Get-Python {
  $pyLauncher = Get-Command py -ErrorAction SilentlyContinue
  if ($pyLauncher) {
    return @{ File = $pyLauncher.Source; Prefix = @("-3") }
  }
  foreach ($name in @("python", "python3")) {
    $cmd = Get-Command $name -ErrorAction SilentlyContinue
    if ($cmd) {
      return @{ File = $cmd.Source; Prefix = @() }
    }
  }
  throw "Python 3.12+ is required for smoke/seed/loadtest. Install from https://www.python.org/downloads/ and tick 'Add python.exe to PATH'."
}

function Invoke-Python {
  param([string[]]$PythonArgs)
  $py = Get-Python
  $all = @()
  $all += $py.Prefix
  $all += $PythonArgs
  & $py.File @all
  if ($LASTEXITCODE -ne 0) {
    throw "python failed (exit $LASTEXITCODE): $($PythonArgs -join ' ')"
  }
}

function Ensure-UnixFile([string]$RelativePath) {
  $path = Join-Path $Root $RelativePath
  if (-not (Test-Path $path)) { return }
  $bytes = [System.IO.File]::ReadAllBytes($path)
  if ($bytes -contains 13) {
    $text = [System.IO.File]::ReadAllText($path)
    $unix = $text.Replace("`r`n", "`n").Replace("`r", "`n")
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($path, $unix, $utf8)
    Write-Host "Normalized CRLF -> LF: $RelativePath"
  }
}

function Test-GatewayPing {
  try {
    $resp = Invoke-WebRequest -Uri "http://localhost:8080/api/ping" -UseBasicParsing -TimeoutSec 5
    return $resp.StatusCode -eq 200
  } catch {
    return $false
  }
}

function Wait-GatewayPing([int]$Attempts = 60, [int]$DelaySeconds = 2) {
  for ($i = 0; $i -lt $Attempts; $i++) {
    if (Test-GatewayPing) { return }
    Start-Sleep -Seconds $DelaySeconds
  }
  throw "Gateway did not answer http://localhost:8080/api/ping. Run 'cascade.cmd status' and 'cascade.cmd logs gateway'."
}

function Get-LoadtestPython {
  $venvPy = Join-Path $Root "loadtest\.venv\Scripts\python.exe"
  if (-not (Test-Path $venvPy)) {
    Write-Host "Creating loadtest\\.venv"
    Invoke-Python @("-m", "venv", (Join-Path $Root "loadtest\.venv"))
  }
  return $venvPy
}

function Invoke-LoadtestPython {
  param([string[]]$PythonArgs)
  $venvPy = Get-LoadtestPython
  & $venvPy @PythonArgs
  if ($LASTEXITCODE -ne 0) {
    throw "loadtest python failed (exit $LASTEXITCODE)"
  }
}

function Invoke-Up {
  Ensure-UnixFile "scripts\kafka-init.sh"
  Write-Host "Building and starting Compose (first run can take 10-20 minutes)..."
  Invoke-Compose @("up", "-d", "--build")
  Write-Host "Waiting for Gateway /api/ping ..."
  Wait-GatewayPing
  Write-Host @"
Stack is up.
  UI:      http://localhost:3000
  Gateway: http://localhost:8080
  Grafana: http://localhost:3001  (admin/admin)
  Smoke:   cascade.cmd smoke
"@
}

function Invoke-Smoke {
  $script = Join-Path $Root "scripts\smoke_test.py"
  if (-not $env:GATEWAY_URL) { $env:GATEWAY_URL = "http://localhost:8080" }
  Invoke-Python @($script)
}

function Invoke-Seed {
  $preset = "ci"
  if ($Rest -and $Rest.Count -ge 1 -and $Rest[0]) { $preset = $Rest[0] }
  $req = Join-Path $Root "loadtest\requirements.txt"
  $venvPy = Get-LoadtestPython
  & $venvPy -m pip install -q -r $req
  if ($LASTEXITCODE -ne 0) { throw "pip install failed" }
  Push-Location (Join-Path $Root "loadtest")
  try {
    & $venvPy "seed.py" "--preset" $preset
    if ($LASTEXITCODE -ne 0) { throw "seed.py failed" }
  } finally {
    Pop-Location
  }
  if ($preset -eq "ci") {
    Write-Host "ci seed uses celebrity threshold 80. Recreate fanout + social-graph so live fanout matches is_celebrity:"
    Write-Host '  $env:CELEBRITY_FOLLOWER_THRESHOLD="80"'
    Write-Host '  $env:CELEBRITY_THRESHOLD="80"'
    Write-Host '  docker compose -f deploy\docker-compose.yml up -d --force-recreate fanout-worker social-graph-service'
  }
}

function Invoke-WarmCache {
  Invoke-Compose @("--profile", "tools", "run", "--rm", "warm-cache")
}

function Invoke-Loadtest {
  $users = "50"
  $duration = "30s"
  $hostUrl = "http://localhost:8080"
  if ($env:USERS) { $users = $env:USERS }
  if ($env:DURATION) { $duration = $env:DURATION }
  if ($env:HOST) { $hostUrl = $env:HOST }
  $req = Join-Path $Root "loadtest\requirements.txt"
  $reports = Join-Path $Root "loadtest\reports"
  if (-not (Test-Path $reports)) { New-Item -ItemType Directory -Path $reports | Out-Null }
  $venvPy = Get-LoadtestPython
  & $venvPy -m pip install -q -r $req
  if ($LASTEXITCODE -ne 0) { throw "pip install failed" }
  Push-Location (Join-Path $Root "loadtest")
  try {
    & $venvPy -m locust -f locustfile.py --headless --host $hostUrl -u $users -r 10 -t $duration --csv reports/manual --html reports/manual.html --only-summary
    if ($LASTEXITCODE -ne 0) { throw "locust failed" }
  } finally {
    Pop-Location
  }
}

function Invoke-KindUp {
  Require-Docker
  Require-Command kind
  Require-Command kubectl
  Ensure-UnixFile "scripts\kafka-init.sh"
  Ensure-UnixFile "deploy\k8s\init\kafka-init.sh"

  $existing = @(kind get clusters 2>$null)
  if ($existing -notcontains $Cluster) {
    kind create cluster --config (Join-Path $Root "deploy\k8s\kind-config.yaml") --name $Cluster
    if ($LASTEXITCODE -ne 0) { throw "kind create cluster failed" }
  } else {
    Write-Host "kind-up: cluster $Cluster already exists"
  }

  Write-Host "kind-up: building application images"
  Invoke-Compose @("build", "post-service", "feed-service", "fanout-worker", "social-graph-service", "gateway", "frontend", "warm-cache")
  $images = @(
    "cascade/post-service:local",
    "cascade/feed-service:local",
    "cascade/fanout-worker:local",
    "cascade/social-graph-service:local",
    "cascade/gateway:local",
    "cascade/frontend:local",
    "cascade/warm-cache:local"
  )
  foreach ($image in $images) {
    kind load docker-image $image --name $Cluster
    if ($LASTEXITCODE -ne 0) { throw "kind load failed for $image" }
  }

  kubectl create namespace cascade --dry-run=client -o yaml | kubectl apply -f -
  kubectl -n cascade delete job kafka-init --ignore-not-found
  kubectl apply -k (Join-Path $Root "deploy\k8s")
  if ($LASTEXITCODE -ne 0) { throw "kubectl apply -k failed" }
  kubectl apply -f (Join-Path $Root "deploy\k8s\metrics-server.yaml")
  if ($LASTEXITCODE -ne 0) { throw "kubectl apply metrics-server failed" }

  Write-Host "kind-up: waiting for data plane"
  kubectl -n cascade wait --for=condition=available --timeout=180s deploy/postgres deploy/redis deploy/kafka
  kubectl -n cascade wait --for=condition=complete --timeout=180s job/kafka-init
  if ($LASTEXITCODE -ne 0) {
    kubectl -n cascade logs job/kafka-init
    throw "kafka-init did not complete"
  }

  Write-Host "kind-up: waiting for application deployments"
  kubectl -n cascade wait --for=condition=available --timeout=300s `
    deploy/post-service deploy/feed-service deploy/fanout-worker `
    deploy/social-graph-service deploy/gateway deploy/frontend
  if ($LASTEXITCODE -ne 0) { throw "application deployments were not ready" }

  Write-Host "kind-up: waiting for Gateway /api/ping"
  Wait-GatewayPing
  Write-Host @"
kind cluster is ready
  UI:      http://localhost:3000
  Gateway: http://localhost:8080
  Smoke:   cascade.cmd kind-smoke
"@
}

function Invoke-KindChaos {
  Require-Command kubectl
  kubectl -n cascade delete pod -l app=feed-service --wait=false
  Wait-GatewayPing -Attempts 60 -DelaySeconds 1
  Write-Host "kind-chaos: Gateway still serving /api/ping after feed-service pod delete"
  kubectl -n cascade get pods -l app=feed-service
}

switch ($Command.ToLowerInvariant()) {
  "help" { Write-Usage }
  "up" { Invoke-Up }
  "down" { Invoke-Compose @("down") }
  "down-v" { Invoke-Compose @("down", "-v") }
  "status" { Invoke-Compose @("ps") }
  "ps" { Invoke-Compose @("ps") }
  "logs" {
    $logArgs = @("logs", "--tail=80")
    if ($Rest -and $Rest.Count -gt 0) {
      $logArgs += $Rest
    } else {
      $logArgs += @("gateway", "feed-service", "post-service", "fanout-worker", "kafka")
    }
    Invoke-Compose @logArgs
  }
  "smoke" { Invoke-Smoke }
  "kafka-topics" { Invoke-Compose @("run", "--rm", "kafka-init") }
  "seed" { Invoke-Seed }
  "warm-cache" { Invoke-WarmCache }
  "loadtest" { Invoke-Loadtest }
  "kind-up" { Invoke-KindUp }
  "kind-down" {
    Require-Command kind
    kind delete cluster --name $Cluster
  }
  "kind-smoke" {
    $env:GATEWAY_URL = "http://localhost:8080"
    Invoke-Smoke
  }
  "k8s-smoke" {
    $env:GATEWAY_URL = "http://localhost:8080"
    Invoke-Smoke
  }
  "kind-hpa" {
    Require-Command kubectl
    kubectl -n cascade get hpa feed-service
    kubectl -n cascade get deploy feed-service
  }
  "kind-chaos" { Invoke-KindChaos }
  default {
    Write-Host "Unknown command: $Command"
    Write-Usage
    exit 1
  }
}
