package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/glamour"
)

func explainJob(host, jobID string, provider string, model string) {
	fmt.Fprintf(stdout, "Fetching explanation for %s...\n", jobID)

	u, err := url.Parse(fmt.Sprintf("%s/jobs/%s/explain", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if provider != "" {
		q.Set("provider", provider)
	}
	if model != "" {
		q.Set("model", model)
	}
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to get explanation: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Explanation string `json:"explanation"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Render the output nicely
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// Fallback to plain text
		fmt.Fprintln(stdout, result.Explanation)
		return
	}

	out, err := r.Render(result.Explanation)
	if err != nil {
		fmt.Fprintln(stdout, result.Explanation)
	} else {
		fmt.Fprint(stdout, out)
	}
}
