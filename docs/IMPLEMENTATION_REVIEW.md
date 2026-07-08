# Review Report: Go rebuild implementation vs plan and issues

**Plan files:** `docs/PLAN.md`, `docs/epics/**/EPIC_*.md`, `docs/epics/**/issues/*.md`  
**Reviewed:** 2026-07-08  
**Status:** CRITICAL ISSUES

---

## 1. Completeness

- [x] `ai/`, `ai/apis/*`, `ai/providers`, `ai/catalog`, `ai/images`, `cmd/pi-ai`, `upstream/`, and `.github/workflows/` exist in the planned shape.
- [x] All 14 epics are marked `status: done` in `docs/epics/index.md`, and the implementation has broad coverage for the planned adapters, catalog, OAuth flow package, CLI, and sync tooling.
- [ ] OAuth login is not complete end-to-end for normal provider use. `cmd/pi-ai login` persists real OAuth credentials via `ai/auth/oauth`, but the provider bindings that `ResolveProviderAuth` uses are still missing or stubbed.
- [ ] Google catalog/provider requests are not complete. Normal catalog models produce a doubled `/v1beta/v1beta/...` request URL; the live smoke test works around this with a per-test model copy.
- [ ] The plan's upstream traceability rule is incomplete. 72 Go files under `ai` and `cmd` do not contain a `// Ports:` header.

## 2. Spec Conformance

- File paths mostly match the planned layout. Documented layout deviations such as `ai/internal/httpx` utilities living in the `ai` root are recorded in `docs/PLAN.md` and `docs/PORTING.md`.
- OAuth provider conformance does not match Epic 12 + Epic 14. Epic 12's goal is OAuth credentials "feeding credentials into the auth core's stores for the adapters ... to consume"; Epic 14's login command depends on that. The flows exist in `ai/auth/oauth`, but provider bindings still do not consume them.
- Google adapter conformance does not match Epic 8/Epic 14 live-provider expectations. The catalog/provider base URL includes `/v1beta`, and `ai/apis/google.requestURL` appends `/v1beta` again.
- Upstream sync tooling appears present: `upstream/sync.sh`, `upstream/sync_test.sh`, `.github/workflows/test.yml`, and `.github/workflows/upstream-sync.yml` implement the planned weekly issue-opening workflow.

## 3. Architecture & SOLID Compliance

- [x] **Single Responsibility** — Package responsibilities are broadly separated by domain, adapter, provider binding, catalog, auth, and CLI.
- [x] **Open/Closed** — Provider/API dispatch is largely registry-based and extensible.
- [x] **Liskov Substitution** — No subtype substitutability issue found in this static pass.
- [x] **Interface Segregation** — `APIKeyAuth`, `OAuthAuth`, `Provider`, and stream interfaces are reasonably narrow.
- [ ] **Dependency Inversion** — Provider OAuth bindings currently depend on placeholder closures instead of the completed OAuth abstractions in `ai/auth/oauth`; this breaks the intended abstraction boundary between login flows and request auth resolution.
- [x] **Layer boundaries** — No obvious inward import from domain/core to adapter/provider layers found in this pass.
- [x] **Dependency direction** — The observed dependency direction follows the plan: `ai` core inward, adapters outward, providers as composition root, CLI on top.

## 4. Code Quality

- [ ] No obvious bugs or logic errors — Failed: saved OAuth credentials are unusable through provider resolution; Google catalog URLs are malformed.
- [ ] Error handling appropriate — Failed: OpenAI Codex and GitHub Copilot provider OAuth methods return "not implemented yet (Epic 12)" despite Epic 12 being done; Anthropic has no provider OAuth method at all.
- [ ] Test coverage for planned functionality — Failed: tests still assert provider OAuth stubs, and the Google live smoke explicitly works around the broken normal catalog/provider path.
- [x] No dead code or leftover scaffolding — Mostly pass, except the OAuth stub closures are leftover scaffolding in completed epics.
- [ ] Naming conventions consistent — Mostly pass, but traceability headers required by the plan are missing from 72 Go files.
- [x] No god classes or god functions — No broad god-object issue found in this static pass.

## 5. Issues

| Severity | File | Description |
|----------|------|-------------|
| CRITICAL | `ai/providers/anthropic.go:23`, `ai/providers/openai_codex.go:21`, `ai/providers/openai_codex.go:31`, `ai/providers/github_copilot.go:36`, `ai/providers/github_copilot.go:130`, `cmd/pi-ai/oauth.go:38` | OAuth login is split from provider auth resolution. `cmd/pi-ai login` uses real `oauth.AnthropicOAuth`, `oauth.CopilotOAuth`, and `oauth.CodexOAuth`, but provider bindings either expose no OAuth (`anthropic`) or return "not implemented yet (Epic 12)" from `Login`/`Refresh`/`ToAuth` (`openai-codex`, `github-copilot`). A stored OAuth credential therefore cannot be used by `ResolveProviderAuth`; for Anthropic, a stored OAuth credential also blocks ambient API-key fallback because `ResolveProviderAuth` returns nil when a stored credential type has no matching handler. Wire provider bindings to the completed `ai/auth/oauth` strategies. |
| CRITICAL | `ai/catalog/data/models/google.json:7`, `ai/providers/google.go:18`, `ai/apis/google/google.go:438`, `ai/apis/google/google.go:443`, `cmd/pi-ai/toolcallexample/live_smoke_test.go:34` | Google requests built from normal catalog/provider models double the version path. Catalog entries and the provider base URL include `https://generativelanguage.googleapis.com/v1beta`, while `requestURL` always appends `/v1beta/models/...`. The live smoke test acknowledges this and manually changes the model base URL, so the accepted path is only passing via a workaround. Fix either catalog/provider base URLs or `requestURL` normalization and add a regression test using `providers.Models(nil).GetModel("google", ...)`. |
| IMPORTANT | `docs/PLAN.md:88`, `docs/epics/epic-1-foundation/EPIC_1.md:56`, multiple Go files | The plan requires every ported Go file to carry a `// Ports: packages/ai/src/... @ <sha>` header. A static scan found 72 Go files under `ai` and `cmd` without any `// Ports:` header, including adapter/test files in Bedrock, Codex, Google, Mistral, OpenAI completions, and CLI examples. This weakens the standing upstream-sync workflow and violates repeated issue acceptance criteria. |

## 6. Verdict

**NEEDS FIXES**

The implementation is broad and much of the planned surface exists, but it should not be treated as plan-complete while OAuth credentials cannot flow from CLI login into provider request auth, and while the normal Google provider path produces malformed URLs. Fix those first, then clean up the missing `// Ports:` traceability headers so future upstream sync remains reviewable.

## Review Notes

This was a static review against the plan and issue documents. I did not run `go test ./...`; the `plan-review` workflow used here is intended to review conformance without modifying source code or executing the suite.
