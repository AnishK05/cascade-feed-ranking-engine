import random
import time

from seeding.graph import assign_users, iter_follow_edges
from seeding.presets import get_preset


def test_ci_graph_stays_under_generation_budget():
    """Phase 16 lightweight perf guard: the 500-user graph must stay cheap in CI."""
    preset = get_preset("ci")
    started = time.perf_counter()
    users = assign_users(preset.degree, preset.celebrity_threshold, rng=random.Random(42))
    n_edges = sum(1 for _ in iter_follow_edges(users, rng=random.Random(42)))
    elapsed = time.perf_counter() - started
    assert len(users) == 500
    assert n_edges == 7922
    assert elapsed < 1.0, f"ci graph took {elapsed:.3f}s, want < 1s"


def test_full_assign_users_stays_under_budget():
    preset = get_preset("full")
    started = time.perf_counter()
    users = assign_users(preset.degree, preset.celebrity_threshold, rng=random.Random(42))
    elapsed = time.perf_counter() - started
    celebs = sum(1 for user in users if user.is_celebrity)
    assert len(users) == 50_000
    assert celebs == 50
    assert elapsed < 1.0, f"full assign_users took {elapsed:.3f}s, want < 1s"
