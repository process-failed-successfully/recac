## 2026-07-25 - Prioritize secrets over configuration
**Vulnerability:** The application was loading API keys from the generic `api_key` configuration key before checking the more secure `secrets.api_key` configuration key.
**Learning:** This misconfiguration allowed a local, less secure, or globally scoped `api_key` to inadvertently override specifically injected production secrets managed under the `secrets.*` namespace, weakening defense-in-depth configuration management.
**Prevention:** When managing configuration in Go using `viper`, sensitive credentials (like API keys) should always attempt to be retrieved from the nested `secrets` structure (e.g., `viper.GetString("secrets.api_key")`) before falling back to top-level keys or environment variables.

## 2026-08-20 - Enforce nested secrets retrieval for webhook/agent configuration
**Vulnerability:** Several webhook integrations and agent configurations directly retrieved secrets using top-level `viper` keys (e.g., `orchestrator.github_webhook_secret`, `agents.qa.api_key`) without first checking the secure `secrets.*` namespace.
**Learning:** Inconsistent application of the `secrets.*` namespace fallback pattern leads to configuration fragility, allowing local, less secure top-level config keys to accidentally override injected production secrets.
**Prevention:** When managing configurations in Go using `viper`, all code paths that retrieve sensitive credentials must strictly check the `secrets.*` namespace (e.g., `secrets.orchestrator.github_webhook_secret`) first, followed by the fallback to the top-level keys or environment variables.
