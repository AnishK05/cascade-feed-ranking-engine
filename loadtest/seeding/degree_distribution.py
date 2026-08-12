"""Follower-count distribution for the load test's seeded follow graph.

IMPLEMENTATION_PLAN.md §13.1 calls for a power-law/Zipfian follower distribution rather than a
uniform-random graph, specifically so the celebrity / hybrid-fanout code path (§5.3) actually
gets exercised by the benchmark instead of being dead code. This module contains that pure,
dependency-free generation logic so it can be unit tested without a database or network call;
Phase 12's actual seeding script (`loadtest/seed.py`) will call into this to build the dataset.
"""

from __future__ import annotations

import random
from dataclasses import dataclass


@dataclass(frozen=True)
class DegreeDistributionConfig:
    """Bounds for the two populations in the seeded follow graph."""

    num_users: int
    num_celebrities: int
    celebrity_min_followers: int = 10_000
    celebrity_max_followers: int = 40_000
    normal_min_followers: int = 0
    normal_max_followers: int = 300
    # Shape parameter for the ordinary-user power-law tail. Larger values concentrate more
    # mass near normal_min_followers (a "few friends" long tail), matching real social graphs
    # better than a uniform distribution would.
    zipf_exponent: float = 2.0

    def __post_init__(self) -> None:
        if self.num_celebrities > self.num_users:
            raise ValueError("num_celebrities cannot exceed num_users")
        if self.num_celebrities < 0 or self.num_users < 0:
            raise ValueError("counts must be non-negative")
        if self.celebrity_min_followers > self.celebrity_max_followers:
            raise ValueError("celebrity_min_followers must be <= celebrity_max_followers")
        if self.normal_min_followers > self.normal_max_followers:
            raise ValueError("normal_min_followers must be <= normal_max_followers")


def generate_follower_counts(
    config: DegreeDistributionConfig, rng: random.Random | None = None
) -> list[int]:
    """Return one follower count per seeded user.

    The first `config.num_celebrities` entries are celebrity accounts, drawn uniformly from
    `[celebrity_min_followers, celebrity_max_followers]`; the remainder are ordinary accounts
    drawn from a Zipf-shaped distribution capped at `normal_max_followers`. Callers should
    shuffle user IDs independently of this list's order if they don't want celebrities to be
    the first N user IDs.
    """
    rng = rng or random.Random()

    counts = [
        rng.randint(config.celebrity_min_followers, config.celebrity_max_followers)
        for _ in range(config.num_celebrities)
    ]

    num_normal = config.num_users - config.num_celebrities
    span = config.normal_max_followers - config.normal_min_followers
    for _ in range(num_normal):
        if span <= 0:
            counts.append(config.normal_min_followers)
            continue
        # random.Random has no zipfian sampler built in; approximate one by transforming a
        # uniform draw through a power curve so low follower-counts are far more common than
        # high ones, then clamp into range.
        u = rng.random()
        skewed = u**config.zipf_exponent
        counts.append(config.normal_min_followers + round(skewed * span))

    return counts


def is_celebrity(follower_count: int, threshold: int) -> bool:
    """Mirrors the Fanout Worker's own celebrity check (see
    `services/fanout-worker/internal/fanout/decision.go`), so the load test's dataset and the
    system under test agree on what counts as a celebrity.
    """
    return follower_count >= threshold
