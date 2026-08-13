# Running Cascade

This is the runbook. Design lives in [`IMPLEMENTATION_PLAN.md`](../IMPLEMENTATION_PLAN.md).

You need **Docker** (Compose v2) and **Python 3.12+**. You do **not** need Go, Java, Node, or
Make to demo: images build those toolchains. First `up` is 10–20 minutes.

| | Windows (PowerShell / cmd) | Linux / macOS |
|--|----------------------------|---------------|
| Start | `cascade.cmd up` | `make up` |
| Prove it | `cascade.cmd smoke` | `make smoke` |
| UI | http://localhost:3000 | same |
| Stop | `cascade.cmd down` | `make down` |

Windows extras (ports, Docker Desktop, ExecutionPolicy):
[`running-on-windows.md`](running-on-windows.md).

Do **not** run Compose and kind at the same time. Both bind **8080** and **3000**.

---

## 1. Install

**Windows:** Docker Desktop + Python (Add to PATH) + Git. See
[`running-on-windows.md`](running-on-windows.md) §1.

**macOS:** [Docker Desktop](https://docs.docker.com/desktop/setup/install/mac-install/) or
Colima (`brew install colima docker docker-compose && colima start`). Python 3 via
`brew install python` or python.org. `make` is in Xcode CLT: `xcode-select --install`.

**Linux:** Docker Engine + Compose plugin
([install](https://docs.docker.com/engine/install/)), your user in the `docker` group
(`sudo usermod -aG docker $USER` then log out). `sudo apt install make python3 python3-venv`
(or the Fedora equivalents).

Check:

```bash
docker version
docker compose version
python3 --version    # Windows: python --version
```

Give Docker **≥ 6 GB RAM**. Kafka plus two JVMs will OOM (exit 137) below that.

---

## 2. Clone and start

```bash
git clone https://github.com/AnishK05/cascade-feed-ranking-engine.git
cd cascade-feed-ranking-engine
```

```bat
REM Windows
cascade.cmd up
cascade.cmd status
```

```bash
# Linux / macOS
make up
docker compose -f deploy/docker-compose.yml ps
```

Wait until `postgres`, `redis`, `kafka`, `post-service`, `feed-service`, `fanout-worker`,
`social-graph-service`, `gateway`, and `frontend` are **healthy**. `kafka-init` should be
**exited (0)**. Then:

```bat
cascade.cmd smoke
```

```bash
make smoke
```

Success:

```text
ok: follower 2 saw post 1 from author 1
```

That is the whole write path (Gateway → Post → Kafka → Fanout → Redis → Feed).

If ping fails or smoke times out, see [Troubleshooting](#6-troubleshooting) or the
[Windows table](running-on-windows.md#5-troubleshooting).

---

## 3. Use the product

Walkthrough with what you should see: [`demo.md`](demo.md).

| URL | What |
|-----|------|
| http://localhost:3000/feed | Ranked feed. User switcher = `X-User-Id` (no login). |
| http://localhost:3000/graph | Follow / unfollow. |
| http://localhost:3000/admin | Cache hits, latency, Kafka lag. |
| http://localhost:8080/api/ping | Gateway health. |
| http://localhost:3001 | Grafana (`admin`/`admin` or anonymous viewer). |
| http://localhost:9095 | Prometheus. |

Host ports: [`ports-and-configuration.md`](ports-and-configuration.md).

---

## 4. Optional next steps

**Seed a graph** (default `ci` = 500 users, celebrity threshold **80**):

```bat
cascade.cmd seed
cascade.cmd warm-cache
```

```bash
make seed
make warm-cache-compose
```

After `ci`, recreate fanout + social-graph so live fanout matches `is_celebrity`:

```bash
# Linux / macOS
CELEBRITY_FOLLOWER_THRESHOLD=80 CELEBRITY_THRESHOLD=80 \
  docker compose -f deploy/docker-compose.yml up -d --force-recreate \
  fanout-worker social-graph-service
```

```powershell
$env:CELEBRITY_FOLLOWER_THRESHOLD = "80"
$env:CELEBRITY_THRESHOLD = "80"
docker compose -f deploy\docker-compose.yml up -d --force-recreate fanout-worker social-graph-service
```

`make seed PRESET=full` / `cascade.cmd seed full` is 50k users / ~7.8M edges — heavy.

**Short Locust run:** `make loadtest USERS=20 DURATION=30s` or
`$env:USERS="20"; $env:DURATION="30s"; cascade.cmd loadtest`.
Full cache protocol: [`../loadtest/README.md`](../loadtest/README.md) and
[`benchmarks/2026-08-13-cache-comparison.md`](benchmarks/2026-08-13-cache-comparison.md).

**Local Kubernetes:** stop Compose first, then `make kind-up` / `cascade.cmd kind-up`.
Details: [`../deploy/k8s/README.md`](../deploy/k8s/README.md).

---

## 5. Stop

```bat
cascade.cmd down
cascade.cmd down-v
```

```bash
make down
docker compose -f deploy/docker-compose.yml down -v
```

`down-v` / `-v` wipes Postgres, Redis, and Kafka volumes.

---

## 6. Troubleshooting

| Symptom | Fix |
|---------|-----|
| `permission denied` talking to Docker (Linux) | Add your user to `docker`; log out; `docker info` without sudo |
| `Cannot connect to the Docker daemon` | Start Docker Desktop / `sudo service docker start` / `colima start` |
| Port already allocated (`5432`, `8080`, `3000`) | Stop local Postgres/Redis, leftover Compose, or kind. Linux: `ss -ltnp \| grep 8080`. macOS: `lsof -i :8080` |
| Compose and kind both up | `make down` or `make kind-down` — pick one |
| Smoke: connection refused | Gateway not healthy yet; wait; `docker compose -f deploy/docker-compose.yml logs gateway` |
| Smoke: timeout waiting for the post | `logs fanout-worker kafka kafka-init`; `make kafka-topics` |
| Container `exit 137` | Raise Docker memory to ≥ 6 GB |
| Empty user switcher | Run smoke or seed |
| `python3: command not found` (Unix) | Install Python 3; on Windows use `cascade.cmd` (`py`/`python`) |
| `make: command not found` | Windows: use `cascade.cmd`. Linux: install `make`. macOS: `xcode-select --install` |
| `go: cannot find main module` | Host-native Go: `go test ./services/...` or `make go-test`, not `go test ./...` |
| UI calls fail / CORS | Browse `http://localhost:3000`, not another origin |
| Stale frontend after changing API URL | `NEXT_PUBLIC_API_BASE` is baked at **image build**; rebuild `frontend` |

Windows-only rows (IIS, ExecutionPolicy, whale icon):
[`running-on-windows.md`](running-on-windows.md) §5.

---

## 7. Command map

| Action | Linux / macOS | Windows |
|--------|---------------|---------|
| Start stack | `make up` | `cascade.cmd up` |
| Status | `docker compose -f deploy/docker-compose.yml ps` | `cascade.cmd status` |
| Logs | `docker compose -f deploy/docker-compose.yml logs -f gateway` | `cascade.cmd logs gateway` |
| Smoke | `make smoke` | `cascade.cmd smoke` |
| Kafka topics | `make kafka-topics` | `cascade.cmd kafka-topics` |
| Seed | `make seed` / `PRESET=full` | `cascade.cmd seed` / `seed full` |
| Warm Redis | `make warm-cache-compose` | `cascade.cmd warm-cache` |
| Locust | `make loadtest` | `cascade.cmd loadtest` |
| kind | `make kind-up` / `k8s-smoke` / `kind-down` | `cascade.cmd kind-up` / `kind-smoke` / `kind-down` |
| Tests (host) | `make test` | install Go/Java/Make, or use WSL; Compose demo does not need this |
| Stop | `make down` | `cascade.cmd down` |

---

## 8. Host-native develop (optional)

Only if you are changing Go/Java/Python, not for the demo:

```bash
make proto     # gitignored *.pb.go
make test      # mirrors CI
make go-cover
```

Compose already compiles protos **inside the image**. Skip this unless you run services on
the host against `localhost` Postgres/Redis/Kafka.
