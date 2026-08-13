import json
import random
from pathlib import Path

import pytest

from locustfile import load_user_ids
from seeding.degree_distribution import is_celebrity
from seeding.graph import assign_users, iter_follow_edges
from seeding.presets import get_preset
from seeding.reports import cache_reduction, parse_locust_stats_csv, parse_prometheus_counters


def test_assign_users_clamps_and_marks_celebrities():
    preset = get_preset("ci")
    users = assign_users(preset.degree, preset.celebrity_threshold, rng=random.Random(0))
    assert len(users) == preset.degree.num_users
    assert [user.id for user in users] == list(range(1, preset.degree.num_users + 1))
    celebs = [user for user in users if user.is_celebrity]
    assert len(celebs) == preset.degree.num_celebrities
    max_followers = preset.degree.num_users - 1
    assert all(0 <= user.follower_count <= max_followers for user in users)
    assert all(is_celebrity(user.follower_count, preset.celebrity_threshold) for user in celebs)


def test_follow_edges_match_in_degree():
    preset = get_preset("ci")
    users = assign_users(preset.degree, preset.celebrity_threshold, rng=random.Random(1))
    indegree: dict[int, int] = {user.id: 0 for user in users}
    seen: set[tuple[int, int]] = set()
    for follower_id, followee_id in iter_follow_edges(users, rng=random.Random(1)):
        assert follower_id != followee_id
        edge = (follower_id, followee_id)
        assert edge not in seen
        seen.add(edge)
        indegree[followee_id] += 1
    for user in users:
        assert indegree[user.id] == user.follower_count


def test_ci_preset_has_celebrities_after_clamp():
    preset = get_preset("ci")
    assert preset.celebrity_threshold < preset.degree.num_users
    users = assign_users(preset.degree, preset.celebrity_threshold, rng=random.Random(2))
    assert any(user.is_celebrity for user in users)
    assert any(not user.is_celebrity for user in users)


def test_full_preset_matches_plan_shape():
    preset = get_preset("full")
    assert preset.degree.num_users == 50_000
    assert preset.degree.num_celebrities == 50
    assert preset.celebrity_threshold == 10_000
    assert preset.degree.celebrity_min_followers == 10_000
    assert preset.degree.celebrity_max_followers == 40_000


def test_unknown_preset_exits():
    with pytest.raises(SystemExit):
        get_preset("nope")


def test_load_user_ids_from_env_and_file(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    monkeypatch.delenv("LOADTEST_USER_IDS", raising=False)
    path = tmp_path / "user_ids.json"
    path.write_text(json.dumps([3, 5, 8]))
    assert load_user_ids(path=path, raw="") == [3, 5, 8]
    assert load_user_ids(path=path, raw="9, 10") == [9, 10]
    missing = tmp_path / "missing.json"
    assert load_user_ids(path=missing, raw="") == [1]


def test_parse_prometheus_and_reduction():
    text = """
# HELP feed_postgres_queries_total PostgreSQL queries
feed_postgres_queries_total{op="candidates"} 30
feed_postgres_queries_total{op="hydrate"} 10
feed_postgres_queries_total{op="signals"} 20
post_postgres_queries_total{op="get_posts"} 4
"""
    feed = parse_prometheus_counters(text, "feed_postgres_queries_total")
    assert feed == {"candidates": 30.0, "hydrate": 10.0, "signals": 20.0}
    post = parse_prometheus_counters(text, "post_postgres_queries_total")
    assert post == {"get_posts": 4.0}
    assert cache_reduction(100, 20) == pytest.approx(0.8)
    assert cache_reduction(0, 1) is None


def test_parse_locust_stats_csv(tmp_path: Path):
    csv_path = tmp_path / "run_stats.csv"
    csv_path.write_text(
        "Type,Name,Request Count,Failure Count,Median Response Time,95%,99%,Requests/s\n"
        "GET,/api/feed,1000,2,12,40,90,250.5\n"
        "POST,/api/posts,10,0,30,80,120,2.5\n"
        "None,Aggregated,1010,2,13,41,91,253.0\n"
    )
    stats = parse_locust_stats_csv(csv_path)
    assert stats["/api/feed"].requests_per_sec == pytest.approx(250.5)
    assert stats["/api/feed"].p99_ms == pytest.approx(90)
    assert stats["Aggregated"].request_count == 1010
    assert stats["Aggregated"].failure_count == 2
