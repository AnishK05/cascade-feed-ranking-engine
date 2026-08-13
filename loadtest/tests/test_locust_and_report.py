from pathlib import Path

from benchmark import format_writeup
from locustfile import CascadeUser
from seeding.reports import LocustStats


def test_locust_user_is_read_heavy():
    assert CascadeUser.get_feed.locust_task_weight == 100
    assert CascadeUser.create_post.locust_task_weight == 1


def test_format_writeup_states_unmeasured_and_ratio(tmp_path: Path):
    body = format_writeup(
        machine="test vm, 2 vCPU",
        instances="1 Feed Service replica",
        latency_threshold_ms=200,
        read_write_ratio="100:1",
        baseline_locust={
            "Aggregated": LocustStats(
                name="Aggregated",
                request_count=100,
                failure_count=0,
                median_ms=10,
                p95_ms=20,
                p99_ms=40,
                requests_per_sec=12.5,
            )
        },
        cached_locust=None,
        baseline_metrics=None,
        cached_metrics=None,
        notes=["Docker was not available in this environment."],
    )
    assert "100:1" in body
    assert "12.5 req/s" in body
    assert "Not measured in this environment." in body
    assert "8,000" not in body or "not targets" in body
    assert "Docker was not available" in body
    out = tmp_path / "writeup.md"
    out.write_text(body)
    assert out.read_text() == body
