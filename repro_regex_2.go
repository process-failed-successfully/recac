package main

import (
	"fmt"
	"regexp"
)

func main() {
	// New Regex: Case insensitive, optional language tag
	var bashBlockRegex = regexp.MustCompile("(?si)```(?:bash|sh)?\\s*(.*?)\\s*```")

	prompt := "TEST PROMPT"
	preview := prompt
	responsePrefix := "Mock agent response"

	// Reconstruct the default response string from MockAgent
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\n```",
		responsePrefix, len(prompt), preview)

	fmt.Printf("Response:\n%s\n---\n", response)

	matches := bashBlockRegex.FindAllStringSubmatch(response, -1)
	fmt.Printf("Matches found: %d\n", len(matches))

	for i, match := range matches {
		fmt.Printf("Match %d: %q\n", i, match[1])
	}

    // Test with no tag
    response2 := "```\necho hello\n```"
    matches2 := bashBlockRegex.FindAllStringSubmatch(response2, -1)
    fmt.Printf("Matches2 found: %d\n", len(matches2))
    if len(matches2) > 0 {
         fmt.Printf("Match 0: %q\n", matches2[0][1])
    }
}
