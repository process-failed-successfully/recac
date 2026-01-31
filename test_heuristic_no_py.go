package main

import (
	"fmt"
	"strings"
)

func main() {
    prompt := "Ticket: ID:[PRIMES] Create Prime Number Script\n\nCreate a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000..."

    // Heuristic: Check for Prime Python Implementation (Agent Loop)
	// Note: Ticket generation prompt ALSO contains "primes.py". We need to distinguish.
    // Ticket Generation prompt has "ID:[PRIMES]"
    // Agent Prompt has the description which ALSO has "ID:[PRIMES]" usually in the title or summary.

    // Let's look at the MockAgent logic again.

    isJiraGen := strings.Contains(prompt, "ID:[PRIMES]")
    isAgentTask := strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "prime numbers")

    fmt.Printf("JiraGen: %v\nAgentTask: %v\n", isJiraGen, isAgentTask)
}
