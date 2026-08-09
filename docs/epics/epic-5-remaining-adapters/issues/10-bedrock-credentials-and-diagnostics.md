---
type: Issue
title: "bedrock: profile precedence over ambient keys, apiKey as a bearer token, and the response-failure diagnostic"
description: "Let an explicitly configured AWS profile win over ambient access keys, accept options.apiKey as a Bedrock bearer token, and attach a structured bedrock_response_failure diagnostic without touching errorMessage."
tags: [epic-5]
timestamp: 2026-08-09T09:45:42Z
epic: 5
issue: 10
slug: bedrock-credentials-and-diagnostics
size: M
status: open
gh_issue: 168
resource: https://github.com/kern-ia/kern-link/issues/168
depends_on: []
---

# bedrock: profile precedence over ambient keys, apiKey as a bearer token, and the response-failure diagnostic

## Summary

The auth and observability half of the Bedrock sync — three changes, all with a
named upstream failure behind them.

1. **A configured profile beats ambient access keys** (upstream issue #6957).
   The AWS SDK's default chain already prefers a configured profile over
   `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` — but *only when `credentials` is
   not set on the client config*. Setting it unconditionally defeats the chain.
   So upstream now captures the profile that came from pi's own auth flow and
   gates the credentials assignment on it:

   ```ts
   const optionsProfile = options.profile || options.env?.AWS_PROFILE;
   const config = { profile: optionsProfile || getProviderEnvValue("AWS_PROFILE", options.env) };
   // …
   if (!skipAuth && credentials && !optionsProfile) config.credentials = credentials;
   ```

   Note the asymmetry that makes this correct: `optionsProfile` reads only the
   **explicit** option and the **scoped** `options.env`, while the `config.profile`
   fallback also consults the ambient process environment. An *ambient*
   `AWS_PROFILE` does **not** suppress explicit credentials; an explicitly
   configured one does. `test/bedrock-credentials.test.ts`'s third case ("uses
   ambient AWS access keys when only an ambient profile is set") exists purely to
   pin that distinction — get it wrong and it fails.

2. **`options.apiKey` is accepted as a bearer token**, second in precedence:

   ```ts
   const bearerToken = options.bearerToken || options.apiKey || getProviderEnvValue("AWS_BEARER_TOKEN_BEDROCK", options.env) || undefined;
   ```

3. **A structured failure diagnostic.** On an errored (not aborted) turn,
   upstream attaches `{type: "bedrock_response_failure", timestamp, details}`
   with up to three keys — `status` (from `$metadata.httpStatusCode`),
   `errorCode` (the SDK error `name`, but **only** when it ends in `Exception`,
   so `TimeoutError` is never reported as a provider error code), and
   `requestId` (from `$metadata.requestId`, falling back to the request id
   captured off the successful `client.send()` response — declared *outside* the
   `try` for exactly that reason, since mid-stream exceptions carry no HTTP
   metadata). Values are trimmed, dropped when empty or over 200 characters
   ("a truncated request id is not a request id"), and the diagnostic is omitted
   entirely when no field survives.

   **`errorMessage` must stay byte-identical.** Upstream says so in a comment:
   `isRetryableAssistantError` matches against it. The diagnostic is *additional*
   structure, never a replacement — and kern-link has the same coupling through
   `ai/retry.go` ([CONVENTIONS.md](../../../planning/CONVENTIONS.md)). This is
   also the convention's own preferred shape: "Non-fatal problems attach to the
   message as `AssistantMessageDiagnostic` rather than being logged — the library
   imports no logger at all."

## Scope

- `ai/apis/bedrock/clientauth.go`:
  - `resolveClientConfig` (`:95`) — introduce the `optionsProfile` distinction:
    `firstNonEmptyString(opts.BedrockProfile, providerEnvValue("AWS_PROFILE", env))`
    where `env` is the **scoped** `opts.Env` only, separate from the existing
    `cfg.Profile` computation which may also fall through to ambient. Gate the
    `getConfiguredBedrockCredentials` assignment (`:147`) on `optionsProfile ==
    ""`. Do not disturb `hasAmbientConfiguredProfile` (`:123`) — its doc comment
    already records why it reads only the real process environment, and that
    stays true.
  - `bearerToken` (`:132`) — insert `opts.APIKey` between `opts.BedrockBearerToken`
    and the env var, preserving that exact order.
- `ai/apis/bedrock/errors.go` — the diagnostic builder:
  - `normalizeDiagnosticValue(string) (string, bool)` — trim; reject empty and
    anything over 200 characters (name the bound as a constant, with upstream's
    reason in the comment).
  - `extractBedrockErrorCode(error) string` — the SDK error name, only when it
    ends in `Exception`. Go's `aws-sdk-go-v2` surfaces modeled errors as typed
    structs rather than a `name` property; use `smithy.APIError.ErrorCode()` (or
    the equivalent seam already used by `formatBedrockError`, `:41`) and apply
    the same `Exception` suffix rule to whatever string it yields. Upstream also
    drops the SDK's literal `Unknown` placeholder — one of the nine upstream
    test cases asserts it.
  - `appendBedrockFailureDiagnostic(output, err, fallbackRequestId)` — build the
    details map, return early when empty, and attach via `ai.AppendDiagnostic`
    (`ai/diagnostics.go:66`) with `ai.NewAssistantMessageDiagnostic`
    (`:55`).
- `ai/apis/bedrock/bedrock.go` `run` (`:146`) — capture the response request id
  from the SDK response metadata into a variable declared **before** the request
  is issued, and call the diagnostic builder from the error path only when the
  reason is `error` (never `aborted`). `output.ErrorMessage` keeps coming from
  `formatBedrockError` unchanged.
- Tests, offline, in `ai/apis/bedrock/error_metadata_test.go` and alongside
  `clientauth_test.go`.
- `// Ports:` headers stay accurate.

## Out of scope

- Stop reasons, strict tools, and the Claude 5 matrix —
  [issue 09](/epic-5-remaining-adapters/issues/09-bedrock-stop-reasons-strict-tools-and-claude-5.md).
  The two PRs touch `bedrock.go`'s `run` from opposite ends (initializer/post-loop
  vs error path); order does not matter but rebasing does.
- Endpoint resolution (`shouldUseExplicitBedrockEndpoint`,
  `getStandardBedrockEndpointRegion`) — `test/bedrock-endpoint-resolution.test.ts`
  changed (+28/-…) but the corresponding source did not; read that test diff and
  report what it now asserts rather than assuming a source change is hiding.
- A new diagnostic *type* in `ai/diagnostics.go`. `AssistantMessageDiagnostic`
  already carries a free-form `details` map; `"bedrock_response_failure"` is just
  a type string.

## Acceptance criteria / Definition of done

- [ ] `TestExplicitProfileBeatsAmbientAccessKeys` — with `opts.BedrockProfile`
      set and ambient `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` present, the
      resolved client config carries the profile and **no** static credentials.
- [ ] `TestScopedAwsProfileBeatsAmbientAccessKeys` — the same when the profile
      arrives via `opts.Env["AWS_PROFILE"]` instead of the option.
- [ ] `TestAmbientProfileDoesNotSuppressAccessKeys` — with only an *ambient*
      `AWS_PROFILE` and ambient access keys, the credentials **are** set. This is
      the case that fails if the two profile lookups get collapsed into one.
- [ ] `TestApiKeyUsedAsBedrockBearerToken` — `opts.APIKey` becomes the bearer
      token when `BedrockBearerToken` is empty, and loses to it when both are
      set; `AWS_BEARER_TOKEN_BEDROCK` remains last.
- [ ] `TestResponseFailureDiagnosticRecordsStatusCodeAndRequestId` — a non-2xx
      from the SDK call attaches one `bedrock_response_failure` diagnostic whose
      details carry `status`, `errorCode` and `requestId`.
- [ ] `TestResponseFailureDiagnosticLeavesErrorMessageUntouched` — the
      `ErrorMessage` is byte-identical to the pre-change value for the same
      failure (assert against the literal string, not against
      `formatBedrockError`'s output, so a future edit to either is caught).
- [ ] `TestMidStreamExceptionReportsFallbackRequestId` — an exception delivered
      as a stream event, carrying no metadata of its own, still reports the
      request id captured from the successful response.
- [ ] `TestTransportFailureNameIsNotAnErrorCode` — an error named
      `TimeoutError` produces no `errorCode`; one named `ThrottlingException`
      does.
- [ ] `TestNoDiagnosticWithoutProviderMetadata` and
      `TestNoDiagnosticOnAbortedTurn` — no diagnostic is attached in either case.
- [ ] `TestOverlongDiagnosticValueIsDropped` — a 201-character request id yields
      no `requestId` key (dropped, not truncated), and the SDK's literal
      `Unknown` placeholder is not reported as a code.
- [ ] Ported upstream coverage: all nine cases of
      `test/bedrock-error-metadata.test.ts` and all three of
      `test/bedrock-credentials.test.ts` come across as discrete Go tests.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2). `gofmt -l .` prints nothing.
- [ ] Conventional Commit, e.g.
      `fix(bedrock): let a configured profile win over ambient AWS access keys`.

## Relevant files / areas

- `ai/apis/bedrock/clientauth.go` (211 lines) — `:95` `resolveClientConfig`,
  `:117` the `cfg.Profile` computation, `:123` `hasAmbientConfiguredProfile` and
  its doc comment, `:132` the bearer token, `:147` the credentials assignment,
  `:172` `getConfiguredBedrockCredentials`.
- `ai/apis/bedrock/bedrock.go:146` — `run`, its `fail` closure (`:155`) and the
  SDK send.
- `ai/apis/bedrock/errors.go` (78 lines) — `:41` `formatBedrockError`, `:62`
  `bedrockError`.
- `ai/diagnostics.go` — `:41` `ExtractDiagnosticError`, `:55`
  `NewAssistantMessageDiagnostic`, `:66` `AppendDiagnostic`.
- `ai/retry.go` — `isRetryableAssistantError`'s Go counterpart, the reason
  `ErrorMessage` is frozen.
- Upstream: `src/api/bedrock-converse-stream.ts` at `936aff00` — the
  `optionsProfile` hunk, the bearer-token chain, `normalizeDiagnosticValue`,
  `extractBedrockErrorCode`, `appendBedrockFailureDiagnostic`, and
  `responseRequestId`'s declaration outside the `try`.
- Upstream tests: `test/bedrock-error-metadata.test.ts` (new, 212 lines),
  `test/bedrock-credentials.test.ts` (new, 115 lines).

## Dependencies

- **Blocked by**: None. (`opts.APIKey` and `AssistantMessageDiagnostic` both
  already exist in `ai`.)
- **Blocks**: None.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.
