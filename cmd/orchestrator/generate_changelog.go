package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func generateChangelog(host, outFile, tag, match, provider, model string) {
	urlStr := fmt.Sprintf("%s/changelog/generate", host)
	q := url.Values{}
	if tag != "" {
		q.Set("tag", tag)
	}
	if match != "" {
		q.Set("match", match)
	}
	if provider != "" {
		q.Set("provider", provider)
	}
	if model != "" {
		q.Set("model", model)
	}

	if len(q) > 0 {
		urlStr += "?" + q.Encode()
	}

	resp, err := http.Get(urlStr)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to generate changelog: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	changelogText, ok := result["changelog"]
	if !ok {
		fmt.Fprintf(stdout, "Invalid response from server (missing 'changelog' field)\n")
		exitFunc(1)
		return
	}

	if outFile == "" || outFile == "-" {
		fmt.Fprintln(stdout, changelogText)
	} else {
		if err := os.WriteFile(outFile, []byte(changelogText), 0644); err != nil {
			fmt.Fprintf(stdout, "Failed to write changelog to file %s: %v\n", outFile, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Changelog successfully written to %s\n", outFile)
	}
}
