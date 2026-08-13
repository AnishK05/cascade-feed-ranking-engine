"""Named dataset sizes for `seed.py`.

`full` matches IMPLEMENTATION_PLAN.md §13.1 (50k users, ~50 celebrities with
10k–40k followers, celebrity threshold 10,000). `ci` is a scaled-down graph that
still has celebrities relative to a lowered threshold, so the hybrid fanout path
is exercised in tests and short local runs without copying millions of rows.
"""

from __future__ import annotations

from dataclasses import dataclass

from seeding.degree_distribution import DegreeDistributionConfig


@dataclass(frozen=True)
class SeedPreset:
    name: str
    degree: DegreeDistributionConfig
    celebrity_threshold: int
    posts_per_user_min: int = 5
    posts_per_user_max: int = 10
    engagement_like_rate: float = 0.2
    max_likes_per_post: int = 5


PRESETS: dict[str, SeedPreset] = {
    "ci": SeedPreset(
        name="ci",
        degree=DegreeDistributionConfig(
            num_users=500,
            num_celebrities=5,
            celebrity_min_followers=80,
            celebrity_max_followers=200,
            normal_min_followers=2,
            normal_max_followers=40,
        ),
        celebrity_threshold=80,
    ),
    "full": SeedPreset(
        name="full",
        degree=DegreeDistributionConfig(
            num_users=50_000,
            num_celebrities=50,
            celebrity_min_followers=10_000,
            celebrity_max_followers=40_000,
            normal_min_followers=50,
            normal_max_followers=300,
        ),
        celebrity_threshold=10_000,
    ),
}


def get_preset(name: str) -> SeedPreset:
    try:
        return PRESETS[name]
    except KeyError as exc:
        known = ", ".join(sorted(PRESETS))
        raise SystemExit(f"unknown preset {name!r}; choose one of: {known}") from exc
