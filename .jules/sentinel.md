## 2026-07-25 - Prioritize secrets over configuration
**Vulnerability:** The application was loading API keys from the generic `api_key` configuration key before checking the more secure `secrets.api_key` configuration key.
**Learning:** This misconfiguration allowed a local, less secure, or globally scoped `api_key` to inadvertently override specifically injected production secrets managed under the `secrets.*` namespace, weakening defense-in-depth configuration management.
**Prevention:** When managing configuration in Go using `viper`, sensitive credentials (like API keys) should always attempt to be retrieved from the nested `secrets` structure (e.g., `viper.GetString("secrets.api_key")`) before falling back to top-level keys or environment variables.
