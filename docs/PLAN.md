# Rebuild `@earendil-works/pi-ai` in Go (kern-proxy)

## Context

`julienlegoux/kern-proxy` is empty (README only). The goal is a **full-parity Go rebuild** of [`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai) (v0.80.3, ~34k lines TS): a unified multi-provider LLM streaming library — unified message/event model, ~35 providers over 9 wire protocols, auth resolution (env / credential store / OAuth), token+cost tracking, model catalog, image generation, and an OAuth-login CLI. Same purpose, same functionality, no deviations; the port must be **kept in sync with upstream** over time. Method: TDD, SOLID, clean architecture.

Source snapshot for study is cloned at `…/scratchpad/pi-src/packages/ai` (re-clone if the container restarted). We skip only upstream's explicitly deprecated shims (`compat.ts` global API, `legacy-api-aliases.ts`), recorded as deviations in `PORTING.md`.

## Architecture

Module `github.com/julienlegoux/kern-proxy`. Dependency rule (clean architecture): `ai` (domain, zero deps) ← `ai/internal/*` ← `ai/apis/*` (adapters) ← `ai/providers` (composition root) ← `cmd`.

```
kern-proxy/
├── PORTING.md                    # TS-file → Go-package map + intentional deviations
├── upstream/UPSTREAM.lock        # pinned earendil-works/pi SHA + pi-ai version
├── upstream/sync.sh              # diff pinned..HEAD -- packages/ai, bucketed by area
├── tools/export-catalog/         # tiny tsx script: upstream models.generated.ts → JSON
├── ai/                           # domain: types, events, model, options, stream, cost, provider ifaces
├── ai/auth/                      # credentials, file store (~/.pi/agent/auth.json, flock), resolve, env map
├── ai/auth/oauth/                # anthropic (PKCE :53692), copilot (device code), codex (PKCE :1455 / device)
├── ai/internal/sse/              # hand-rolled SSE: CR/LF/CRLF, early-EOF detection
├── ai/internal/partialjson/      # strict → repair → partial → {} cascade (written from scratch)
├── ai/internal/jsonschema/       # santhosh-tekuri/jsonschema/v6 + ported coercion pass
├── ai/internal/httpx/            # retry classifier, overflow regexes, error-body normalize, proxy, zstd, unicode scrub
├── ai/apis/                      # transform.go (cross-provider normalizer), simpleopts.go, lazy.go
│   ├── anthropic/  openaicompletions/  openairesponses/ (+azure/, codex/)
│   ├── google/ (+vertex/)  mistral/  bedrock/
├── ai/providers/                 # registry (createProvider/createModels), all.go, ~35 thin bindings, faux/
├── ai/catalog/                   # go:embed data/*.json, BuiltinModel(s)/Providers loaders
├── ai/images/                    # parallel image-gen stack (openrouter adapter)
├── cmd/pi-ai/                    # CLI: login / list / help
└── testdata/                     # SSE fixtures copied verbatim from upstream test/data
```

### Key Go mapping decisions

- **Unions → sealed interfaces** with one struct per variant (`ContentPart`, `Message`, `Event`) and custom `MarshalJSON`/`UnmarshalJSON` discriminating on `"type"` so wire JSON stays byte-identical to TS.
- **EventStream → `Stream`**: buffered channel behind `Events() <-chan Event` plus `Result(ctx) *AssistantMessage`. **Errors stay in-band events** (`error{reason: aborted|error}`), never Go errors after invocation — this is the core contract, same as TS. `Complete = Stream().Result()`.
- **`lazyStream` pattern**: `Stream()` returns synchronously; auth resolution/setup runs in a goroutine; setup failure becomes an in-band `error` event with a zero-usage AssistantMessage.
- **AbortSignal → `context.Context`** as first param everywhere; `ctx` cancellation → `stopReason:"aborted"` (which `transform.go` then drops from replay).
- **Options**: plain structs (`*StreamOptions`, `SimpleStreamOptions` embedding it), not functional options — mirrors TS shape, keeps upstream diffs easy to port. `Transport`/`WebsocketConnectTimeout` are in base options (verified), so no per-api generics needed.
- **`Model`**: non-generic struct; `Compat` holds three optional pointers (`*OpenAICompletionsCompat` ~18 fields incl. 10-value thinkingFormat enum, `*OpenAIResponsesCompat`, `*AnthropicMessagesCompat`); tri-state flags are `*bool` (nil = auto-detect). Catalog test enforces sub-struct matches `Api`.
- **`Tool.parameters` → `json.RawMessage`** (schema stays a document); validation via jsonschema/v6 + a hand-port of upstream's coercion pass.
- **Transport**: raw `net/http` + own SSE for Anthropic/OpenAI/Google/Mistral (upstream's SDK use is incidental); **`aws-sdk-go-v2` bedrockruntime** for Bedrock (SigV4 + event-stream framing); `coder/websocket` + `klauspost/compress/zstd` for Codex (verified: WS transport with SSE fallback that requires zstd Content-Encoding decoding).
- No OTel needed — upstream observability is just `OnPayload`/`OnResponse` hooks + a `Diagnostics` slice.

### Catalog strategy (important)

**Do not port the ~2100-line `generate-models.ts` initially.** Its value is upstream's hand-maintained fixups. Instead: `tools/export-catalog` (a ~50-line tsx script run against the pinned upstream checkout) serializes `models.generated.ts` + `image-models.generated.ts` to JSON; Go embeds it via `go:embed`. Catalog sync becomes one command with zero Go churn and byte-parity with upstream. Providers with live model refresh (OpenRouter, Vercel gateway, NVIDIA, Copilot policy) get native Go `RefreshModels`. A native Go generator is an optional far-future phase.

### Fidelity-critical mechanics to preserve (per exploration)

- Anthropic: hand-rolled SSE, usage seeded at `message_start` and only non-null-overwritten at `message_delta`, "ended before message_stop" → retryable, OAuth **Claude Code impersonation** mode (identity system block, beta headers, `user-agent: claude-cli/…`, tool-name remap), adaptive thinking, cache_control placement; **1h cache-write billed at 2× input** in `calculateCost`.
- OpenAI completions: compat auto-detection from provider/baseUrl (~18 flags serving ~15 vendors), 10 thinking-format encodings, `max_tokens` vs `max_completion_tokens`, anthropic-style cache_control replication, session-affinity headers, tool-call delta correlation by **both index and id** (dual maps), usage = `prompt_tokens − cached − cache_write`.
- Partial tool-arg JSON re-parsed on every delta (strict → repair → partial → `{}`); scratch buffers stripped before persistence.
- `transform-messages`: same-model keeps thinking+signatures; cross-model downgrades thinking→text, remaps tool-call ids; synthesizes `"No result provided"` toolResults for orphans; skips errored/aborted assistant turns.
- Auth precedence: explicit override → stored credential (OAuth refresh with **double-checked locking under `CredentialStore.Modify`**, file store `~/.pi/agent/auth.json` 0600/0700 + flock) → ambient env only when nothing stored. Ambient sentinels for Vertex ADC and Bedrock AWS auth matrix. `expires` = epoch-ms minus 5-min margin (except Codex).

## Phases (dependency-ordered, TDD gates)

Per phase: port the relevant upstream tests first (red) → implement (green) → golden-request snapshots via `OnPayload` compared against TS-captured goldens.

| # | Phase | Deliverables | Test gate | Size |
|---|---|---|---|---|
| 1 | Foundation | `ai` types/events/options/cost/stream; `internal/{sse,partialjson,httpx}` | union JSON goldens vs TS wire format; SSE + partial-json fixture suites; cost tables (`tokens`, `anthropic-cache-write-1h-cost`) | M |
| 2 | Faux + registry | `providers/faux` (the executable spec of the event contract), `Provider`/`Models` registry, `lazy.go`, `simpleopts.go` | ports of `faux-provider`, `stream`, `abort` tests — event contract frozen | M |
| 3 | Auth core | credential types, InMemory + flock file store, resolve precedence, env-key map + sentinels | `oauth-auth`, `env-api-keys` ports; concurrent-refresh race test (`-race`) | S–M |
| 4 | Transform + validation | cross-provider normalizer, jsonschema+coercion, unicode scrub | `transform-messages-*`, `validation`, `lax-message-content`, `tool-call-id-normalization`, `unicode-surrogate` ports | M |
| 5 | Anthropic adapter | full anthropic-messages incl. OAuth impersonation, thinking, retry/overflow integration | ~12 `anthropic-*` test ports + fixtures + goldens; env-gated live smoke | L |
| 6 | OpenAI completions | quirk matrix, thinking formats, prompt cache, dual-map correlation, usage math | ~10 `openai-completions-*` ports + goldens | L |
| 7 | Responses family | openai-responses + shared, azure, codex (WebSocket, zstd SSE fallback, connection-limit retry, JWT accountId) | responses/codex ports; WS integration test against local ws server | L–XL |
| 8 | Google + Vertex | google-shared converters, thinking signatures, raw HTTP; Vertex ADC via `golang.org/x/oauth2/google` | `google-*` ports | M–L |
| 9 | Mistral | mistral-conversations, raw HTTP | `mistral-*` ports | S–M |
| 10 | Bedrock | aws-sdk-go-v2 ConverseStream; full AWS auth matrix incl. bearer token; thinking payloads | `bedrock-*` ports | M–L |
| 11 | Catalog + all providers | export-catalog tool, `ai/catalog` embed, ~35 provider bindings, RefreshModels impls | catalog validation ports (`models-runtime`, `providers`, per-provider model tests) | M |
| 12 | OAuth flows | anthropic PKCE/callback+manual-code race, copilot device-code+token-exchange+per-cred baseUrl, codex dual-flow | `oauth-device-code`, copilot/codex oauth ports; manual live login check | M |
| 13 | Images | parallel image stack + openrouter-images adapter | images test ports | S |
| 14 | CLI + sync tooling | `cmd/pi-ai`, `PORTING.md` complete, `upstream/sync.sh` + CI job | full suite + env-gated e2e green | S |

Effort weighting: phases 5–7 ≈ 45%, 1–4 ≈ 25%, rest ≈ 30%. Each phase ends with a commit + push to `claude/writing-plans-go-rebuild-dvggn6`.

## Upstream-sync workflow (standing)

1. `upstream/UPSTREAM.lock` pins the `earendil-works/pi` commit SHA + pi-ai version (start: v0.80.3 snapshot).
2. Every ported Go file carries `// Ports: packages/ai/src/… @ <sha>`; `PORTING.md` holds the full mapping + deviations.
3. `upstream/sync.sh`: fetch upstream, `git diff <pinned>..origin/main -- packages/ai`, bucket by area, print mapped Go packages. Catalog-only diffs → "rerun export-catalog + catalog tests".
4. Sync procedure: run script → port each semantic change with its new tests → regen catalog JSON → bump lock SHA → full suite + gated live smokes.
5. Weekly CI job (or Routine) runs the diff and opens an issue when non-empty; Go tags mirror upstream releases (`v0.80.3-go.N`).

## Key risks

1. **`internal/partialjson`** — no Go equivalent of the strict→repair→partial cascade; written from scratch in phase 1, fixture-locked.
2. **Codex transport** — WS + zstd SSE fallback + per-session fallback memory; highest-fidelity-risk adapter (phase 7).
3. **Bedrock bearer-token auth** — verify expressible in aws-sdk-go-v2, else hand-roll that one mode.
4. **JSON-schema coercion** — custom pass; lock with upstream's `validation.test.ts`.
5. **SSE edge cases** feed the retry classifier — must be bit-exact (fixtures exist).

## Verification

- Default `go test ./... -race`: all ported unit tests green with no network (faux provider + httptest fixtures).
- Golden-request parity: outgoing payloads captured via `OnPayload` match goldens generated from the TS library for the same Context/options.
- Env-gated live smokes (mirroring upstream `skipIf`): with `ANTHROPIC_API_KEY`/`OPENAI_API_KEY` etc. set, run tagged e2e tests per adapter phase.
- End-to-end: `cmd/pi-ai list`, then a small example program streaming a tool-call round-trip through faux and (if a key is present) a real provider.