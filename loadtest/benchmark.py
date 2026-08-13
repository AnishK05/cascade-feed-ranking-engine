#!/usr/bin/env python3
"""Run the Phase 12 before/after cache protocol and emit a Markdown write-up.

The comparison is:

    cache_reduction = 1 - (postgres_qps_with_cache / postgres_qps_baseline)

Postgres QPS is taken from the Prometheus counters:

    feed_postgres_queries_total{op="candidates|hydrate"}
    post_postgres_queries_total{op="get_posts"}

Signal queries (`op="signals"`) run in both modes and are reported separately.

This script does not start services. Point it at a running stack:

    # Baseline (Redis bypassed)
    FEED_BYPASS_CACHE=true POST_BYPASS_CACHE=true  # then restart feed/post
    python benchmark.py --label baseline --users 50 --spawn-rate 10 --duration 30s

    # Cached (default)
    FEED_BYPASS_CACHE=false POST_BYPASS_CACHE=false
    python benchmark.py --label cached --users 50 --spawn-rate 10 --duration 30s

    python benchmark.py --compare reports/baseline_stats.csv reports/cached_stats.csv \\
        --baseline-metrics reports/baseline_metrics.json \\
        --cached-metrics reports/cached_metrics.json \\
        --write-docs ../docs/benchmarks/2026-08-13-cache-comparison.md
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from pathlib import Path

from seeding.reports import (
    LocustStats,
    cache_reduction,
    parse_locust_stats_csv,
    parse_prometheus_counters,
    sum_ops,
)

ROOT = Path(__file__).resolve().parent
CACHEABLE_FEED_OPS = ("candidates", "hydrate")
POST_OPS = ("get_posts",)
SIGNAL_OPS = ("signals",)


@dataclass
class MetricSnapshot:
    captured_at: str
    feed: dict[str, float]
    post: dict[str, float]
    duration_s: float = 0.0

    @property
    def cacheable(self) -> float:
        return sum_ops(self.feed, CACHEABLE_FEED_OPS) + sum_ops(self.post, POST_OPS)

    @property
    def signals(self) -> float:
        return sum_ops(self.feed, SIGNAL_OPS)


def fetch_text(url: str, timeout: float = 5.0) -> str:
    with urllib.request.urlopen(url, timeout=timeout) as response:
        return response.read().decode("utf-8")


def scrape_metrics(feed_url: str, post_url: str) -> MetricSnapshot:
    feed_text = fetch_text(feed_url)
    post_text = fetch_text(post_url)
    return MetricSnapshot(
        captured_at=datetime.now(UTC).isoformat(),
        feed=parse_prometheus_counters(feed_text, "feed_postgres_queries_total"),
        post=parse_prometheus_counters(post_text, "post_postgres_queries_total"),
    )


def delta(before: MetricSnapshot, after: MetricSnapshot) -> MetricSnapshot:
    duration = max(
        (
            datetime.fromisoformat(after.captured_at) - datetime.fromisoformat(before.captured_at)
        ).total_seconds(),
        1e-6,
    )
    feed = {op: after.feed.get(op, 0.0) - before.feed.get(op, 0.0) for op in set(before.feed) | set(after.feed)}
    post = {op: after.post.get(op, 0.0) - before.post.get(op, 0.0) for op in set(before.post) | set(after.post)}
    return MetricSnapshot(
        captured_at=after.captured_at, feed=feed, post=post, duration_s=duration
    )


def run_locust(
    host: str,
    users: int,
    spawn_rate: int,
    duration: str,
    csv_prefix: Path,
    locustfile: Path,
    html: Path | None,
) -> int:
    csv_prefix.parent.mkdir(parents=True, exist_ok=True)
    cmd = [
        sys.executable,
        "-m",
        "locust",
        "-f",
        str(locustfile),
        "--headless",
        "--host",
        host,
        "-u",
        str(users),
        "-r",
        str(spawn_rate),
        "-t",
        duration,
        "--csv",
        str(csv_prefix),
        "--only-summary",
    ]
    if html is not None:
        cmd.extend(["--html", str(html)])
    print(" ".join(cmd), file=sys.stderr)
    return subprocess.call(cmd, cwd=str(ROOT))


def qps(snapshot: MetricSnapshot) -> float:
    return snapshot.cacheable / snapshot.duration_s if snapshot.duration_s else 0.0


def format_writeup(
    *,
    machine: str,
    instances: str,
    latency_threshold_ms: float,
    read_write_ratio: str,
    baseline_locust: dict[str, LocustStats] | None,
    cached_locust: dict[str, LocustStats] | None,
    baseline_metrics: MetricSnapshot | None,
    cached_metrics: MetricSnapshot | None,
    notes: list[str],
) -> str:
    today = datetime.now(UTC).date().isoformat()
    reduction = None
    if baseline_metrics and cached_metrics:
        reduction = cache_reduction(qps(baseline_metrics), qps(cached_metrics))
    reduction_pct = f"{reduction * 100:.1f}%" if reduction is not None else "not measured"

    def locust_block(title: str, stats: dict[str, LocustStats] | None) -> str:
        if not stats:
            return f"### {title}\n\nNot measured in this environment.\n"
        agg = stats.get("Aggregated") or stats.get("Total")
        feed = stats.get("/api/feed")
        lines = [f"### {title}", ""]
        if agg:
            lines.append(
                f"- Aggregate: **{agg.requests_per_sec:.1f} req/s**, "
                f"p50={agg.median_ms:.0f}ms p95={agg.p95_ms:.0f}ms p99={agg.p99_ms:.0f}ms, "
                f"failures={agg.failure_count}/{agg.request_count}"
            )
        if feed:
            lines.append(
                f"- `GET /api/feed`: **{feed.requests_per_sec:.1f} req/s**, "
                f"p50={feed.median_ms:.0f}ms p95={feed.p95_ms:.0f}ms p99={feed.p99_ms:.0f}ms"
            )
        lines.append("")
        return "\n".join(lines)

    def metrics_block(title: str, snap: MetricSnapshot | None) -> str:
        if not snap:
            return f"### {title}\n\nNot measured in this environment.\n"
        lines = [
            f"### {title}",
            "",
            f"- Window: **{snap.duration_s:.1f}s**",
            (
                f"- Cacheable Postgres queries: **{snap.cacheable:.0f}** "
                f"({qps(snap):.1f}/s) — feed `candidates`+`hydrate` plus post `get_posts`"
            ),
            (
                f"- Ranking-signal Postgres queries: **{snap.signals:.0f}** "
                f"({snap.signals / snap.duration_s:.1f}/s) — present in both runs"
            ),
            f"- Feed counters: `{snap.feed}`",
            f"- Post counters: `{snap.post}`",
            "",
        ]
        return "\n".join(lines)

    note_lines = "\n".join(f"- {note}" for note in notes) or "- None."
    return f"""# Cache comparison — {today}

Before/after Redis cache benchmark for Cascade GetFeed, following
`IMPLEMENTATION_PLAN.md` §13.3–13.4. Numbers below are **measured**, not targets.
Do not copy the plan's example "8,000+ req/s" or "80% reduction" unless this
file's Locust/Prometheus tables actually contain those values.

## Setup

- **Machine / environment:** {machine}
- **Feed Service instances:** {instances}
- **Latency threshold used to interpret throughput:** p99 < {latency_threshold_ms:.0f}ms
- **Read:write mix:** {read_write_ratio} (`GET /api/feed` : `POST /api/posts`)
- **Baseline config:** `FEED_BYPASS_CACHE=true` (candidates + hydration from PostgreSQL)
- **Cached config:** `FEED_BYPASS_CACHE=false` (Redis timelines + post cache, Post Service on miss)
- **Comparison metric:** `1 − (Postgres QPS with cache ÷ Postgres QPS baseline)` on
  `feed_postgres_queries_total{{op="candidates|hydrate"}}` + `post_postgres_queries_total{{op="get_posts"}}`

## Results

**Measured cacheable-Postgres load reduction: {reduction_pct}**

{locust_block("Baseline (cache bypassed)", baseline_locust)}
{metrics_block("Baseline Postgres counters", baseline_metrics)}
{locust_block("With Redis cache", cached_locust)}
{metrics_block("Cached Postgres counters", cached_metrics)}

## How to reproduce

```bash
make up && make smoke
make seed PRESET=ci
make warm-cache-compose

FEED_BYPASS_CACHE=true docker compose -f deploy/docker-compose.yml up -d --force-recreate feed-service
make benchmark LABEL=baseline USERS=50 DURATION=60s

FEED_BYPASS_CACHE=false docker compose -f deploy/docker-compose.yml up -d --force-recreate feed-service
make warm-cache-compose
make benchmark LABEL=cached USERS=50 DURATION=60s

cd loadtest && python benchmark.py compare \\
  --baseline-csv reports/baseline_stats.csv --cached-csv reports/cached_stats.csv \\
  --baseline-metrics reports/baseline_metrics.json --cached-metrics reports/cached_metrics.json
```

Ramp `-u` until GetFeed p99 crosses the latency threshold above; the last
concurrency that stayed under the threshold is the "sustaining N req/s" figure.

## Notes

{note_lines}
"""


def snapshot_to_json(snap: MetricSnapshot) -> dict:
    payload = asdict(snap)
    payload["cacheable"] = snap.cacheable
    payload["signals"] = snap.signals
    return payload


def load_metric_file(path: Path) -> MetricSnapshot:
    payload = json.loads(path.read_text())
    return MetricSnapshot(
        captured_at=payload["captured_at"],
        feed=payload.get("feed") or {},
        post=payload.get("post") or {},
        duration_s=float(payload.get("duration_s") or 0.0),
    )


def cmd_run(args: argparse.Namespace) -> int:
    reports = Path(args.reports_dir)
    reports.mkdir(parents=True, exist_ok=True)
    prefix = reports / args.label
    before: MetricSnapshot | None = None
    after: MetricSnapshot | None = None
    try:
        before = scrape_metrics(args.feed_metrics, args.post_metrics)
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        print(f"warning: could not scrape metrics before run: {exc}", file=sys.stderr)

    started = time.monotonic()
    code = run_locust(
        host=args.host,
        users=args.users,
        spawn_rate=args.spawn_rate,
        duration=args.duration,
        csv_prefix=prefix,
        locustfile=Path(args.locustfile),
        html=prefix.with_suffix(".html") if args.html else None,
    )
    elapsed = time.monotonic() - started
    try:
        after = scrape_metrics(args.feed_metrics, args.post_metrics)
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        print(f"warning: could not scrape metrics after run: {exc}", file=sys.stderr)

    if before and after:
        diff = delta(before, after)
        if diff.duration_s < 1:
            diff = MetricSnapshot(
                captured_at=diff.captured_at,
                feed=diff.feed,
                post=diff.post,
                duration_s=max(elapsed, 1e-6),
            )
        (prefix.with_name(f"{args.label}_metrics.json")).write_text(
            json.dumps(snapshot_to_json(diff), indent=2) + "\n"
        )
        print(
            f"{args.label} cacheable postgres queries={diff.cacheable:.0f} "
            f"qps={qps(diff):.1f} window={diff.duration_s:.1f}s",
            file=sys.stderr,
        )
    return code


def cmd_compare(args: argparse.Namespace) -> int:
    baseline_locust = parse_locust_stats_csv(Path(args.baseline_csv)) if args.baseline_csv else None
    cached_locust = parse_locust_stats_csv(Path(args.cached_csv)) if args.cached_csv else None
    baseline_metrics = load_metric_file(Path(args.baseline_metrics)) if args.baseline_metrics else None
    cached_metrics = load_metric_file(Path(args.cached_metrics)) if args.cached_metrics else None
    body = format_writeup(
        machine=args.machine,
        instances=args.instances,
        latency_threshold_ms=args.latency_threshold_ms,
        read_write_ratio="100:1",
        baseline_locust=baseline_locust,
        cached_locust=cached_locust,
        baseline_metrics=baseline_metrics,
        cached_metrics=cached_metrics,
        notes=args.note or [],
    )
    out = Path(args.write_docs)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(body)
    print(f"wrote {out}", file=sys.stderr)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command")

    run = sub.add_parser("run", help="Scrape metrics, run Locust, scrape again")
    run.add_argument("--label", required=True, help="baseline or cached (csv prefix)")
    run.add_argument("--host", default=os.environ.get("GATEWAY_URL", "http://localhost:8080"))
    run.add_argument("--users", type=int, default=50)
    run.add_argument("--spawn-rate", type=int, default=10)
    run.add_argument("--duration", default="30s")
    run.add_argument("--locustfile", default=str(ROOT / "locustfile.py"))
    run.add_argument("--reports-dir", default=str(ROOT / "reports"))
    run.add_argument("--feed-metrics", default=os.environ.get("FEED_METRICS_URL", "http://localhost:9101/metrics"))
    run.add_argument("--post-metrics", default=os.environ.get("POST_METRICS_URL", "http://localhost:9100/metrics"))
    run.add_argument("--html", action=argparse.BooleanOptionalAction, default=True)
    run.set_defaults(func=cmd_run)

    compare = sub.add_parser("compare", help="Write the Markdown comparison from saved artifacts")
    compare.add_argument("--baseline-csv", dest="baseline_csv")
    compare.add_argument("--cached-csv", dest="cached_csv")
    compare.add_argument("--baseline-metrics")
    compare.add_argument("--cached-metrics")
    compare.add_argument(
        "--write-docs",
        default=str(ROOT.parent / "docs" / "benchmarks" / "2026-08-13-cache-comparison.md"),
    )
    compare.add_argument(
        "--machine",
        default=os.environ.get("BENCHMARK_MACHINE", "unspecified (fill in CPU/RAM and container limits)"),
    )
    compare.add_argument(
        "--instances",
        default=os.environ.get("BENCHMARK_FEED_INSTANCES", "1 Feed Service replica"),
    )
    compare.add_argument("--latency-threshold-ms", type=float, default=200)
    compare.add_argument("--note", action="append")
    compare.set_defaults(func=cmd_compare)

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if not getattr(args, "command", None):
        parser.print_help()
        return 2
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
