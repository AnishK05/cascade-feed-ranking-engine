# loadtest/

Python tooling for seeding the 50,000-simulated-user dataset and running the before/after
Redis-cache benchmark described in `IMPLEMENTATION_PLAN.md` §12-13. Most of this package is
built out in Phase 12; Phase 0 only scaffolds the project and one piece of pure, testable logic
that Phase 12's seeding script will depend on: generating a realistic (power-law) follower
count distribution, so the celebrity/hybrid-fanout code path actually gets exercised by the
seeded data instead of being dead code under a uniform-random graph.

## Setup

```bash
cd loadtest
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt -r requirements-dev.txt
```

## Running tests

```bash
pytest
```
