# ranking/

Offline ranking-model training pipeline.

**Status: not part of core scope.** Per the Decisions Log in `IMPLEMENTATION_PLAN.md` (§19,
decision 5), an ML-trained ranking model is explicitly deprioritized for this project — the
core ranking layer is a heuristic scoring function implemented directly in Feed Service (Go),
described in §8.1 of the plan. This directory is kept as a placeholder for the optional,
unscheduled stretch goal described in §8.2 (synthetic engagement data generation + a small
scikit-learn model whose learned weights would be loaded by Feed Service), in case that's
picked up later.

There is nothing to run here yet.
