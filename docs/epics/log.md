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
