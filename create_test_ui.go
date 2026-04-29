package main

import (
	"fmt"
	"os"
	"recac/internal/orchestrator"
)

func main() {
	err := os.WriteFile("test_ui.html", []byte(orchestrator.DashboardHTML), 0644)
	if err != nil {
		fmt.Println(err)
	}
}
