package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/charmbracelet/lipgloss"
)

type TagInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func listTags(host string) {
	u, err := url.Parse(fmt.Sprintf("%s/tags", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch tags: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var tags []TagInfo
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(tags) == 0 {
		fmt.Fprintln(stdout, "No tags found across any jobs.")
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Job Tags (%d)", len(tags))))
	fmt.Fprintln(stdout, "")

	headerCol1 := headerStyle.Width(30).Render("Tag Name")
	headerCol2 := headerStyle.Width(10).Render("Count")

	// Table Header
	fmt.Fprintf(stdout, "%s %s\n", headerCol1, headerCol2)

	for _, tag := range tags {
		rowCol1 := rowStyle.Width(30).Render(limitString(tag.Name, 28))
		rowCol2 := rowStyle.Width(10).Render(fmt.Sprintf("%d", tag.Count))
		fmt.Fprintf(stdout, "%s %s\n", rowCol1, rowCol2)
	}
}
