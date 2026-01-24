package main

import (
	"fmt"
	"github.com/kballard/go-shellquote"
)

func main() {
	test1 := "foo; bar"
	fmt.Printf("Input: %s\nOutput: %s\n", test1, shellquote.Join(test1))

	test2 := "'; echo PWNED; '"
	fmt.Printf("Input: %s\nOutput: %s\n", test2, shellquote.Join(test2))
}
