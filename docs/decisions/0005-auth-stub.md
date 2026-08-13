# ADR 0005: Spoofable X-User-Id auth stub

## Context

The demo needs a "logged-in user" so feeds, follows, and posts are personalized. Real OAuth
or session auth would dominate the project without teaching fanout, caching, or ranking.

## Decision

The Gateway trusts `X-User-Id` for mutations and GetFeed. Missing or non-positive values
return 401. Creating users requires no header. This is documented in the README as a
deliberate non-boundary, not an oversight.

## Consequences

- The frontend user switcher is a `localStorage` integer.
- Locust can pick random seeded IDs without a login flow.
- This system must not be exposed beyond localhost / a local kind cluster.
