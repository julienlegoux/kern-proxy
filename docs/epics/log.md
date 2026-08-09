# Log

## 2026-08-09

* **Creation**: Established [Epic 1: Repository hygiene](/epic-1-repository-hygiene/EPIC_1.md).
* **Creation**: Established [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md).
* **Creation**: Established [Epic 3: Catalog schema and export tooling](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md).
* **Creation**: Established [Epic 4: OpenAI-family adapters](/epic-4-openai-family-adapters/EPIC_4.md).
* **Creation**: Established [Epic 5: Remaining adapters](/epic-5-remaining-adapters/EPIC_5.md).
* **Creation**: Established [Epic 6: Auth core and env-API-key bindings](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md).
* **Creation**: Established [Epic 7: Four new OAuth flows](/epic-7-four-new-oauth-flows/EPIC_7.md).
* **Creation**: Established [Epic 8: pi-messages and radius](/epic-8-pi-messages-and-radius/EPIC_8.md).
* **Creation**: Established [Epic 9: Classifier audit, disposition sweep, and release](/epic-9-classifier-audit-and-release/EPIC_9.md).
* **Creation**: Established [Issue 01: Bump the go directive from 1.25.0 to 1.26](/epic-1-repository-hygiene/issues/01-bump-go-directive-to-1-26.md) (#127).
* **Creation**: Established [Issue 02: Move the module path to github.com/kern-ia/kern-link](/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md) (#128).
* **Creation**: Established [Issue 01: Widen StopReason and ThinkingLevel, and sweep for non-exhaustive switches](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md) (#129).
* **Creation**: Established [Issue 02: Port the 0.84.1 message-model additions](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md) (#130).
* **Creation**: Established [Issue 03: Port tiered model pricing](/epic-2-core-types-and-models-contracts/issues/03-tiered-model-cost.md) (#131).
* **Creation**: Established [Issue 04: Extend Compat with the 0.84.1 flags, BedrockCompat, and SessionAffinityFormat](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md) (#132).
* **Creation**: Established [Issue 05: Introduce ProviderRequestOptions as the shared request base](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md) (#133).
* **Creation**: Established [Issue 06: Update simple-options and lazy to 0.84.1](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md) (#134).
* **Creation**: Established [Issue 07: Port models-store.ts as ai.ModelsStore](/epic-2-core-types-and-models-contracts/issues/07-models-store.md) (#135).
* **Creation**: Established [Issue 08: Port the Models refresh contract](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md) (#136).
* **Creation**: Established [Issue 09: Add ModelsRequestTransforms and case-insensitive header merging](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md) (#137).
* **Creation**: Established [Issue 10: Dispatch deferred responses through ProviderStreams, Provider, and Models](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md) (#138).
* **Creation**: Established [Issue 11: Port utils/deferred-tools.ts as the deferred-tool split](/epic-2-core-types-and-models-contracts/issues/11-deferred-tools-split.md) (#139).
* **Creation**: Established [Issue 12: Prove the pending and deferred stop reasons end to end through the faux provider](/epic-2-core-types-and-models-contracts/issues/12-faux-deferred-responses.md) (#140).
* **Creation**: Established [Issue 01: Spike — price tools/export-catalog against upstream 936aff00](/epic-3-catalog-schema-and-export-tooling/issues/01-export-catalog-spike.md) (#141).
* **Creation**: Established [Issue 02: Rework tools/export-catalog to consume upstream's generated JSON catalog](/epic-3-catalog-schema-and-export-tooling/issues/02-export-catalog-json-input.md) (#142).
* **Creation**: Established [Issue 03: Bring ai/catalog validation up to the 0.84.1 schema](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md) (#143).
* **Creation**: Established [Issue 04: Regenerate the embedded model catalog from upstream 936aff00](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md) (#144).
* **Creation**: Established [Issue 05: Regenerate the image catalog and cover its decode path](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md) (#145).
* **Creation**: Established [Issue 06: Honor MaxRetries and MaxRetryDelay in the OpenRouter images adapter](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md) (#146).
* **Creation**: Established [Issue 01: Port constrained-sampling.ts as a shared grammar package](/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md) (#147).
* **Creation**: Established [Issue 02: Honor Fetch and SamplingParams in the four OpenAI-family adapters](/epic-4-openai-family-adapters/issues/02-fetch-and-sampling-params.md) (#148).
* **Creation**: Established [Issue 03: openai-completions — emit and stream grammar custom tools](/epic-4-openai-family-adapters/issues/03-completions-grammar-custom-tools.md) (#149).
* **Creation**: Established [Issue 04: openai-completions — baseten thinking format and thinking_token_budget](/epic-4-openai-family-adapters/issues/04-completions-thinking-formats.md) (#150).
* **Creation**: Established [Issue 05: openai-completions — Kimi deferred tools, finish-reason inference, and tool call ids](/epic-4-openai-family-adapters/issues/05-completions-deferred-tools-and-finish-reason.md) (#151).
* **Creation**: Established [Issue 06: Shared responses core — grammar custom tools and tool-result output](/epic-4-openai-family-adapters/issues/06-responses-shared-grammar-and-tool-results.md) (#152).
* **Creation**: Established [Issue 07: Shared responses core — tool namespaces and deferred tool loading](/epic-4-openai-family-adapters/issues/07-responses-shared-namespace-and-deferred-tools.md) (#153).
* **Creation**: Established [Issue 08: Shared responses core — custom tool-call streaming, reasoning backfill, stop reasons](/epic-4-openai-family-adapters/issues/08-responses-shared-stream-decode.md) (#154).
* **Creation**: Established [Issue 09: openai-responses — compat resolution, session affinity, and tool wiring](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md) (#155).
* **Creation**: Established [Issue 10: azure-openai-responses — grammar tools and the pending stop reason](/epic-4-openai-family-adapters/issues/10-azure-responses-wiring.md) (#156).
* **Creation**: Established [Issue 11: openai-codex-responses — request body tools, end_turn and stop-reason guards](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md) (#157).
* **Creation**: Established [Issue 12: openai-codex-responses — session ids, UUIDv7, and the continuation retry](/epic-4-openai-family-adapters/issues/12-codex-session-ids-and-continuation-retry.md) (#158).
* **Creation**: Established [Issue 01: Close the Copilot dynamic-headers gap](/epic-5-remaining-adapters/issues/01-copilot-dynamic-headers.md) (#159).
* **Creation**: Established [Issue 02: anthropic — pending and raw stop reasons, prefilled content blocks, and nullable message_delta usage](/epic-5-remaining-adapters/issues/02-anthropic-stream-lifecycle.md) (#160).
* **Creation**: Established [Issue 03: anthropic — strict tool schemas and signature-only thinking blocks](/epic-5-remaining-adapters/issues/03-anthropic-strict-tools-and-signed-thinking.md) (#161).
* **Creation**: Established [Issue 04: anthropic — deferred tools via defer_loading and tool_reference blocks](/epic-5-remaining-adapters/issues/04-anthropic-deferred-tools.md) (#162).
* **Creation**: Established [Issue 05: google-shared — Gemini 3 tool-call ids, signature-bearing empty blocks, and the VALIDATED function-calling mode](/epic-5-remaining-adapters/issues/05-google-shared-converters.md) (#163).
* **Creation**: Established [Issue 06: google + vertex — pending and raw stop reasons, toolConfig wiring, and the max thinking level](/epic-5-remaining-adapters/issues/06-google-and-vertex-stream-and-params.md) (#164).
* **Creation**: Established [Issue 07: mistral — pending and raw stop reasons with provider-stopped error text, and strict tool sampling](/epic-5-remaining-adapters/issues/07-mistral-stop-reasons-and-strict-tools.md) (#165).
* **Creation**: Established [Issue 08: mistral — request-shape and header parity with the SDK-free upstream client](/epic-5-remaining-adapters/issues/08-mistral-wire-and-header-parity.md) (#166).
* **Creation**: Established [Issue 09: bedrock — pending and raw stop reasons, strict tool schemas, and the Claude 5 model matrix](/epic-5-remaining-adapters/issues/09-bedrock-stop-reasons-strict-tools-and-claude-5.md) (#167).
* **Creation**: Established [Issue 10: bedrock — profile precedence over ambient keys, apiKey as a bearer token, and the response-failure diagnostic](/epic-5-remaining-adapters/issues/10-bedrock-credentials-and-diagnostics.md) (#168).
* **Creation**: Established [Issue 11: cloudflare-stream — classify upstream's dispatch-time base-URL resolution and give it a Go home](/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md) (#169).
* **Creation**: Established [Issue 01: Extend the auth contract — AuthCheck, AuthType, subscription metadata, and the info auth event](/epic-6-auth-core-and-env-api-key-bindings/issues/01-auth-contract-surface.md) (#170).
* **Creation**: Established [Issue 02: Add CredentialStore.List and CredentialInfo across the in-memory and file stores](/epic-6-auth-core-and-env-api-key-bindings/issues/02-credential-store-list.md) (#171).
* **Creation**: Established [Issue 03: Provider-scope api-key resolution — drop Model from APIKeyResolveInput and rebuild the Cloudflare resolvers](/epic-6-auth-core-and-env-api-key-bindings/issues/03-provider-scoped-apikey-resolution.md) (#172).
* **Creation**: Established [Issue 04: Refresh OAuth credentials five minutes before expiry, with a MinOAuthValidity override and a bounded refresh](/epic-6-auth-core-and-env-api-key-bindings/issues/04-oauth-refresh-window.md) (#173).
* **Creation**: Established [Issue 05: Add Models.CheckAuth, Models.GetAvailable, and Provider.FilterModels](/epic-6-auth-core-and-env-api-key-bindings/issues/05-models-availability.md) (#174).
* **Creation**: Established [Issue 06: Add Models.Login, Models.Logout, and the provider-id GetAuth overload](/epic-6-auth-core-and-env-api-key-bindings/issues/06-models-login-logout.md) (#175).
* **Creation**: Established [Issue 07: Anthropic — resolve ANTHROPIC_AUTH_TOKEN as a bearer header ahead of the API-key envs](/epic-6-auth-core-and-env-api-key-bindings/issues/07-anthropic-auth-token.md) (#176).
* **Creation**: Established [Issue 08: Collapse the anthropic and codex login flows onto upstream's always-racing manual-code prompt](/epic-6-auth-core-and-env-api-key-bindings/issues/08-anthropic-codex-login-race.md) (#177).
* **Creation**: Established [Issue 09: Copilot — policy-state model fallback for individual accounts, and a disposition for the unported availability calls](/epic-6-auth-core-and-env-api-key-bindings/issues/09-copilot-model-availability.md) (#178).
* **Creation**: Established [Issue 10: Bind baseten and the three qwen-token-plan providers](/epic-6-auth-core-and-env-api-key-bindings/issues/10-env-api-key-bindings.md) (#179).
* **Creation**: Established [Issue 11: Point docs/PORTING.md at upstream's src/auth/* paths and disposition the new auth files](/epic-6-auth-core-and-env-api-key-bindings/issues/11-porting-paths-and-dispositions.md) (#180).
* **Creation**: Established [Issue 01: Port the xAI device-code OAuth flow and bind it to the xai provider](/epic-7-four-new-oauth-flows/issues/01-xai-device-code-oauth.md) (#181).
* **Creation**: Established [Issue 02: Port the Kimi Code device-code OAuth flow with its retrying refresh, and bind it to kimi-coding](/epic-7-four-new-oauth-flows/issues/02-kimi-coding-device-code-oauth.md) (#182).
* **Creation**: Established [Issue 03: Port the OpenRouter PKCE OAuth flow and bind it to both OpenRouter providers](/epic-7-four-new-oauth-flows/issues/03-openrouter-pkce-oauth.md) (#183).
* **Creation**: Established [Issue 04: Port the Radius gateway OAuth flow: discovery, browser PKCE, and device code](/epic-7-four-new-oauth-flows/issues/04-radius-gateway-oauth.md) (#184).
* **Creation**: Established [Issue 05: Offer every registered OAuth provider in pi-ai login, using each flow's login label](/epic-7-four-new-oauth-flows/issues/05-cli-login-new-flows.md) (#185).
* **Creation**: Established [Issue 06: Document the four new flows in docs/auth.md and disposition their files in docs/PORTING.md](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md) (#186).
* **Creation**: Established [Issue 01: Open the pi-messages package with its wire event union and the converter onto ai.Event](/epic-8-pi-messages-and-radius/issues/01-pimessages-wire-and-converter.md) (#187).
* **Creation**: Established [Issue 02: Stream pi-messages over net/http — request, options, error mapping, and the Stream entry points](/epic-8-pi-messages-and-radius/issues/02-pimessages-stream-entry.md) (#188).
* **Creation**: Established [Issue 03: Port radius-config — the gateway catalog types, their sanitizer, and the gateway-config fetch](/epic-8-pi-messages-and-radius/issues/03-radius-gateway-config.md) (#189).
* **Creation**: Established [Issue 04: Bind the radius provider over pi-messages, with its two-phase catalog refresh](/epic-8-pi-messages-and-radius/issues/04-radius-provider-binding.md) (#190).
* **Creation**: Established [Issue 05: Disposition pi-messages and radius in docs/PORTING.md, and document the tenth adapter](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md) (#191).
