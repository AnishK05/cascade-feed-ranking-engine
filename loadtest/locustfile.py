"""Locust load profile for Cascade Gateway.

Read:write ratio is 100:1 (`GetFeed` vs `CreatePost`), matching a realistic
consumer-feed mix (IMPLEMENTATION_PLAN.md §13.2). User IDs come from the seeder
sidecar at `loadtest/data/user_ids.json` (or `LOADTEST_USERS_FILE` /
`LOADTEST_USER_IDS`).
"""

from __future__ import annotations

import json
import os
import random
from pathlib import Path

from locust import HttpUser, between, task

DEFAULT_USERS_FILE = Path(__file__).resolve().parent / "data" / "user_ids.json"


def load_user_ids(
    path: Path | None = None,
    raw: str | None = None,
) -> list[int]:
    env_raw = raw if raw is not None else os.environ.get("LOADTEST_USER_IDS", "")
    if env_raw.strip():
        return [int(part) for part in env_raw.split(",") if part.strip()]
    users_path = path or Path(os.environ.get("LOADTEST_USERS_FILE", DEFAULT_USERS_FILE))
    if not users_path.is_file():
        return [1]
    payload = json.loads(users_path.read_text())
    if isinstance(payload, dict):
        payload = payload.get("user_ids", [])
    return [int(user_id) for user_id in payload]


USER_IDS = load_user_ids()


class CascadeUser(HttpUser):
    # Short think-time so a small user count can still push the Gateway; raise
    # this if you want a more human-paced session mix.
    wait_time = between(0.01, 0.05)

    @task(100)
    def get_feed(self) -> None:
        user_id = random.choice(USER_IDS)
        self.client.get(
            "/api/feed",
            headers={"X-User-Id": str(user_id)},
            name="/api/feed",
        )

    @task(1)
    def create_post(self) -> None:
        user_id = random.choice(USER_IDS)
        self.client.post(
            "/api/posts",
            json={"content": f"loadtest post from {user_id}"},
            headers={"X-User-Id": str(user_id)},
            name="/api/posts",
        )
