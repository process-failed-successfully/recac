package utils

import (
	"os"
	"strings"
)

// GetSafeEnv returns a whitelist of environment variables safe to pass to subprocesses.
func GetSafeEnv() []string {
	var env []string
	allowedExact := []string{"PATH", "HOME", "USER", "LANG", "TERM"}
	allowedPrefixes := []string{"GO", "NODE_", "PYTHON", "DOCKER_", "XDG_", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "GIT_", "SSH_"}

	for _, e := range os.Environ() {
		idx := strings.IndexByte(e, '=')
		if idx != -1 {
			key := e[:idx]
			allow := false
			for _, a := range allowedExact {
				if key == a {
					allow = true
					break
				}
			}
			if !allow {
				for _, p := range allowedPrefixes {
					if strings.HasPrefix(key, p) {
						allow = true
						break
					}
				}
			}
			if allow {
				env = append(env, e)
			}
		}
	}
	return env
}
