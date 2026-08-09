# Log

## 2026-08-08

* **Creation**: mapped kern-link at `b70cba3` — wrote [SPECS](SPECS.md) and
  [CONVENTIONS](CONVENTIONS.md) from the manifests, folder tree, source, tests,
  CI workflows, and git history. The sweep produced facts almost everywhere;
  two genuine ambiguities went to the [mapping ledger](mapping/index.md) and
  were both accepted at triage: the planning bundle nests as a sibling OKF root
  under `docs/` rather than folding into the consumer bundle
  ([01](mapping/01-planning-bundle-nesting.md)), and the canonical test command
  is two-tier — `-race` as the CI gate, a `GOTMPDIR`-scoped plain run locally
  ([02](mapping/02-canonical-test-command.md)).
* **Note**: recorded Unknowns rather than guesses — no deployment artifacts (the
  module is consumed with `go get`), no observability wiring in the library by
  design, no coverage configuration, and no visible branch-protection or review
  requirements.
