package main

import (
	"fmt"
	"os"
)

func main() {
	_, err := os.ReadFile("")
	fmt.Println(err)
}
