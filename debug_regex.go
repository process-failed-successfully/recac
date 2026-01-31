package main

import (
	"fmt"
	"regexp"
)

var bashBlockRegex = regexp.MustCompile("(?s)```bash\\s*(.*?)\\s*```")

func main() {
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\n```",
		"Mock agent response", 123, "some prompt")

	fmt.Printf("Response:\n%q\n", response)

	matches := bashBlockRegex.FindAllStringSubmatch(response, -1)
	fmt.Printf("Matches: %d\n", len(matches))
	for i, m := range matches {
		fmt.Printf("Match %d: %q\n", i, m[1])
	}
}
