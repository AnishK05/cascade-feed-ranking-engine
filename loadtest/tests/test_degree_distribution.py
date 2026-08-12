import random

import pytest

from seeding.degree_distribution import (
    DegreeDistributionConfig,
    generate_follower_counts,
    is_celebrity,
)


def test_generate_follower_counts_returns_one_entry_per_user():
    config = DegreeDistributionConfig(num_users=1_000, num_celebrities=10)
    counts = generate_follower_counts(config, rng=random.Random(42))

    assert len(counts) == config.num_users


def test_celebrity_entries_are_within_celebrity_bounds():
    config = DegreeDistributionConfig(
        num_users=500,
        num_celebrities=25,
        celebrity_min_followers=10_000,
        celebrity_max_followers=40_000,
    )
    counts = generate_follower_counts(config, rng=random.Random(7))

    celebrity_counts = counts[: config.num_celebrities]
    assert all(
        config.celebrity_min_followers <= c <= config.celebrity_max_followers
        for c in celebrity_counts
    )
    assert all(is_celebrity(c, threshold=10_000) for c in celebrity_counts)


def test_normal_entries_stay_within_bounds_and_are_not_celebrities():
    config = DegreeDistributionConfig(
        num_users=500,
        num_celebrities=25,
        normal_min_followers=0,
        normal_max_followers=300,
    )
    counts = generate_follower_counts(config, rng=random.Random(7))

    normal_counts = counts[config.num_celebrities :]
    assert all(0 <= c <= 300 for c in normal_counts)
    assert all(not is_celebrity(c, threshold=10_000) for c in normal_counts)


def test_distribution_is_skewed_not_uniform():
    """A uniform-random graph would defeat the point of testing the celebrity code path with
    realistic traffic; assert the ordinary-user distribution is meaningfully skewed toward low
    follower counts (median well below the midpoint of the allowed range).
    """
    config = DegreeDistributionConfig(
        num_users=10_000, num_celebrities=0, normal_min_followers=0, normal_max_followers=300
    )
    counts = generate_follower_counts(config, rng=random.Random(1))
    counts.sort()

    median = counts[len(counts) // 2]
    midpoint = (config.normal_min_followers + config.normal_max_followers) / 2
    assert median < midpoint * 0.5


def test_rejects_more_celebrities_than_users():
    with pytest.raises(ValueError):
        DegreeDistributionConfig(num_users=10, num_celebrities=20)


def test_rejects_inverted_bounds():
    with pytest.raises(ValueError):
        DegreeDistributionConfig(num_users=10, num_celebrities=1, celebrity_min_followers=100, celebrity_max_followers=50)
