# Running Cascade on Windows

OS-agnostic runbook (Linux/macOS included): [`running.md`](running.md).
Five-minute UI walkthrough: [`demo.md`](demo.md). Ports and env:
[`ports-and-configuration.md`](ports-and-configuration.md).

This page is the **Windows** path: Docker Desktop + PowerShell or cmd. You do not need
Ubuntu, Git Bash, or `make`.

The stack itself is Linux containers. On Windows those containers run inside
**Docker Desktop**. You drive everything from **PowerShell or cmd** with:

```bat
cascade.cmd up
cascade.cmd smoke
```

`cascade.cmd` calls `cascade.ps1` with `-ExecutionPolicy Bypass`, so you do not
have to change your machine-wide PowerShell policy.

A laptop with **16 GB RAM** is comfortable. Give Docker Desktop **at least 6 GB**.
Budget **~20 GB disk** for images. **8 GB** machines can boot the stack but will swap.

---

## 1. Install once

1. [Docker Desktop for Windows](https://docs.docker.com/desktop/setup/install/windows-install/)
   - Settings → General → **Use the WSL 2 based engine** (Docker’s default on Win 11;
     you still work in PowerShell — you never have to open a Linux terminal).
   - Settings → Resources → Memory **≥ 6 GB**.
   - Start Docker Desktop and wait until the whale icon is idle.
2. [Python 3.12+](https://www.python.org/downloads/windows/) — tick **Add python.exe to PATH**.
3. [Git for Windows](https://git-scm.com/download/win).
4. Clone (PowerShell):

```powershell
cd $HOME\src
git clone https://github.com/AnishK05/cascade-feed-ranking-engine.git
cd cascade-feed-ranking-engine
```

Check:

```powershell
docker version
docker compose version
python --version
.\cascade.cmd help
```

Optional (only for `kind-up`): [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installing-from-release-binaries)
and [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/) on your PATH.

You do **not** need Go, Java, Node, or Make to demo the product. Compose builds those
inside images.

---

## 2. End-to-end demo

All commands below are from the **repo root** in PowerShell or cmd.

### 2.1 Free the ports

| Port | Used by |
|------|---------|
| 3000 | Next.js UI |
| 3001 | Grafana |
| 5432 | Postgres |
| 6379 | Redis |
| 8080 | Gateway |
| 8081 | Social Graph |
| 9090 / 9091 | Post / Feed gRPC |
| 9092 | Kafka |
| 9095 | Prometheus |
| 9100–9102 | Go metrics |

Do **not** run Compose and kind at the same time — both bind **8080** and **3000**.

If Windows already has Postgres or IIS:

```powershell
Get-NetTCPConnection -LocalPort 8080,3000,5432 -ErrorAction SilentlyContinue |
  Select-Object LocalPort,OwningProcess
```

Stop the leftover service, or `.\cascade.cmd down` / `.\cascade.cmd kind-down`.

### 2.2 Start

```powershell
.\cascade.cmd up
```

First build is 10–20 minutes (Go + Maven + Next.js images). Success prints the UI
and Gateway URLs and waits until `http://localhost:8080/api/ping` answers.

```powershell
.\cascade.cmd status
```

App containers should be `healthy`. `kafka-init` should have exited `0`.

### 2.3 Smoke test

```powershell
.\cascade.cmd smoke
```

Expected:

```text
ok: follower 2 saw post 1 from author 1
```

That is Gateway → Post Service → Kafka → Fanout → Redis → Feed → follower GetFeed.

### 2.4 Use the UI (normal Windows browser)

| URL | What |
|-----|------|
| http://localhost:3000/feed | Home feed. Pick a user in the top bar, compose a post. |
| http://localhost:3000/graph | Follow / unfollow. |
| http://localhost:3000/admin | Cache hit ratio, latency, Kafka lag. |
| http://localhost:8080/api/ping | Gateway health. |
| http://localhost:3001 | Grafana (`admin` / `admin`, or anonymous viewer). |

There is no login. The switcher sends `X-User-Id` (spoofable on purpose).

1. After smoke (or seed), open `/feed` and pick a user.
2. Post as user A. Switch to a follower B and refresh — the post appears after fanout
   (usually well under a second).
3. `/admin` moves after a few requests.

### 2.5 Optional: seed users

```powershell
.\cascade.cmd seed
# .\cascade.cmd seed full    # 50k users — heavy
.\cascade.cmd warm-cache
```

Default `ci` is 500 users with celebrity threshold **80**. Recreate fanout + social-graph
so live fanout matches the seeded `is_celebrity` flags:

```powershell
$env:CELEBRITY_FOLLOWER_THRESHOLD = "80"
$env:CELEBRITY_THRESHOLD = "80"
docker compose -f deploy\docker-compose.yml up -d --force-recreate fanout-worker social-graph-service
```

### 2.6 Optional: Locust

```powershell
$env:USERS = "20"
$env:DURATION = "30s"
.\cascade.cmd loadtest
```

Report: `loadtest\reports\manual.html`.

### 2.7 Stop

```powershell
.\cascade.cmd down
.\cascade.cmd down-v    # also delete Postgres/Redis/Kafka volumes
```

---

## 3. Optional: kind (local Kubernetes)

Stop Compose first (`.\cascade.cmd down`).

```powershell
.\cascade.cmd kind-up
.\cascade.cmd kind-smoke
.\cascade.cmd kind-hpa
.\cascade.cmd kind-down
```

Same URLs: UI `:3000`, Gateway `:8080`.

---

## 4. What “it works” looks like

1. `.\cascade.cmd up` waits until ping succeeds.
2. `.\cascade.cmd smoke` prints `ok: follower … saw post …`.
3. http://localhost:3000/feed opens in Edge/Chrome.
4. (Optional) `.\cascade.cmd seed` then `/graph` lists hundreds of users.

---

## 5. Troubleshooting

```powershell
docker info
.\cascade.cmd status
.\cascade.cmd logs gateway
.\cascade.cmd logs fanout-worker
```

| Symptom | Cause | Fix |
|---------|--------|-----|
| `Missing 'docker' on PATH` | Docker Desktop not installed, or terminal opened before install | Install Desktop; **open a new** PowerShell |
| `Docker Desktop is not running` | App still starting | Wait for the whale; `wsl --shutdown` then restart Desktop if it hangs |
| `Bind for 0.0.0.0:5432 failed` | Windows PostgreSQL service | Stop it in `services.msc` |
| `Bind for 0.0.0.0:8080` / `:3000` | IIS, leftover Compose, or kind | `.\cascade.cmd down`; `.\cascade.cmd kind-down`; `Get-NetTCPConnection -LocalPort 8080` |
| `python failed` / `py` not found | Python not on PATH | Re-run the Python installer with “Add to PATH”; new terminal |
| Smoke: connection refused | Gateway not up yet | `.\cascade.cmd status`; wait; rerun smoke |
| Smoke: `timeout after 45s waiting for post` | Fanout/Kafka | `.\cascade.cmd logs fanout-worker`; `.\cascade.cmd kafka-topics` |
| Java containers `exit 137` | Docker RAM too low | Desktop → Resources → Memory ≥ 6 GB |
| `kafka-init` never exits 0 | Kafka still electing, or `kafka-init.sh` has CRLF | `.\cascade.cmd logs kafka`; `up` already converts that script to LF |
| Frontend loads, API fails | Gateway down, or not using localhost:3000 | CORS allows `http://localhost:3000` only |
| Empty user switcher | No rows in Postgres | `.\cascade.cmd smoke` or `.\cascade.cmd seed` |
| kind `ImagePullBackOff` | Images not loaded | `.\cascade.cmd kind-up` again (`imagePullPolicy: Never`) |
| kind Gateway never pings | Compose still owns 8080 | `.\cascade.cmd down` first |
| `.\cascade.ps1` blocked by ExecutionPolicy | Running the `.ps1` directly | Use `.\cascade.cmd …` instead |
| Desktop stuck “Starting the engine” | WSL VM wedged after sleep | PowerShell: `wsl --shutdown`, then start Docker Desktop |

Compose **or** kind, never both.

---

## 6. Linux/macOS

Those machines still use `make up` / `make smoke`. Windows uses `cascade.cmd`. Same
containers, same ports, same UI.
