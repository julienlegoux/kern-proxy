---
okf_version: "0.1"
---

# kern-link planning

Planning bundle for kern-link, reverse-engineered from the codebase rather than
decided in interviews. It exists so brownfield planning skills have the same
inputs they would have had on a greenfield project. Consumer-facing
documentation lives in the sibling bundle at [`docs/`](../index.md).

* [Upstream sync scope](SCOPE.md) - Bringing the Go port to full parity with @earendil-works/pi-ai v0.84.1, shipped as a breaking v0.2.0 under the kern-ia module path.
* [Technical specs](SPECS.md) - A Go library exposing one streaming LLM interface across 35 providers — a domain core in package ai, nine wire adapters over net/http, an embedded model catalog, and file-backed credential resolution.
* [Conventions](CONVENTIONS.md) - How kern-link's code is actually written — upstream-provenance headers, receiver rules, typed errors whose text is load-bearing, stdlib-only tests, and Conventional Commits onto a develop integration branch.
* [Drift](DRIFT.md) - Standards the implementation has diverged from, promoted at each epic's close.
* [Mapping decisions](mapping/index.md) - The two ambiguities the codebase could not answer for itself, and their verdicts.
* [Upstream sync scope decisions](scope/index.md) - The 26-decision ledger behind SCOPE.md, with the impact-audit facts under each verdict.
* [Log](log.md) - History of this bundle.
