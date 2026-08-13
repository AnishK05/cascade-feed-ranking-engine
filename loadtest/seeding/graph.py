"""Follow-graph construction for the Phase 12 seeder.

Turns a per-user follower-count vector into concrete `(follower_id, followee_id)`
edges. Counts are clamped to `num_users - 1` so a celebrity target of 40,000
followers is legal on the 50k-user dataset and still produces celebrities on the
scaled `--preset ci` graph.
"""

from __future__ import annotations

import random
from collections.abc import Iterator, Sequence
from dataclasses import dataclass

from seeding.degree_distribution import (
    DegreeDistributionConfig,
    generate_follower_counts,
    is_celebrity,
)


@dataclass(frozen=True)
class SeedUser:
    id: int
    follower_count: int
    is_celebrity: bool


def assign_users(
    config: DegreeDistributionConfig,
    threshold: int,
    rng: random.Random | None = None,
) -> list[SeedUser]:
    """Pair 1-indexed user IDs with follower counts.

    Celebrity counts occupy the first `num_celebrities` slots of the generated
    vector; user IDs are shuffled so those accounts are not always 1..N.
    """
    rng = rng or random.Random()
    counts = generate_follower_counts(config, rng=rng)
    user_ids = list(range(1, config.num_users + 1))
    rng.shuffle(user_ids)
    max_followers = max(config.num_users - 1, 0)
    users: list[SeedUser] = []
    for user_id, raw_count in zip(user_ids, counts, strict=True):
        count = min(int(raw_count), max_followers)
        users.append(
            SeedUser(
                id=user_id,
                follower_count=count,
                is_celebrity=is_celebrity(count, threshold),
            )
        )
    users.sort(key=lambda user: user.id)
    return users


def iter_follow_edges(users: Sequence[SeedUser], rng: random.Random | None = None) -> Iterator[tuple[int, int]]:
    """Yield `(follower_id, followee_id)` edges matching each user's follower_count."""
    rng = rng or random.Random()
    n = len(users)
    if n <= 1:
        return
    # assign_users sorts by id, so this is 1..n. Fall back to the actual ID list
    # if a caller passes a non-contiguous set.
    ids = [user.id for user in users]
    contiguous = ids == list(range(ids[0], ids[0] + n))
    for user in users:
        take = min(user.follower_count, n - 1)
        if take <= 0:
            continue
        if take == n - 1:
            for follower_id in ids:
                if follower_id != user.id:
                    yield follower_id, user.id
            continue
        chosen: set[int] = set()
        while len(chosen) < take:
            if contiguous:
                candidate = rng.randint(ids[0], ids[-1])
            else:
                candidate = ids[rng.randrange(n)]
            if candidate != user.id:
                chosen.add(candidate)
        for follower_id in chosen:
            yield follower_id, user.id
