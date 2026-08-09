# Mapping decisions

Ambiguities the codebase could not answer for itself, surfaced while mapping
kern-link at `b70cba3`. Everything else in
[SPECS](/SPECS.md) and [CONVENTIONS](/CONVENTIONS.md) is a looked-up fact, not a
decision.

| # | Decision | Status | Recommendation / verdict |
|---|---|---|---|
| 01 | [Planning bundle nesting under docs/](01-planning-bundle-nesting.md) | decided | Nested sibling bundle — `docs/planning/` gets its own `index.md` + `log.md`, pointed at from `docs/index.md` |
| 02 | [Canonical test command](02-canonical-test-command.md) | decided | Two-tier — `-race` is the CI gate, local runs use `go test ./...` with an in-project `GOTMPDIR` |

Both accepted at triage on 2026-08-08.
