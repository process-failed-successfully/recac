package main

import (
	"fmt"
	"recac/internal/orchestrator"
)

func main() {
	yamlData := []byte(`
name: Pipeline Stages
stages:
  - build
  - test
  - deploy
jobs:
  build1:
    summary: Build App 1
    stage: build
  build2:
    summary: Build App 2
    stage: build
  test1:
    summary: Test App 1
    stage: test
  deploy1:
    summary: Deploy App
    stage: deploy
`)

	items, err := orchestrator.ParsePipelineToWorkItemsWithRunID(yamlData, "", nil, "stable")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for _, item := range items {
		fmt.Printf("Job %s DependsOn: %v\n", item.ID, item.DependsOn)
	}
}
