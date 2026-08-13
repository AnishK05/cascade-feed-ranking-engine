#!/usr/bin/env python3
"""Seed Postgres with a power-law follow graph and an initial post batch.

Uses `COPY` (not REST) so 50k users + their edges stay tractable. After seeding,
run `make warm-cache` so Redis timelines exist before a cached Locust run.

Examples:
    python seed.py --preset ci --reset
    python seed.py --preset full --dsn postgres://cascade:cascade@localhost:5432/cascade
"""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import random
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path

import asyncpg

from seeding.graph import assign_users, iter_follow_edges
from seeding.presets import SeedPreset, get_preset

ROOT = Path(__file__).resolve().parent
DEFAULT_DSN = os.environ.get(
    "DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"
)
COPY_BATCH = 20_000


async def copy_records(
    conn: asyncpg.Connection, table: str, columns: list[str], records: list[tuple]
) -> None:
    if not records:
        return
    for start in range(0, len(records), COPY_BATCH):
        chunk = records[start : start + COPY_BATCH]
        await conn.copy_records_to_table(table, records=chunk, columns=columns)


async def reset_tables(conn: asyncpg.Connection) -> None:
    await conn.execute(
        "TRUNCATE TABLE public.engagements, public.posts, public.follows, public.users "
        "RESTART IDENTITY CASCADE"
    )


async def seed(conn: asyncpg.Connection, preset: SeedPreset, rng: random.Random) -> dict:
    users = assign_users(preset.degree, preset.celebrity_threshold, rng=rng)
    user_records = [
        (user.id, f"user_{user.id}", f"User {user.id}", user.is_celebrity, user.follower_count)
        for user in users
    ]
    await copy_records(
        conn,
        "users",
        ["id", "username", "display_name", "is_celebrity", "follower_count"],
        user_records,
    )
    await conn.execute(
        "SELECT setval(pg_get_serial_sequence('public.users', 'id'), "
        "(SELECT COALESCE(MAX(id), 1) FROM public.users))"
    )

    follow_records: list[tuple[int, int]] = []
    for follower_id, followee_id in iter_follow_edges(users, rng=rng):
        follow_records.append((follower_id, followee_id))
        if len(follow_records) >= COPY_BATCH:
            await copy_records(
                conn, "follows", ["follower_id", "followee_id"], follow_records
            )
            follow_records.clear()
    await copy_records(conn, "follows", ["follower_id", "followee_id"], follow_records)

    now = datetime.now(UTC)
    post_records: list[tuple] = []
    post_id = 1
    user_ids = [user.id for user in users]
    for user in users:
        n_posts = rng.randint(preset.posts_per_user_min, preset.posts_per_user_max)
        for _ in range(n_posts):
            age_hours = rng.randint(0, 72)
            created = now - timedelta(hours=age_hours, seconds=rng.randint(0, 3599))
            post_records.append(
                (
                    post_id,
                    user.id,
                    f"seeded post {post_id} from user {user.id}",
                    None,
                    created,
                )
            )
            post_id += 1
            if len(post_records) >= COPY_BATCH:
                await copy_records(
                    conn,
                    "posts",
                    ["id", "author_id", "content", "media_url", "created_at"],
                    post_records,
                )
                post_records.clear()
    await copy_records(
        conn,
        "posts",
        ["id", "author_id", "content", "media_url", "created_at"],
        post_records,
    )
    n_posts = post_id - 1
    if n_posts > 0:
        await conn.execute(
            "SELECT setval(pg_get_serial_sequence('public.posts', 'id'), "
            "(SELECT COALESCE(MAX(id), 1) FROM public.posts))"
        )

    like_records: list[tuple] = []
    engagement_id = 1
    for pid in range(1, n_posts + 1):
        if rng.random() > preset.engagement_like_rate:
            continue
        n_likes = rng.randint(1, preset.max_likes_per_post)
        likers = rng.sample(user_ids, k=min(n_likes, len(user_ids)))
        for liker in likers:
            like_records.append((engagement_id, pid, liker, "like"))
            engagement_id += 1
            if len(like_records) >= COPY_BATCH:
                await copy_records(
                    conn,
                    "engagements",
                    ["id", "post_id", "user_id", "type"],
                    like_records,
                )
                like_records.clear()
    await copy_records(
        conn, "engagements", ["id", "post_id", "user_id", "type"], like_records
    )
    if engagement_id > 1:
        await conn.execute(
            "SELECT setval(pg_get_serial_sequence('public.engagements', 'id'), "
            "(SELECT COALESCE(MAX(id), 1) FROM public.engagements))"
        )

    n_follows = await conn.fetchval("SELECT COUNT(*) FROM public.follows")
    n_celebrities = sum(1 for user in users if user.is_celebrity)
    return {
        "preset": preset.name,
        "num_users": len(users),
        "num_celebrities": n_celebrities,
        "num_follows": int(n_follows),
        "num_posts": n_posts,
        "num_engagements": engagement_id - 1,
        "celebrity_threshold": preset.celebrity_threshold,
        "user_ids": [user.id for user in users],
        "celebrity_ids": [user.id for user in users if user.is_celebrity],
    }


def write_sidecar(meta: dict, out_dir: Path) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "user_ids.json").write_text(json.dumps(meta["user_ids"]))
    sidecar = {k: v for k, v in meta.items() if k != "user_ids"}
    (out_dir / "seed_meta.json").write_text(json.dumps(sidecar, indent=2) + "\n")


async def amain(args: argparse.Namespace) -> int:
    preset = get_preset(args.preset)
    rng = random.Random(args.seed)
    conn = await asyncpg.connect(dsn=args.dsn)
    try:
        if args.reset:
            await reset_tables(conn)
        meta = await seed(conn, preset, rng)
    finally:
        await conn.close()

    write_sidecar(meta, Path(args.out_dir))
    print(
        f"seeded preset={meta['preset']} users={meta['num_users']} "
        f"celebrities={meta['num_celebrities']} follows={meta['num_follows']} "
        f"posts={meta['num_posts']} engagements={meta['num_engagements']} "
        f"threshold={meta['celebrity_threshold']}",
        file=sys.stderr,
    )
    print(
        "Fanout/Social Graph celebrity threshold must match this seed: "
        f"CELEBRITY_FOLLOWER_THRESHOLD={meta['celebrity_threshold']} "
        f"CELEBRITY_THRESHOLD={meta['celebrity_threshold']}",
        file=sys.stderr,
    )
    print("Next: make warm-cache  # rebuild Redis timelines from Postgres", file=sys.stderr)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--preset", default="ci", help="ci (500 users) or full (50k users)")
    parser.add_argument("--dsn", default=DEFAULT_DSN)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--reset", action=argparse.BooleanOptionalAction, default=True)
    parser.add_argument("--out-dir", default=str(ROOT / "data"))
    return parser


def main() -> int:
    return asyncio.run(amain(build_parser().parse_args()))


if __name__ == "__main__":
    raise SystemExit(main())
