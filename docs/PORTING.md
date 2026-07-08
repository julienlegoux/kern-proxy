# Porting map: @earendil-works/pi-ai → kern-proxy (Go)

kern-proxy is a full-parity Go rebuild of
[`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai).
The pinned upstream revision lives in `upstream/UPSTREAM.lock`; use
`upstream/sync.sh` to diff upstream changes since the pin.

Every ported Go file carries a `// Ports: packages/ai/src/...` header comment
naming its upstream source.

## File mapping

| Upstream (packages/ai) | Go | Status |
|---|---|---|
| `src/types.ts` | `ai/types.go`, `ai/model.go`, `ai/options.go`, `ai/events.go` | ported |
| `src/utils/event-stream.ts` | `ai/stream.go` | ported |
| `src/models.ts` (calculateCost, thinking helpers) | `ai/cost.go`, `ai/model.go` | ported |
| `src/models.ts` (Provider/Models/createProvider) | `ai/provider.go`, `ai/lazy.go` | ported |
| `src/utils/retry.ts` | `ai/retry.go` | ported |
| `src/utils/overflow.ts` | `ai/overflow.go` | ported |
| `src/utils/estimate.ts` | `ai/estimate.go` | ported |
| `src/utils/hash.ts` | `ai/hash.go` | ported (hash-compatible) |
| `src/utils/sanitize-unicode.ts` | `ai/sanitize.go` | ported |
| `src/utils/headers.ts` | `ai/headers.go` | ported |
| `src/utils/diagnostics.ts` | `ai/diagnostics.go` | ported |
| `src/utils/json-parse.ts` + npm `partial-json` | `ai/internal/partialjson` | ported |
| SSE parsing (from `src/api/anthropic-messages.ts`) | `ai/internal/sse` | ported |
| `src/providers/faux.ts` | `ai/providers/faux` | ported |
| `src/api/lazy.ts` | `ai/lazy.go` | ported (module lazy-loading is a TS concern; only lazyStream semantics carry over) |
| `src/api/*.lazy.ts` (`anthropic-messages`, `azure-openai-responses`, `bedrock-converse-stream`, `google-generative-ai`, `google-vertex`, `mistral-conversations`, `openai-codex-responses`, `openai-completions`, `openai-responses`, `openrouter-images`) | n/a | not ported: each is a trivial `lazyApi(() => import("./x.ts"))` wrapper around `src/api/lazy.ts`'s dynamic import, one per adapter, for bundler code-splitting — the same TS-bundler concern as `src/api/lazy.ts` itself, with no Go equivalent; every adapter package above is imported directly |
| `src/api/simple-options.ts` | `ai/apis/simpleopts.go` | ported |
| `src/auth/types.ts`, `src/auth/resolve.ts`, `src/auth/credential-store.ts`, `src/auth/context.ts` | `ai/auth.go`, `ai/resolve.go`, `ai/credentialstore.go`, `ai/authcontext.go` (in package `ai`: the Provider interface references ProviderAuth, so a separate package would cycle) | ported |
| persistent auth.json store (from `packages/coding-agent/src/core/auth-storage.ts`) | `ai/auth/filestore.go` | ported |
| `src/auth/helpers.ts` | `ai/auth/helpers.go` | ported (phase 11, issue 02: `EnvAPIKeyAuth` backs the 8 core provider bindings below. `lazyOAuth` is not ported — it exists only to keep Node-only OAuth flow code out of bundles via a dynamic import, a TS-bundler concern with no Go equivalent; Go providers construct their `ai.OAuthAuth` directly) |
| `src/utils/oauth/*`, `src/oauth.ts` | `ai/auth/oauth` | ported (phase 12, issue 01: the shared scaffolding reused by every flow — `GeneratePKCE` (`pkce.ts`), `CallbackServer` (`anthropic.ts`'s `startCallbackServer`, generalized so Codex's `:1455` callback can reuse it), `PostJSON` (`anthropic.ts`'s `postJson`), and the `oauthSuccessHTML`/`oauthErrorHTML` callback pages (`oauth-page.ts`) — plus the Anthropic PKCE flow itself (`anthropic.ts`): authorize-URL building, the callback-server-vs-manual-code race (`loginAnthropic`), token exchange/refresh with the 5-minute `expires` margin, and the `AnthropicOAuth` `ai.OAuthAuth` value. `device-code.ts` (device-code grant polling) is out of scope here; issues 02 (Copilot) and 03 (Codex) port it as their first consumers. **Deviation**: `ai/providers/anthropic.go`'s `ProviderAuth.OAuth` still has no wiring to `AnthropicOAuth` — the issue's Relevant-files scope covers only `ai/auth/oauth/`, not the provider binding; wiring it in is a natural follow-up for the CLI `login` command (Epic 14) or a small epic-12 close-out. Issue 02 (Copilot device code): `device-code.ts`'s `pollOAuthDeviceCodeFlow` ported generically as `PollDeviceCodeFlow[T]` (`devicecode.go`) — RFC 8628 section 3.5 `slow_down` back-off (server-provided interval when given, else +5s), an injectable `clock`/`deviceCodeSleep` pair so tests fast-forward a virtual timeline instead of sleeping — plus `github-copilot.ts`'s device-code grant (`startCopilotDeviceFlow`, including the `verification_uri` http(s)-only validation/normalization), the GitHub-token → Copilot-token exchange and refresh (`exchangeCopilotToken`), and per-credential `baseUrl` derived from the Copilot token's `proxy-ep` or a stored enterprise domain (`getGitHubCopilotBaseURL`), as the `CopilotOAuth` `ai.OAuthAuth` value (`copilot.go`). **Deviation**: upstream's post-login model-policy bookkeeping — `enableGitHubCopilotModel`/`enableAllGitHubCopilotModels` and `fetchAvailableGitHubCopilotModelIds`/`isSelectableCopilotModel`/`CopilotCredentials.availableModelIds` (the `githubCopilotOAuthProvider.modifyModels` catalog filtering) — is not ported: it's this issue's "policy/RefreshModels" out-of-scope area, already covered by `ai/providers/github_copilot.go`'s `RefreshModels` (Epic 11) against the `COPILOT_GITHUB_TOKEN` env fallback; wiring that same filtering to an OAuth-derived token, and wiring `CopilotOAuth` itself into `ai/providers/github_copilot.go` (which still stubs `Login`/`Refresh`/`ToAuth`), is a natural follow-up for the CLI `login` command (Epic 14) or a small epic-12 close-out, matching the Anthropic deviation above. Issue 03 (Codex dual flow, epic close-out): `openai-codex.ts`'s dual login — the PKCE/callback-server flow on `:1455` (`loginOpenAICodex`, reusing `StartCallbackServer`/`GeneratePKCE`/the shared `parseAuthorizationInput`, racing a manual-code prompt exactly like the Anthropic flow) and the device-code alternative (`loginOpenAICodexDeviceCode`, reusing `PollDeviceCodeFlow`), selected by an `ai.AuthPromptSelect` prompt — as `CodexOAuth` (`codex.go`). Token exchange/refresh (`exchangeAuthorizationCode`/`refreshAccessToken` → `codexRequestToken`) store `expires` **without** the 5-minute margin, the one exception in this package, matching upstream's `readTokenResponse` verbatim. `accountId` is extracted from the access token's JWT payload (`decodeJwt`/`getAccountId` → `codexDecodeAccountID`) and round-trips through `OAuthCredential.Extra`. **Deviation**: upstream's `fetchWithLoginCancellation` (which maps an aborted fetch to a "Login cancelled" error) is applied inconsistently upstream — the authorization-code exchange uses it, the plain refresh call does not; this port applies the equivalent ctx-cancellation-to-`ErrDeviceCodeCancelled` mapping uniformly to both, since Go's ctx-based cancellation makes that the more natural shape here. **Not ported**: upstream's legacy `openaiCodexOAuthProvider` object (the pre-`ai.OAuthAuth` provider-interface shape) — like the Anthropic/Copilot issues before it, only the `ai.OAuthAuth`-shaped `openaiCodexOAuth` export (`CodexOAuth`) is a live consumer in this port. **Deviation** (matching the Anthropic/Copilot ones above): `ai/providers/openai_codex.go`'s `ProviderAuth.OAuth` still stubs `Login`/`Refresh`/`ToAuth` with a "not implemented yet (Epic 12)" error — wiring it to `CodexOAuth` is this epic's shared follow-up, deferred to the CLI `login` command (Epic 14) per this issue's "Out of scope".) |
| `src/api/transform-messages.ts` | `ai/apis/transform.go` | ported |
| `src/api/cloudflare.ts` (base-URL templates) | `ai/catalog/data/models/*.json`'s per-model `{CLOUDFLARE_ACCOUNT_ID}`/`{CLOUDFLARE_GATEWAY_ID}`-templated `baseUrl`, resolved by `ai/providers/cloudflare_auth.go`'s `resolveCloudflareBaseURL` | ported (distributed like the rest of the generated catalog — see the `src/providers/*.models.ts` row below — rather than as Go source constants) |
| `src/api/github-copilot-headers.ts` | none | **not ported**: the dynamic `X-Initiator`/`Copilot-Vision-Request` request headers (agent-vs-user turn detection, image-content detection) have no Go equivalent yet in `ai/providers/github_copilot.go` or `ai/apis/openaicompletions`/`ai/apis/anthropic`/`ai/apis/openairesponses` (whichever wire adapter a given Copilot model dispatches to). Follow-up, out of this issue's scope — same category as row 50's Google `baseUrl` doubling bug below |
| `src/utils/validation.ts`, `src/utils/typebox-helpers.ts` | `ai/internal/jsonschema` | ported (typebox-helpers' StringEnum is a TypeBox schema-authoring convenience; plain JSON Schema documents need no equivalent) |
| `src/api/anthropic-messages.ts` | `ai/apis/anthropic` | ported (phase 5, issues 01–04: core, adaptive thinking, cache_control, OAuth Claude Code impersonation, and retry/overflow classifier + HTTP error-body wiring) |
| `src/api/openai-completions.ts`, `openai-prompt-cache.ts` | `ai/apis/openaicompletions` | ported (phase 6, issues 01-04. Issue 01: core adapter — request/message building, dual-map tool-call correlation, partial-JSON re-parsing, prompt/cache usage math. Issue 02: `detectCompat`/`getCompat` vendor auto-detection matrix (~18 flags) and `max_tokens`/`max_completion_tokens` selection, wired into request/message building for the flags already read (store, developer role, strict mode, usage-in-streaming, tool-result name, synthetic post-tool-result assistant message). Issue 03: the 10-value `thinkingFormat` enum's per-format request encoding (`reasoning_effort`/`thinking`/`enable_thinking`/`chat_template_kwargs`/`reasoning`/`tool_stream`), same-model thinking-block replay (plain text under `requiresThinkingAsText`, else the signature-named field plus `requiresReasoningContentOnAssistantMessages`), and Google-style `reasoning_details` encode/decode on tool calls. Issue 04 (epic close-out): `prompt_cache_key`/`prompt_cache_retention` (`openai-prompt-cache.ts`'s clamp, ported to `promptcache.go`), Anthropic-style `cache_control` replication for `compat.cacheControlFormat === "anthropic"` vendors, session-affinity headers (`session_id`/`x-client-request-id`/`x-session-affinity`), and the env-gated `OPENAI_API_KEY` live smoke) |
| `src/api/openai-responses.ts`, `openai-responses-shared.ts` | `ai/apis/openairesponses` | ported (phase 7, issue 01: request building — input items, tools, reasoning config — and SSE stream decode into unified events; `ConvertMessages`/`ConvertTools`/`DecodeStream` exported for reuse by the Azure and Codex variants below) |
| `src/api/azure-openai-responses*.ts` | `ai/apis/azure` | ported (phase 7, issue 02: endpoint/deployment-name resolution — `AZURE_OPENAI_BASE_URL`/`AZURE_OPENAI_RESOURCE_NAME`/`AZURE_OPENAI_API_VERSION`/`AZURE_OPENAI_DEPLOYMENT_NAME_MAP` env fallbacks, Azure-host base-URL normalization to `/openai/v1` — and its own buildParams over the shared `ConvertMessages`/`ConvertTools`/`DecodeStream` from `ai/apis/openairesponses`; the provider binding/catalog entry is deferred to epic 11) |
| `src/api/openai-codex-responses*.ts` | `ai/apis/codex` | ported (phase 7, issue 03: request/auth over the shared `ai/apis/openairesponses` core on the plain HTTP/SSE path -- JWT `chatgpt_account_id` extraction, Codex's own request shaping (unconditional `store`/`stream`/`instructions`/`text.verbosity`/`include`/`tool_choice`/`parallel_tool_calls`, no `max_output_tokens`, no automatic default-off reasoning fallback), session-affinity headers, prompt-cache-key clamping, per-request HTTP retry with exponential backoff and `retry-after(-ms)` handling, and the Codex-specific service-tier resolution (`resolveCodexServiceTier`, now a `ResolveServiceTier` hook on the shared `openairesponses.ServiceTierOptions`). Issue 04: the WebSocket transport (`github.com/coder/websocket`) tried first for any `Transport` other than `"sse"`, with its connection-limit-reached retry-once behavior and per-session fallback memory (`ai/apis/codex/websocket.go`); the SSE path (both the direct `"sse"` transport and the WebSocket-failure fallback) now always zstd-compresses its request body (`github.com/klauspost/compress/zstd`, `ai/apis/codex/zstd.go`), matching the Codex backend's own compressed traffic. **Not ported**: upstream's WebSocket connection *cache* (reusing an open connection across separate requests in the same session, with its busy/idle/age-limit bookkeeping) and the resulting `"websocket-cached"` delta/`previous_response_id` continuation mode -- every request here opens and closes its own connection, so `"auto"`/`"websocket-cached"` always send the full request body, same as `"websocket"`; see `ai/apis/codex/websocket.go`'s package doc for the reasoning) |
| `src/api/google-generative-ai.ts`, `google-shared.ts` | `ai/apis/google` | ported (phase 8, issue 01: google-shared converters (contents, tools, generation config) and thinking-signature handling, exported as `ConvertMessages`/`ConvertTools`/`DecodeStream` for the Vertex variant (issue 02) to reuse; streaming decode into unified events and API-key auth, over raw `net/http` + `ai/internal/sse` against the Generative Language REST API directly (no Go equivalent of the `@google/genai` SDK -- see "Vendor SDKs" below)) |
| `src/api/google-vertex.ts` | `ai/apis/google/vertex` | ported (phase 8, issue 02: Vertex endpoint shaping (project/location-scoped REST path for ADC, a project-less publisher path for an explicit API key) over `ai/apis/google`'s shared `ConvertMessages`/`ConvertTools`/`DecodeStream`, plus the thinking-level/budget helpers and `wireRequest` shape duplicated from that package -- matching upstream's own duplication between `google-generative-ai.ts` and `google-vertex.ts`, since only `google-shared.ts` is actually shared upstream. Application Default Credentials via `golang.org/x/oauth2/google` (`google.DefaultTokenSource`), reached through an injectable package-level `adcTokenFunc` (mirroring `ai/resolve.go`'s `authClock` stubbing pattern) so tests cover ADC resolution with a fake token source, no live GCP credentials needed. The `gcp-vertex-credentials` sentinel and `<placeholder>`-shaped API keys both fall back to ADC, matching `resolveApiKey`/`isPlaceholderApiKey`. **Deviation**: the exact REST path the `@google/genai` SDK builds when a custom `httpOptions.baseUrl` is combined with project/location is an internal SDK detail with no vendored source to introspect; this port instead defines and tests its own contract -- a genuine `baseUrl` override replaces the default host outright, and a catalog `baseUrl` template still containing `{location}` is ignored in favor of the default host) |
| `src/api/mistral-conversations.ts` | `ai/apis/mistral` | ported (phase 9: request building — messages, tools, tool choice, prompt_mode/reasoning_effort reasoning controls, prompt_cache_key/x-affinity session caching — and SSE stream decode into unified events over raw `net/http` + `ai/internal/sse`, since there is no Go equivalent of the `@mistralai/mistralai` SDK (see "Vendor SDKs" below); the provider binding/catalog entry is deferred to epic 11) |
| `src/api/bedrock-converse-stream.ts`, `src/bedrock-provider.ts` | `ai/apis/bedrock` | ported (phase 10, issue 01: Converse request building — messages, tools, inference config, system-prompt/last-user-message prompt-cache points — and `ConverseStream` event decode into unified events, plus Claude thinking-payload encode/decode (adaptive and budget-based, GovCloud display omission, interleaved-thinking beta), over `aws-sdk-go-v2`'s `bedrockruntime` client with default-credential-chain auth only. Issue 02 (epic close-out): the full AWS auth matrix — a pure `resolveClientConfig` (`ai/apis/bedrock/clientauth.go`) resolves explicit access-key/secret/session-token credentials, profile, region (inference-profile ARN extraction including GovCloud, built-in endpoint-derived region, ambient-profile precedence quirk), custom endpoint pinning, and bearer-token auth from `BedrockOptions`-equivalent fields plus the ambient `AWS_*` env vars, and `newBedrockRuntimeClient` (`ai/apis/bedrock/client.go`) applies it over `aws-sdk-go-v2`'s own default credential chain for whatever it leaves unset — plus the custom-headers Build-step middleware (`ai/apis/bedrock/headers_middleware.go`). **Risk #3 resolved SDK-native**: `bedrockruntime.Options` exposes both `BearerAuthTokenProvider` and `AuthSchemePreference`, and its generated `auth.go` already advertises `smithy.api#httpBearerAuth` alongside SigV4 for every operation, so bearer-token auth needed no hand-rolled HTTP signing) |
| `src/providers/*.ts` (bindings), `src/providers/all.ts` | `ai/providers` | ported (phase 11. Issue 01: `getBuiltinModel`/`getBuiltinModels`/`getBuiltinProviders`'s static-catalog-read portion only, as `ai/catalog`'s `BuiltinModel`/`BuiltinModels`/`Providers`. Issue 02: the 8 bindings fronting a first-party adapter package — `anthropic.ts`, `openai.ts`, `azure-openai-responses.ts`, `openai-codex.ts`, `google.ts`, `google-vertex.ts`, `mistral.ts`, `amazon-bedrock.ts` — as `AnthropicProvider`/`OpenAIProvider`/`AzureOpenAIResponsesProvider`/`OpenAICodexProvider`/`GoogleProvider`/`GoogleVertexProvider`/`MistralProvider`/`AmazonBedrockProvider`, plus `all.go`'s `Providers`/`Models` aggregating them (the upstream `builtinProviders`/`builtinModels` names collide with `ai/catalog`'s already-taken `BuiltinModels`, hence the shorter Go names). The custom Vertex/Bedrock `ApiKeyAuth` resolvers and the Anthropic OAuth-token-precedence env list are ported verbatim. **Deviation**: `openai-codex.ts`'s only auth strategy upstream is `lazyOAuth` into a real login flow; since `ai/auth/oauth` (Epic 12) doesn't exist yet, `OpenAICodexProvider`'s `ai.OAuthAuth` advertises the same `Name` (for status UI) but its `Login`/`Refresh`/`ToAuth` are stubs returning a "not implemented yet (Epic 12)" error — `ResolveProviderAuth` already reports "unconfigured" for a provider with no stored credential regardless, so this has the same observable behavior as upstream before a user has logged in. Issue 03 (epic close-out): the remaining ~27 compat-vendor bindings, bringing the total to upstream's ~35 — `ant-ling.ts`, `cerebras.ts`, `cloudflare-ai-gateway.ts`+`cloudflare-workers-ai.ts`+`cloudflare-auth.ts`, `deepseek.ts`, `fireworks.ts`, `github-copilot.ts`, `groq.ts`, `huggingface.ts`, `kimi-coding.ts`, `minimax.ts`+`minimax-cn.ts`, `moonshotai.ts`+`moonshotai-cn.ts`, `nvidia.ts`, `opencode.ts`+`opencode-go.ts`, `openrouter.ts`, `together.ts`, `vercel-ai-gateway.ts`, `xai.ts`, `xiaomi.ts`+the three `xiaomi-token-plan-*.ts`, `zai.ts`+`zai-coding-cn.ts` — most thin declarations over the already-ported `openaicompletions`/`anthropic` (and, for `opencode.ts`, `google`/`openairesponses` too) adapters, dispatching per-model via `ApiByProtocol` where a binding fronts more than one. `github-copilot.ts`'s OAuth strategy is stubbed the same way as `openai-codex.ts` above (Epic 12 pending). **Deviation — native `RefreshModels`**: no upstream provider file wires a `refreshModels` hook (docs/PLAN.md's phase-11 note asks for one anyway, for OpenRouter/Vercel AI Gateway/NVIDIA/GitHub Copilot); `ai/providers/openrouter.go` and `vercel_ai_gateway.go` reuse the response shape and tool-capable-only transform documented by upstream's *build-time* catalog generator's `fetchOpenRouterModels`/`fetchAiGatewayModels` (`packages/ai/scripts/generate-models.ts`, itself explicitly out of this epic's scope) as a runtime fetch instead, constructing fresh `ai.Model` entries from each vendor's public `/models` listing; `nvidia.go` and `github_copilot.go` instead narrow the embedded static catalog down to currently-available models (NVIDIA via `fetchNvidiaNimModelIds`'s id-normalization rule, Copilot via `github-copilot.ts`'s OAuth-flow `isSelectableCopilotModel` "policy" rule reused against the `COPILOT_GITHUB_TOKEN` env-key fallback instead of a live OAuth token) since neither live endpoint carries the cost/context-window metadata needed to fabricate a whole new catalog entry) |
| `src/providers/*.models.ts`, `src/models.generated.ts`, `src/image-models.generated.ts` | `ai/catalog/data/models/*.json`, `ai/catalog/data/images/*.json` (via `tools/export-catalog`) | ported (phase 11, issue 01: `tools/export-catalog/export-catalog.ts` serializes both generated catalogs from the pinned upstream checkout to JSON; `ai/catalog` embeds `data/models/*.json` via `go:embed` and exposes the loaders above, plus a runtime `Model.Compat`/`Api` consistency check standing in for upstream's compile-time-only `Model<TApi>["compat"]` discriminated union. `data/images/*.json` was exported and embedded for byte-parity in phase 11; phase 13, issue 01 added its loader — `ai/catalog/images.go`'s `ImagesProviders`/`ImagesData` (raw bytes only, to avoid a cycle back through `ai/images`), decoded into `ai/images.Model` by `ai/images/catalog.go`) |
| `src/env-api-keys.ts` (per-provider env-var precedence, e.g. `ANTHROPIC_OAUTH_TOKEN` before `ANTHROPIC_API_KEY`) | `ai/providers/*.go` via `auth.EnvAPIKeyAuth` (`ai/auth/helpers.go`) | ported (distributed: each provider binding passes its own env-var list to the shared helper rather than one central map — no `ai/auth/env.go` file exists, or is needed. The aggregate introspection helpers `findEnvKeys`/`getEnvApiKey` — including the Vertex-ADC-file-exists and Bedrock-ambient-credential checks — have no Go consumer: no CLI surface here needs a standalone "is this provider configured?" check outside of an actual `ResolveProviderAuth` call, so they are not ported) |
| `src/images*.ts`, `src/api/openrouter-images.ts`, `src/providers/openrouter-images.ts` | `ai/images` | ported (phase 13, issue 01, the whole epic: `images.ts`'s api-registry-dispatching `generateImages` (registry.go's `GenerateImages`/`RegisterAPIProvider`, ported from `images-api-registry.ts`), `images-models.ts`'s auth-resolving `ImagesModels`/`ImagesProvider` collection (provider.go, reusing `ai.ResolveProviderAuth` through a small `*ai.Model` field-overlap shim since `ImagesModel` isn't `ai.Model`), `api/openrouter-images.ts`'s adapter (openrouter.go, raw `net/http` — no Go equivalent of the `openai` SDK, same "Vendor SDKs" deviation as every other adapter), and `providers/openrouter-images.ts` plus `providers/all.ts`'s `builtinImagesProviders`/`builtinImagesModels` (builtin.go's `OpenRouterProvider`/`BuiltinProviders`/`BuiltinModels`). The embedded `data/images/*.json` catalog (Epic 11) finally gets a loader: raw bytes from `ai/catalog` (catalog.go's new `ImagesProviders`/`ImagesData`, kept byte-only to avoid an `ai/catalog` <-> `ai/images` import cycle) decoded into typed `images.Model` values by `ai/images/catalog.go`'s `CatalogModels`/`CatalogModel` (the `image-models.ts` reader, `getImageModels`/`getImageModel`/`getImageProviders`). **Deviation**: `api/openrouter-images.lazy.ts`'s lazy dynamic import and `images-api-registry.ts`'s side-effect-import registration (`providers/images/register-builtins.ts`) are TS code-splitting concerns with no Go equivalent (matching the already-noted `api/lazy.ts` deviation); `openrouter.go` registers itself directly via `init()`. **Deviation**: upstream's plain `generateImages` throws synchronously when no adapter is registered for `model.api` (a config bug, not a request failure); this port folds that case into the same in-band `AssistantImages{StopReason: "error"}` result every adapter already promises never to error on, for one consistent "never fails" contract across the package — no ported test exercises the alternate branch.) |
| `src/cli.ts` | `cmd/pi-ai` | ported (phase 14, issue 01: `login`/`list`/`help` argv dispatch (`main.go`), each OAuth flow's shared interactive `ai.AuthLoginCallbacks` implementation over the three ported strategies — `AnthropicOAuth`/`CopilotOAuth`/`CodexOAuth` (`oauth.go`, ai/auth/oauth, Epic 12) — persisting to the shared `ai/auth.FileCredentialStore` (Epic 3) rather than upstream's local `auth.json`, and a provider/model listing (`list.go`) over the full embedded catalog (ai/providers, Epic 11) rather than upstream's narrower OAuth-providers-only `list`. Plus the epic's e2e acceptance criterion: `cmd/pi-ai/toolcallexample`'s `Run` streams a tool-call round trip (deterministic against `ai/providers/faux`, env-gated against live Gemini when `GEMINI_API_KEY` is set) and `cmd/pi-ai/example`'s runnable wrapper. **Follow-up bug found, not fixed here** (out of this issue's scope): `ai/catalog/data/models/google.json`'s Google models already carry `"baseUrl": ".../v1beta"`, but `ai/apis/google`'s `requestURL` (phase 8) appends `/v1beta/models/...` again, doubling the path segment and 404ing against the live API through the normal catalog/provider path — the live smoke test above works around it by overriding the request model's `BaseURL` to the bare host.) |
| `src/utils/error-body.ts` | inline per-adapter (`ai/apis/anthropic`, `ai/apis/openaicompletions`'s `httpStatusError`, `ai/apis/openairesponses`, `ai/apis/bedrock/errors.go`'s `formatBedrockError`, ...) | ported (distributed, not centralized: `ai/internal/httpx` was never created as a package — see Epic 1's layout-deviation note — so each adapter composes its own status+body error text inline against its own wire shape instead of sharing one `normalizeProviderError`/`formatProviderError`. `ai/apis/anthropic` reads Anthropic's already-"carries the body" error text directly; `ai/apis/bedrock`'s version reads `smithy.APIError` directly rather than probing an arbitrary shape, documented separately below) |
| `src/utils/node-http-proxy.ts` | Go `net/http`'s built-in `ProxyFromEnvironment` (used automatically by every adapter's zero-value `http.Client{}`) | ported (no dedicated Go file needed: `net/http`'s default transport already implements the same `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` precedence and hostname/port `no_proxy` matching this file hand-rolls for Node; see the Bedrock proxy-agent deviation below for the one adapter that constructs a non-default HTTP client) |
| `src/session-resources.ts` | `ai/sessionresources.go` | ported |

## Intentional deviations

- **`src/compat.ts` and `src/legacy-api-aliases.ts` are not ported.** Both are
  deprecated back-compat shims for pre-`createModels()` JS callers; the Go port
  targets the `Models`/`createProvider` design directly.
- **Cancellation**: TS `AbortSignal` → `context.Context` passed as the first
  argument of `Stream`/`Complete`. Context cancellation produces the in-band
  `error` event with StopReason `"aborted"`, same as an aborted signal.
- **Durations**: TS `timeoutMs`/`maxRetryDelayMs` numbers → `time.Duration`
  fields (`Timeout`, `MaxRetryDelay`). Options are not part of the persisted
  wire format, so this does not affect interop.
- **Compat struct**: the three per-api TS compat interfaces are merged into one
  flat Go `ai.Compat` struct (the wire JSON is a flat object in both ports);
  adapters read only their own fields.
- **Per-adapter options struct**: TS's `AnthropicOptions extends StreamOptions`
  (and sibling per-api options interfaces) rely on function overloading that
  Go's single `StreamFunc` signature can't express. Anthropic-specific request
  knobs (`ThinkingEnabled`, `ThinkingBudgetTokens`, `Effort`,
  `ThinkingDisplay`) are merged directly into `ai.StreamOptions` instead,
  mirroring the Compat merge above; other adapters ignore fields they don't
  read.
- **Vendor SDKs**: the TS package delegates transport to @anthropic-ai/sdk,
  openai, @google/genai, @mistralai/mistralai. The Go port uses raw `net/http`
  plus `ai/internal/sse` for those protocols, and `aws-sdk-go-v2` for Bedrock
  (SigV4 + event-stream framing). Vertex's Application Default Credentials
  path is the one exception needing a real Google-authored dependency rather
  than raw `net/http`: `golang.org/x/oauth2/google` performs the actual ADC
  discovery/token-refresh algorithm (well-known credentials file, metadata
  server, ...), which would be impractical to hand-roll faithfully.
- **`src/utils/provider-env.ts` Bun sandbox fallback** (/proc/self/environ) is
  not ported; Go reads the process environment directly.
- **`src/utils/abort-signals.ts`** (combineAbortSignals) is not ported;
  compose `context.Context` instead.
- **Codex tool `strict` flag**: upstream's Codex adapter calls
  `convertResponsesTools(tools, { strict: null })`, sending a literal
  `"strict": null` on every tool. `ai/apis/codex` reuses the shared
  `openairesponses.ConvertTools` with its default `false` instead, sending
  `"strict": false`. No upstream or ported test asserts on this field's exact
  wire value; revisit if a future golden needs the literal `null`.
- **Mistral cached-prompt-token probing**: upstream's
  `getMistralCachedPromptTokens` defensively probes six differently-cased
  usage field variants (`promptTokensDetails`/`prompt_tokens_details`/
  `numCachedTokens`/...) across SDK versions. `ai/apis/mistral` probes only
  the documented `prompt_tokens_details.cached_tokens` wire field, since this
  port talks one fixed REST shape directly rather than an SDK whose JS
  bindings evolved across versions.
- **Mistral auth check has no header escape valve**: unlike the sibling
  anthropic/google adapters' `assertRequestAuth` (which accept a
  caller-supplied auth header in lieu of `apiKey`), `ai/apis/mistral` requires
  a non-empty `apiKey` unconditionally, matching upstream's own unconditional
  `if (!apiKey) throw ...` in `mistral-conversations.ts`'s `stream` function.
- **Bedrock "unknown content type" tests have no Go equivalent input**:
  upstream's `bedrock-convert-messages.test.ts` includes cases feeding a
  message an `{ type: "unknown", ... }` content block to verify it's skipped
  rather than throwing. `ai.UserContentPart`/`ai.AssistantContentPart` are
  closed Go interfaces that structurally cannot hold an unrecognized block
  type, so those specific cases aren't portable; the surrounding
  blank/placeholder-handling behavior they were bundled with is ported and
  tested in `ai/apis/bedrock/messages_test.go`.
- **Bedrock unrecognized image MIME type**: upstream's `createImageBlock`
  throws synchronously on an unrecognized `mimeType`. `ai/apis/bedrock`
  passes an unrecognized value through unchanged instead, letting Bedrock's
  own request validation reject it (surfacing as a normal error event via
  `formatBedrockError`), rather than threading an error return through every
  message-conversion function for a case no upstream or ported test
  exercises.
- **Bedrock error formatting reads `smithy.APIError` directly**: upstream's
  `formatBedrockError` uses `normalizeProviderError` to defensively probe an
  arbitrary thrown value's shape, because JS has no static error typing. The
  AWS Go SDK returns errors implementing `smithy.APIError`
  (`ErrorCode`/`ErrorMessage`/`ErrorFault`) uniformly for both request-level
  failures and stream-level exceptions, so `ai/apis/bedrock` reads those
  directly instead of re-implementing upstream's shape-probing.
- **Bedrock custom-headers middleware and full endpoint/region resolution**
  (deferred from issue 01) are ported in issue 02: both
  `bedrock-custom-headers.test.ts` and `bedrock-endpoint-resolution.test.ts`
  are ported directly onto the new pure `resolveClientConfig` function and
  the `customHeadersBuildMiddleware` smithy middleware, rather than onto a
  mocked `BedrockRuntimeClient` constructor/`middlewareStack.add` the way
  upstream's tests do — Go has no equivalent of `vi.mock`, but
  `bedrockruntime.Client.Options()` exposes the same resolved
  Region/BaseEndpoint/BearerAuthTokenProvider/AuthSchemePreference/APIOptions
  fields upstream's tests assert on the mocked constructor's `config`
  argument, without any live AWS credentials or network access.
- **Bedrock explicit-credentials/bearer-token/skip-auth/custom-headers cells
  have no upstream test**: `bedrock-endpoint-resolution.test.ts` only covers
  profile/region/endpoint. `ai/apis/bedrock/clientauth_test.go` and
  `client_test.go` add this port's own coverage for
  `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`AWS_SESSION_TOKEN`,
  `AWS_BEARER_TOKEN_BEDROCK`/`bearerToken`, and `AWS_BEDROCK_SKIP_AUTH`.
- **Bedrock's Node-only HTTP-proxy-agent branch is not ported**: upstream
  swaps in `NodeHttpHandler` with `HttpProxyAgent`/`HttpsProxyAgent` (and
  forces HTTP/1.1 via `AWS_BEDROCK_FORCE_HTTP1`) to work around
  `NodeHttp2Handler` having no HTTP-proxy-agent support. Go's `net/http`
  already honors `HTTP_PROXY`/`HTTPS_PROXY` and negotiates HTTP/1.1 or
  HTTP/2 per connection without a handler swap, so this Node-specific
  workaround has no Go equivalent; no upstream or ported test exercises it.

## Upstream sync procedure

A weekly job (`.github/workflows/upstream-sync.yml`, Monday 06:00 UTC, plus a
manual `workflow_dispatch` trigger) runs `upstream/sync.sh` and opens a
`upstream-sync`-labeled GitHub issue when the diff is non-empty and no such
issue is already open. `upstream/sync_test.sh` (run on every push via
`.github/workflows/test.yml`) offline-tests `sync.sh`'s diff-detection logic,
including the `UPSTREAM_PINNED_SHA` override that lets a deliberately stale
pin rehearse the non-empty-diff path against an already-synced checkout.

On a non-empty diff (from the CI issue or a manual `upstream/sync.sh` run):

1. `upstream/sync.sh` — fetches upstream and diffs `packages/ai` from the
   pinned SHA, bucketing changed files by area.
2. Catalog-only changes (`src/providers/*.models.ts`): re-run
   `tools/export-catalog` and the catalog tests; no Go code changes.
3. For each semantic change: port the change and its new tests into the mapped
   Go package (table above).
4. Update `upstream/UPSTREAM.lock` with the new SHA/version. Go tags mirror
   upstream releases (`v0.80.3-go.N`).
5. `go test ./... -race`, then the env-gated live smokes.
