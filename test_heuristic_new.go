package main

import (
	"fmt"
	"strings"
)

func main() {
    jiraGenPrompt := "Ticket: ID:[PRIMES] Create Prime Number Script\n\nCRITICAL INSTRUCTION: You MUST create exactly ONE ticket.\nCreate a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000..."
    agentLoopPrompt := "Ticket: ID:[PRIMES] Create Prime Number Script\n\nTask: Implement the script.\n\nDescription: Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000..."

    check := func(prompt string) {
        if strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "prime numbers") && !strings.Contains(prompt, "create exactly ONE ticket") {
            fmt.Println("MATCH: Agent Loop")
        } else if strings.Contains(prompt, "ID:[PRIMES]") {
            fmt.Println("MATCH: Jira Gen")
        } else {
            fmt.Println("NO MATCH")
        }
    }

    fmt.Print("Checking Jira Gen Prompt: ")
    check(jiraGenPrompt)

    fmt.Print("Checking Agent Loop Prompt: ")
    check(agentLoopPrompt)
}
