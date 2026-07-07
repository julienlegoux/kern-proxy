# kern-proxy

A Go port of [`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai):
a unified LLM API with provider collections, automatic auth resolution, token
and cost tracking, and simple context persistence and hand-off to other models
mid-session.

The port targets full functional parity with the upstream TypeScript package
and tracks it over time — see `PORTING.md` for the file mapping and
`upstream/UPSTREAM.lock` for the pinned upstream revision.

## Layout

- `ai/` — domain core: unified message model, streaming event protocol, model
  descriptors, cost/usage accounting, retry/overflow classifiers.
- `ai/internal/` — SSE parsing, partial-JSON streaming, shared HTTP helpers.
- `ai/apis/` — wire-protocol adapters (anthropic-messages, openai-completions,
  openai-responses family, google, mistral, bedrock).
- `ai/providers/` — provider bindings + registry (`createProvider`/`Models`),
  including the in-process `faux` provider for tests.
- `ai/auth/` — credential store, auth resolution, env keys, OAuth flows.
- `ai/catalog/` — embedded model catalog exported from upstream.
- `cmd/pi-ai/` — OAuth login CLI.

## Development

```sh
go test ./... -race
```

Live provider tests are gated behind environment API keys, mirroring upstream.
