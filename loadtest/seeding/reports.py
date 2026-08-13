"""Parse Locust CSV output and Prometheus text exposition for the cache comparison."""

from __future__ import annotations

import csv
import re
from dataclasses import dataclass
from pathlib import Path

PROMETHEUS_LINE = re.compile(
    r'^(?P<name>[a-zA-Z_:][a-zA-Z0-9_:]*)\{?(?P<labels>[^}]*)\}?\s+(?P<value>[-+0-9.eE]+)\s*$'
)


@dataclass(frozen=True)
class LocustStats:
    name: str
    request_count: int
    failure_count: int
    median_ms: float
    p95_ms: float
    p99_ms: float
    requests_per_sec: float


def parse_prometheus_counters(text: str, metric_name: str) -> dict[str, float]:
    """Return `{op: value}` for a `*_total` counter with an `op` label."""
    values: dict[str, float] = {}
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        match = PROMETHEUS_LINE.match(line)
        if match is None or match.group("name") != metric_name:
            continue
        labels = _parse_labels(match.group("labels"))
        op = labels.get("op", "")
        values[op] = float(match.group("value"))
    return values


def _parse_labels(raw: str) -> dict[str, str]:
    if not raw:
        return {}
    labels: dict[str, str] = {}
    for part in raw.split(","):
        if "=" not in part:
            continue
        key, value = part.split("=", 1)
        labels[key.strip()] = value.strip().strip('"')
    return labels


def sum_ops(values: dict[str, float], ops: tuple[str, ...]) -> float:
    return sum(values.get(op, 0.0) for op in ops)


def cache_reduction(baseline_qps: float, cached_qps: float) -> float | None:
    """`1 - (cached / baseline)` as a fraction. None if baseline is zero."""
    if baseline_qps <= 0:
        return None
    return 1.0 - (cached_qps / baseline_qps)


def parse_locust_stats_csv(path: Path) -> dict[str, LocustStats]:
    """Parse Locust `--csv` `*_stats.csv` into per-endpoint stats plus Aggregated."""
    result: dict[str, LocustStats] = {}
    with path.open(newline="") as handle:
        reader = csv.DictReader(handle)
        for row in reader:
            name = (row.get("Name") or row.get("name") or "").strip()
            if not name:
                continue
            result[name] = LocustStats(
                name=name,
                request_count=_csv_int(row, "Request Count", "request_count"),
                failure_count=_csv_int(row, "Failure Count", "failure_count"),
                median_ms=_csv_float(row, "Median Response Time", "median_response_time"),
                p95_ms=_csv_percentile(row, "95%"),
                p99_ms=_csv_percentile(row, "99%"),
                requests_per_sec=_csv_float(
                    row, "Requests/s", "requests_per_s", "Requests/s"
                ),
            )
    return result


def _csv_int(row: dict[str, str], *keys: str) -> int:
    return int(float(_csv_field(row, *keys) or "0"))


def _csv_float(row: dict[str, str], *keys: str) -> float:
    return float(_csv_field(row, *keys) or "0")


def _csv_percentile(row: dict[str, str], label: str) -> float:
    # Locust 2.x uses columns like "95%" and "99%".
    if label in row and row[label] not in (None, "", "N/A"):
        return float(row[label])
    return _csv_float(row, label)


def _csv_field(row: dict[str, str], *keys: str) -> str:
    for key in keys:
        if key in row and row[key] not in (None, ""):
            return row[key]
    lowered = {k.lower(): v for k, v in row.items()}
    for key in keys:
        value = lowered.get(key.lower())
        if value not in (None, ""):
            return value
    return "0"
