---
okf_version: "0.1"
---

# kern-link documentation

Documentation bundle for kern-link, a unified LLM API for Go. Start with the
[usage guide](usage.md) if you're consuming the library, or
[architecture](architecture.md) if you're reading the code.

* [Usage guide](usage.md) - Installing kern-link and using the unified API — model registry, streaming, tool calls, thinking levels, cost tracking, and session persistence.
* [Architecture](architecture.md) - Package layering of the kern-link library — domain core, wire adapters, provider registry, auth, embedded catalog — and how a streaming request flows through them.
* [Auth & credentials](auth.md) - How kern-link resolves provider credentials — env API keys, the persistent credential store, the pi-ai login CLI, per-provider OAuth flows, and the terms-of-service risk each credential mode carries.
* [Porting map](PORTING.md) - Upstream-to-Go file mapping for the pi-ai port, intentional deviations, and the upstream sync procedure.

Development planning lives in a separate bundle, [`docs/planning/`](planning/index.md) —
technical specs and code conventions reverse-engineered from the codebase, for
planning changes rather than consuming the library. The epics that plan is split
into live in a third bundle, [`docs/epics/`](epics/index.md).
