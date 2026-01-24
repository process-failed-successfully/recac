package main

import (
	"fmt"
	"github.com/kballard/go-shellquote"
)

func main() {
	payload := "'; echo PWNED; '"
	quoted := shellquote.Join(payload)
	fmt.Printf("Original: %s\n", payload)
	fmt.Printf("Quoted:   %s\n", quoted)
}
