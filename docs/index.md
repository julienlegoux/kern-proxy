---
okf_version: "0.1"
---

# kern-proxy documentation

Documentation bundle for kern-proxy, a unified LLM API for Go. Start with the
[usage guide](usage.md) if you're consuming the library, or
[architecture](architecture.md) if you're reading the code.

* [Usage guide](usage.md) - Installing kern-proxy and using the unified API — model registry, streaming, tool calls, thinking levels, cost tracking, and session persistence.
* [Architecture](architecture.md) - Package layering of the kern-proxy library — domain core, wire adapters, provider registry, auth, embedded catalog — and how a streaming request flows through them.
* [Auth & credentials](auth.md) - How kern-proxy resolves provider credentials — env API keys, the persistent credential store, the pi-ai login CLI, per-provider OAuth flows, and the terms-of-service risk each credential mode carries.
* [Porting map](PORTING.md) - Upstream-to-Go file mapping for the pi-ai port, intentional deviations, and the upstream sync procedure.
* [Post-review improvement plan](PLAN.md) - Improvement plan from the external adoption review — honor MaxRetries in all adapters, fix the Stream.Events goroutine leak, API polish, lint CI, credential-risk docs, and a v0.1.0 tag.
