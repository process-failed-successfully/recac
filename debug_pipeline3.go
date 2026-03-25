package main

import (
	"fmt"
	"recac/internal/orchestrator"
	"strings"
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

	jobMap := make(map[string]orchestrator.WorkItem)
	for _, item := range items {
		parts := strings.Split(item.ID, "-")
		jobKey := parts[len(parts)-2]
		jobMap[jobKey] = item
	}

	for k, v := range jobMap {
		fmt.Printf("Parsed %s: id=%s depends=%v\n", k, v.ID, v.DependsOn)
	}

	test1 := jobMap["test1"]
	fmt.Printf("test1 depends on: %v\n", test1.DependsOn)
}
