# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-07-10

### Added

- `doc.go` package overview for the root `ai` package, runnable examples for
  streaming, tool calling, and cost tracking, README badges, and this changelog.

### Changed

- Renamed the repository and Go module from `kern-proxy` to
  `github.com/julienlegoux/kern-link`. GitHub redirects the old repository
  URL, but new imports should use the new module path.

### Removed

- Dev-process planning documents (`docs/epics/`, `docs/PLAN.md`) no longer
  ship in the module; they are preserved in git history.

## [0.1.0] - 2026-07-09

First tagged release: a full-parity Go port of
[`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai),
hardened by an external adoption review.

### Added

- Unified message model (`ai.Context`, `ai.Message`) and typed streaming event
  protocol across 35 providers (Anthropic, OpenAI Completions/Responses,
  Google Gemini & Vertex, Mistral, AWS Bedrock, Azure, GitHub Copilot, OpenAI
  Codex, OpenRouter, Groq, xAI, …).
- Credential resolution: per-provider env keys, a cross-process-safe
  credential store, and OAuth flows (Claude Pro/Max, GitHub Copilot,
  ChatGPT Plus/Pro) via the `pi-ai login` CLI.
- Embedded model catalog with per-model pricing, `ai.CalculateCost`, token
  estimation, and cache-read/write accounting.
- Session persistence: conversations round-trip through `encoding/json` and
  resume on any provider.
- In-process `faux` provider exercising the full streaming contract offline.
- `golangci-lint` in CI alongside race-enabled tests and weekly upstream-drift
  detection.
- Credential-mode documentation covering the terms-of-service risk of
  API-key vs. subscription-OAuth paths.

### Fixed

- `StreamOptions.MaxRetries`/`MaxRetryDelay` are honored by every adapter
  (shared HTTP retry helper with exponential backoff, `Retry-After` support,
  and non-retryable quota/billing short-circuit); Bedrock maps them onto the
  AWS SDK retryer.
- `Stream.Events` takes a `context.Context` and can no longer leak its pump
  goroutine when a consumer stops ranging early.
- All message types use pointer receivers consistently.

[Unreleased]: https://github.com/julienlegoux/kern-link/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/julienlegoux/kern-link/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/julienlegoux/kern-link/releases/tag/v0.1.0
