---
okf_version: "0.1"
---

# Epics

Epics for the upstream sync program (`@earendil-works/pi-ai` v0.80.3 → v0.84.1,
frozen at `936aff00`), split from
[the scope](../planning/SCOPE.md). GitHub issue **#113** is the umbrella
tracking issue for the program; all nine epic issues are its sub-issues.

Epic 0 is the remediation lane: the repairs the nine review reports
([REPORT_1](../REPORT_1.md)–[REPORT_9](../REPORT_9.md)) turned up, which land before
Epic 1 opens a branch. It is temporary and is retired once its issues are done.

Every issue file's `depends_on:` is a list of zero-padded, quoted issue numbers
matching its own epic's `issue:` field (e.g. `depends_on: ["05", "13"]` referencing
`issue: 05` and `issue: 13`) — the form every issue in every epic uses. Write new
issues to this bundle in that form.

* [Epic 0: Plan remediation](/epic-0-plan-remediation/EPIC_0.md) - open, [#198](https://github.com/kern-ia/kern-link/issues/198)
* [Epic 1: Repository hygiene](/epic-1-repository-hygiene/EPIC_1.md) - open, [#118](https://github.com/kern-ia/kern-link/issues/118)
* [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md) - open, [#119](https://github.com/kern-ia/kern-link/issues/119)
* [Epic 3: Catalog schema and export tooling](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md) - open, [#120](https://github.com/kern-ia/kern-link/issues/120)
* [Epic 4: OpenAI-family adapters](/epic-4-openai-family-adapters/EPIC_4.md) - open, [#121](https://github.com/kern-ia/kern-link/issues/121)
* [Epic 5: Remaining adapters](/epic-5-remaining-adapters/EPIC_5.md) - open, [#122](https://github.com/kern-ia/kern-link/issues/122)
* [Epic 6: Auth core and env-API-key bindings](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md) - open, [#123](https://github.com/kern-ia/kern-link/issues/123)
* [Epic 7: Four new OAuth flows](/epic-7-four-new-oauth-flows/EPIC_7.md) - open, [#124](https://github.com/kern-ia/kern-link/issues/124)
* [Epic 8: pi-messages and radius](/epic-8-pi-messages-and-radius/EPIC_8.md) - open, [#125](https://github.com/kern-ia/kern-link/issues/125)
* [Epic 9: Classifier audit, disposition sweep, and release](/epic-9-classifier-audit-and-release/EPIC_9.md) - open, [#126](https://github.com/kern-ia/kern-link/issues/126)
* [Log](/log.md) - History of this bundle.
