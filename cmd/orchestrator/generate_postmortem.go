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

func generatePostmortem(host, outFile, tag, match, provider, model string) {
	u, err := url.Parse(fmt.Sprintf("%s/postmortem/generate", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
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
		fmt.Fprintf(stdout, "Failed to generate postmortem: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	postmortemText, ok := result["postmortem"]
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	if outFile == "-" || outFile == "stdout" {
		fmt.Fprintln(stdout, postmortemText)
	} else {
		err := os.WriteFile(outFile, []byte(postmortemText), 0644)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to write postmortem to file %s: %v\n", outFile, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Postmortem generated successfully and saved to %s\n", outFile)
	}
}
