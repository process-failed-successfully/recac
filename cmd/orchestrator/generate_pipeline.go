package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func generatePipeline(host, prompt, outFile, provider, model string) {
	urlStr := fmt.Sprintf("%s/pipeline/generate", host)

	// Add query params if provider/model are set
	u, err := url.Parse(urlStr)
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

	reqBody := struct {
		Prompt string `json:"prompt"`
	}{
		Prompt: prompt,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal generate pipeline request: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to generate pipeline: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		PipelineYAML string `json:"pipeline_yaml"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if outFile == "" || outFile == "-" {
		fmt.Fprintln(stdout, result.PipelineYAML)
	} else {
		file, err := os.Create(outFile)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to create output file %s: %v\n", outFile, err)
			exitFunc(1)
			return
		}
		defer file.Close()

		if _, err := file.WriteString(result.PipelineYAML); err != nil {
			fmt.Fprintf(stdout, "Failed to write to file %s: %v\n", outFile, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Pipeline successfully generated to %s\n", outFile)
	}
}
