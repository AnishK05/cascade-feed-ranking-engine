# Documentation

**To run the stack:** start at [`running.md`](running.md) (all OSes). On Windows, also
read [`running-on-windows.md`](running-on-windows.md).

| Doc | What it is |
|-----|------------|
| [running.md](running.md) | Clone → Compose → smoke → UI → seed → kind. Windows and Linux/macOS commands. |
| [running-on-windows.md](running-on-windows.md) | PowerShell/`cascade.cmd` path, Windows port conflicts, ExecutionPolicy, Docker Desktop. |
| [demo.md](demo.md) | Five-minute click-through of `/feed`, `/graph`, `/admin`. |
| [ports-and-configuration.md](ports-and-configuration.md) | Host ports and environment variables. |
| [architecture.md](architecture.md) | Diagrams and data model (mirrors the plan). |
| [decisions/](decisions/README.md) | Architecture Decision Records. |
| [benchmarks/](benchmarks/2026-08-13-cache-comparison.md) | How to measure cache reduction; measured seed numbers. |
| [../IMPLEMENTATION_PLAN.md](../IMPLEMENTATION_PLAN.md) | Design and phase roadmap (source of truth for *what* to build). |
| [../deploy/k8s/README.md](../deploy/k8s/README.md) | Local kind manifests. |
| [../loadtest/README.md](../loadtest/README.md) | Seeder + Locust details. |
