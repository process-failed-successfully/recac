package main

import (
	"fmt"
	"strings"
)

func main() {
    prompt := "Ticket: ID:[PRIMES] Create Prime Number Script\n\nCreate a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000..."

    if strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "prime numbers") {
        fmt.Println("MATCH")
    } else {
        fmt.Println("NO MATCH")
    }
}
