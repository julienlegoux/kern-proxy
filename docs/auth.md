---
type: Guide
title: Auth & credentials
description: How kern-link resolves provider credentials — env API keys, the persistent credential store, the pi-ai login CLI, per-provider OAuth flows, and the terms-of-service risk each credential mode carries.
tags: [auth, oauth, credentials, env, terms-of-service]
timestamp: "2026-07-09"
---

# Auth & credentials

kern-link resolves credentials automatically per provider. You never wire an
HTTP header yourself — set an env var, log in once with the CLI, or pass a key
per request, and `ai.ResolveProviderAuth` does the rest.

# Resolution order

For each request (implemented in `ai/resolve.go`):

1. **Explicit per-request key** — `ai.StreamOptions.APIKey` (plus optional
   `StreamOptions.Env` overrides). Bypasses everything below.
2. **Stored credential** — from the `CredentialStore` given to
   `providers.Models` (an OAuth credential, refreshed under lock when expired,
   or a stored API key). A stored credential *owns* the provider — there is no
   silent env fallback behind it.
3. **Ambient sources** — env vars, AWS credential chain, Google ADC — only
   when nothing is stored.

`models.GetAuth(ctx, model)` reports the outcome without sending a request:
`(nil, nil)` means unconfigured, and `AuthResult.Source` is a display label
(the env var name, `"stored credential"`, or `"OAuth"`).

# Env API keys

| Provider id | Env var(s), in precedence order |
|---|---|
| `anthropic` | `ANTHROPIC_OAUTH_TOKEN`, `ANTHROPIC_API_KEY` |
| `openai` | `OPENAI_API_KEY` |
| `google` | `GEMINI_API_KEY` |
| `mistral` | `MISTRAL_API_KEY` |
| `groq` | `GROQ_API_KEY` |
| `xai` | `XAI_API_KEY` |
| `deepseek` | `DEEPSEEK_API_KEY` |
| `cerebras` | `CEREBRAS_API_KEY` |
| `fireworks` | `FIREWORKS_API_KEY` |
| `together` | `TOGETHER_API_KEY` |
| `huggingface` | `HF_TOKEN` |
| `nvidia` | `NVIDIA_API_KEY` |
| `openrouter` | `OPENROUTER_API_KEY` |
| `vercel-ai-gateway` | `AI_GATEWAY_API_KEY` |
| `azure-openai-responses` | `AZURE_OPENAI_API_KEY` (config: `AZURE_OPENAI_BASE_URL` / `AZURE_OPENAI_RESOURCE_NAME` / `AZURE_OPENAI_API_VERSION` / `AZURE_OPENAI_DEPLOYMENT_NAME_MAP`) |
| `github-copilot` | `COPILOT_GITHUB_TOKEN` |
| `ant-ling` | `ANT_LING_API_KEY` |
| `kimi-coding` | `KIMI_API_KEY` |
| `minimax` / `minimax-cn` | `MINIMAX_API_KEY` / `MINIMAX_CN_API_KEY` |
| `moonshotai` / `moonshotai-cn` | `MOONSHOT_API_KEY` |
| `opencode` | `OPENCODE_API_KEY` |
| `xiaomi` | `XIAOMI_API_KEY` |
| `xiaomi-token-plan-cn/-ams/-sgp` | `XIAOMI_TOKEN_PLAN_{CN,AMS,SGP}_API_KEY` |
| `zai` / `zai-coding-cn` | `ZAI_API_KEY` / `ZAI_CODING_CN_API_KEY` |

**Special auth:**

- `amazon-bedrock` — the ambient AWS chain, checked as: stored bearer token →
  `AWS_BEARER_TOKEN_BEDROCK` → `AWS_PROFILE` → `AWS_ACCESS_KEY_ID` +
  `AWS_SECRET_ACCESS_KEY` → container/web-identity credentials. Requests are
  SigV4-signed by `aws-sdk-go-v2`.
- `google-vertex` — stored key → `GOOGLE_CLOUD_API_KEY` → Application Default
  Credentials. ADC needs the credentials file
  (`GOOGLE_APPLICATION_CREDENTIALS` or
  `~/.config/gcloud/application_default_credentials.json`) **plus**
  `GOOGLE_CLOUD_PROJECT` (or `GCLOUD_PROJECT`) **plus**
  `GOOGLE_CLOUD_LOCATION`.
- `cloudflare-workers-ai` / `cloudflare-ai-gateway` — `CLOUDFLARE_API_KEY` +
  `CLOUDFLARE_ACCOUNT_ID` (+ `CLOUDFLARE_GATEWAY_ID` for the gateway, which
  authenticates via the `cf-aig-authorization` header).
- `openai-codex` — OAuth only (no API-key path).

# Credential store

`auth.NewFileCredentialStore(path)` persists credentials as a JSON object
keyed by provider id; `auth.DefaultPath()` is `~/.pi/agent/auth.json`
(shared with the `pi-ai` CLI).

- Cross-process safe: every read/modify/delete takes an OS-level file lock on
  an `auth.json.lock` sidecar (flock on Unix, `LockFileEx` on Windows).
- Written `0600` in a `0700` directory; unknown providers and credential types
  round-trip untouched.
- Without a store, `providers.Models(nil)` uses an in-memory store — env keys
  still work, OAuth logins just aren't persisted.

# OAuth logins — the `pi-ai` CLI

```sh
go run github.com/julienlegoux/kern-link/cmd/pi-ai login            # interactive picker
go run github.com/julienlegoux/kern-link/cmd/pi-ai login anthropic  # direct
go run github.com/julienlegoux/kern-link/cmd/pi-ai list             # providers + models
```

`login` saves to `~/.pi/agent/auth.json`; any program using
`auth.NewFileCredentialStore(auth.DefaultPath())` picks the credential up
automatically, including token refresh. Three providers support OAuth:

| Provider | Flow | What you do |
|---|---|---|
| `anthropic` (Claude Pro/Max) | PKCE, local callback on port 53692 | Open the printed URL, approve; the callback completes automatically (or paste the code manually). |
| `github-copilot` | Device code | Optionally enter a GitHub Enterprise domain, then open the verification URI and type the shown user code. |
| `openai-codex` (ChatGPT Plus/Pro) | Dual: browser PKCE (port 1455) or device code (headless) | Pick a method at the prompt; browser flow mirrors Anthropic's, device flow shows a code for `https://auth.openai.com/codex/device`. |

Access tokens are refreshed automatically on expiry (under the store's lock,
so concurrent processes refresh once). GitHub Copilot additionally derives its
per-credential API base URL from the token's `proxy-ep` (or the enterprise
domain).

There is no `logout` command yet — remove a credential by deleting its entry
from `auth.json` or calling `CredentialStore.Delete`.

# Credential modes and terms-of-service risk

kern-link has two kinds of credential, and they carry different risk. Which
one you are on is decided by how you authenticated, not by which model you
call.

## API keys — no terms-of-service risk

An API key you obtained from the provider's console (`ANTHROPIC_API_KEY`,
`OPENAI_API_KEY`, `GEMINI_API_KEY`, `MISTRAL_API_KEY`, …), an AWS credential
for Bedrock, or Google ADC for Vertex. You are billed per token under a
developer agreement that exists precisely so programs can call the API. This
is the mode to use in anything you ship.

## Subscription OAuth — account-revocation risk if you ship it

Logging in with `pi-ai login` to **Claude Pro/Max** (`anthropic`), **ChatGPT
Plus/Pro** (`openai-codex`), or **GitHub Copilot** (`github-copilot`) does not
give you an API key. It gives you the credential a *first-party client* uses,
and kern-link then presents itself as that client so the request is accepted:

* `anthropic` sends `user-agent: claude-cli/<version>` and `x-app: cli`, plus
  the `oauth-2025-04-20` beta header — the Claude Code CLI's own identity
  (`ai/apis/anthropic/anthropic.go`).
* `github-copilot` sends `Editor-Version: vscode/1.107.0`
  (`ai/auth/oauth/copilot.go`, and the model catalog's own header defaults).
* All three authenticate with the first-party application's OAuth client ID
  (`ai/auth/oauth/anthropic.go`, `codex.go`, `copilot.go`).

This is inherited upstream behavior, ported faithfully from
[`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai),
not something kern-link invented. It is why a Claude Pro subscription can
drive this library at all.

What it means for you:

* **Personal use** — driving your own subscription from your own machine, the
  way the official CLI would. This is what the flows are for.
* **Shipping it in a product** — you are directing your users (or yourself, at
  scale) to impersonate a first-party client against a subscription that is
  not licensed for programmatic access. Providers can and do revoke accounts
  for this. Their consumer terms cover subscription plans; the developer
  agreement that permits programmatic access covers API keys.

Nothing in kern-link stops you from using OAuth credentials in production.
Nothing in the provider's terms stops them from closing the account when you
do. If you are building something you intend to distribute, use API keys.

# Passing a key programmatically

```go
opts := &ai.SimpleStreamOptions{}
opts.APIKey = "sk-..."          // per-request override, top precedence
stream := models.StreamSimple(ctx, model, chat, opts)
```

`StreamOptions.Env` can likewise supply per-request env-style values (useful
for Azure resource names, Cloudflare account ids, …) without touching the
process environment.

See [usage](/usage.md) for the surrounding request flow and
[architecture](/architecture.md) for where auth sits in the stack.
