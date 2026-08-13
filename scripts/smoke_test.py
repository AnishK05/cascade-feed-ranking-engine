#!/usr/bin/env python3
"""End-to-end smoke: create users → follow → create post → fanout → follower feed.

Talks only to the Gateway (default http://localhost:8080). Intended to run after
`make up` once every service reports healthy.
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from datetime import UTC, datetime

GATEWAY = os.environ.get("GATEWAY_URL", "http://localhost:8080").rstrip("/")
TIMEOUT_S = float(os.environ.get("SMOKE_TIMEOUT_S", "45"))
POLL_S = float(os.environ.get("SMOKE_POLL_S", "0.5"))


def request(method: str, path: str, *, body: dict | None = None, user_id: int | None = None) -> tuple[int, object]:
    data = None if body is None else json.dumps(body).encode("utf-8")
    headers = {"Accept": "application/json"}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if user_id is not None:
        headers["X-User-Id"] = str(user_id)
    req = urllib.request.Request(GATEWAY + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            raw = response.read()
            payload: object = json.loads(raw.decode("utf-8")) if raw else None
            return response.status, payload
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        try:
            payload = json.loads(raw.decode("utf-8")) if raw else {"error": str(exc)}
        except json.JSONDecodeError:
            payload = {"error": raw.decode("utf-8", errors="replace")}
        return exc.code, payload


def must(status: int, payload: object, expected: int, what: str) -> object:
    if status != expected:
        raise SystemExit(f"{what} failed: HTTP {status} {payload}")
    return payload


def main() -> int:
    stamp = datetime.now(UTC).strftime("%Y%m%d%H%M%S%f")
    ping_status, ping = request("GET", "/api/ping")
    must(ping_status, ping, 200, "GET /api/ping")

    author = must(
        *request(
            "POST",
            "/api/users",
            body={"username": f"smoke_author_{stamp}", "displayName": "Smoke Author"},
        ),
        201,
        "create author",
    )
    follower = must(
        *request(
            "POST",
            "/api/users",
            body={"username": f"smoke_follower_{stamp}", "displayName": "Smoke Follower"},
        ),
        201,
        "create follower",
    )
    if not isinstance(author, dict) or not isinstance(follower, dict):
        raise SystemExit("user responses were not objects")
    author_id = int(author["id"])
    follower_id = int(follower["id"])

    must(
        *request(
            "POST",
            "/api/follows",
            body={"followeeId": author_id},
            user_id=follower_id,
        ),
        201,
        "follow",
    )

    content = f"smoke post {stamp}"
    created = must(
        *request("POST", "/api/posts", body={"content": content}, user_id=author_id),
        201,
        "create post",
    )
    if not isinstance(created, dict):
        raise SystemExit("create-post response was not an object")
    post_id = int(created["postId"])

    deadline = time.monotonic() + TIMEOUT_S
    last: object = None
    while time.monotonic() < deadline:
        status, feed = request("GET", "/api/feed", user_id=follower_id)
        last = (status, feed)
        if status == 200 and isinstance(feed, dict):
            items = feed.get("items") or []
            if any(int(item.get("postId", 0)) == post_id for item in items if isinstance(item, dict)):
                print(
                    f"ok: follower {follower_id} saw post {post_id} from author {author_id}",
                    file=sys.stderr,
                )
                return 0
        time.sleep(POLL_S)

    raise SystemExit(
        f"timeout after {TIMEOUT_S:.0f}s waiting for post {post_id} in follower {follower_id} feed; last={last}"
    )


if __name__ == "__main__":
    raise SystemExit(main())
