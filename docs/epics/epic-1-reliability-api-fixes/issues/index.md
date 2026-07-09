# Issues — Epic 1: Reliability & API fixes

* [Extract a shared HTTP retry helper from the Codex adapter](./01-shared-http-retry-helper.md) - M, done, [#87](https://github.com/julienlegoux/kern-proxy/issues/87)
* [Adopt the retry helper in the anthropic and openaicompletions adapters](./02-retry-anthropic-openaicompletions.md) - M, done, [#88](https://github.com/julienlegoux/kern-proxy/issues/88)
* [Adopt the retry helper in the openairesponses and azure adapters](./03-retry-openairesponses-azure.md) - M, open, [#89](https://github.com/julienlegoux/kern-proxy/issues/89)
* [Adopt the retry helper in the google, vertex, and mistral adapters](./04-retry-google-vertex-mistral.md) - M, open, [#90](https://github.com/julienlegoux/kern-proxy/issues/90)
* [Map MaxRetries/MaxRetryDelay onto the bedrock AWS SDK retryer](./05-bedrock-sdk-retries.md) - S, open, [#91](https://github.com/julienlegoux/kern-proxy/issues/91)
* [Fix the Stream.Events() goroutine leak with a context-taking signature](./06-stream-events-context-cancel.md) - M, open, [#92](https://github.com/julienlegoux/kern-proxy/issues/92)
* [Unify Message implementations on pointer receivers](./07-message-pointer-receivers.md) - S, open, [#93](https://github.com/julienlegoux/kern-proxy/issues/93)
* [Document session serialize/restore round-tripping](./08-session-roundtrip-docs.md) - S, open, [#94](https://github.com/julienlegoux/kern-proxy/issues/94)
