---
okf_version: "0.1"
---

# kern-link planning

Planning bundle for kern-link, reverse-engineered from the codebase rather than
decided in interviews. It exists so brownfield planning skills have the same
inputs they would have had on a greenfield project. Consumer-facing
documentation lives in the sibling bundle at [`docs/`](../index.md).

* [Technical specs](SPECS.md) - A Go library exposing one streaming LLM interface across 35 providers — a domain core in package ai, nine wire adapters over net/http, an embedded model catalog, and file-backed credential resolution.
* [Conventions](CONVENTIONS.md) - How kern-link's code is actually written — upstream-provenance headers, receiver rules, typed errors whose text is load-bearing, stdlib-only tests, and Conventional Commits onto a develop integration branch.
* [Mapping decisions](mapping/index.md) - The two ambiguities the codebase could not answer for itself, and their verdicts.
* [Log](log.md) - History of this bundle.
