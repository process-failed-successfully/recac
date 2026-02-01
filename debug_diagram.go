package main

import (
	"fmt"
	"recac/internal/analysis"
)

func main() {
	classes, relationships, err := analysis.AnalyzeStructs("preview_env")
	if err != nil {
		panic(err)
	}
	mermaid := analysis.GenerateMermaidClassDiagram(classes, relationships, nil, true)
	fmt.Println(mermaid)
}
