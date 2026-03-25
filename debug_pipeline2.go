package main

import (
	"fmt"
	"recac/internal/orchestrator"
	"gopkg.in/yaml.v3"
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

	var p orchestrator.Pipeline
	if err := yaml.Unmarshal(yamlData, &p); err != nil {
		fmt.Println("Error unmarshaling YAML:", err)
		return
	}

	fmt.Printf("Pipeline Name: %s\n", p.Name)
	fmt.Printf("Stages: %v\n", p.Stages)
}
