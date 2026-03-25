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

	items, _ := orchestrator.ParsePipelineToWorkItemsWithRunID(yamlData, "", nil, "stable")

	for _, item := range items {
		fmt.Printf("Job ID: %s\n", item.ID)
	}
}
