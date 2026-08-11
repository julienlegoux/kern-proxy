# Issues — Epic 6: Auth core and env-API-key bindings

* [Extend the auth contract: AuthCheck, AuthType, subscription metadata, and the info auth event](./01-auth-contract-surface.md) - M, open, [#170](https://github.com/kern-ia/kern-link/issues/170)
* [Add CredentialStore.List and CredentialInfo across the in-memory and file stores](./02-credential-store-list.md) - S, open, [#171](https://github.com/kern-ia/kern-link/issues/171)
* [Provider-scope api-key resolution: drop Model from APIKeyResolveInput and rebuild the Cloudflare resolvers](./03-provider-scoped-apikey-resolution.md) - M, open, [#172](https://github.com/kern-ia/kern-link/issues/172)
* [Refresh OAuth credentials five minutes before expiry, with a MinOAuthValidity override and a bounded refresh](./04-oauth-refresh-window.md) - M, open, [#173](https://github.com/kern-ia/kern-link/issues/173)
* [Add Models.CheckAuth, Models.GetAvailable, and Provider.FilterModels](./05-models-availability.md) - M, open, [#174](https://github.com/kern-ia/kern-link/issues/174)
* [Add Models.Login, Models.Logout, and the provider-id GetAuth overload, and drive pi-ai login through them](./06-models-login-logout.md) - M, open, [#175](https://github.com/kern-ia/kern-link/issues/175)
* [Anthropic: resolve ANTHROPIC_AUTH_TOKEN as a bearer header ahead of the API-key envs](./07-anthropic-auth-token.md) - M, open, [#176](https://github.com/kern-ia/kern-link/issues/176)
* [Collapse the anthropic and codex login flows onto upstream's always-racing manual-code prompt](./08-anthropic-codex-login-race.md) - L, open, [#177](https://github.com/kern-ia/kern-link/issues/177)
* [Copilot: policy-state model fallback for individual accounts, and a disposition for the unported availability calls](./09-copilot-model-availability.md) - M, open, [#178](https://github.com/kern-ia/kern-link/issues/178)
* [Bind baseten and the three qwen-token-plan providers](./10-env-api-key-bindings.md) - S, open, [#179](https://github.com/kern-ia/kern-link/issues/179)
* [Point docs/PORTING.md at upstream's src/auth/* paths and disposition the new auth files](./11-porting-paths-and-dispositions.md) - S, open, [#180](https://github.com/kern-ia/kern-link/issues/180)
