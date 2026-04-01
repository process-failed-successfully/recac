package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func uploadArtifact(host, jobID, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	filename := filepath.Base(filePath)
	urlStr := fmt.Sprintf("%s/jobs/%s/artifacts/%s", host, url.PathEscape(jobID), url.PathEscape(filename))

	req, err := http.NewRequest(http.MethodPut, urlStr, file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to upload artifact: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully uploaded artifact %s for job %s\n", filename, jobID)
}

func downloadArtifact(host, jobID, filename, outPath string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/artifacts/%s", host, url.PathEscape(jobID), url.PathEscape(filename))

	resp, err := http.Get(urlStr)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to download artifact: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	outFilePath := outPath
	if outFilePath == "" {
		outFilePath = filename
	}

	outFile, err := os.Create(outFilePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create output file %s: %v\n", outFilePath, err)
		exitFunc(1)
		return
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		fmt.Fprintf(stdout, "Failed to save artifact: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully downloaded artifact %s to %s\n", filename, outFilePath)
}

func listArtifacts(host, jobID string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/artifacts", host, url.PathEscape(jobID))

	resp, err := http.Get(urlStr)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to list artifacts: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Artifacts []string `json:"artifacts"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(result.Artifacts) == 0 {
		fmt.Fprintf(stdout, "No artifacts found for job %s\n", jobID)
		return
	}

	fmt.Fprintf(stdout, "Artifacts for job %s:\n", jobID)
	for _, a := range result.Artifacts {
		fmt.Fprintf(stdout, "  - %s\n", a)
	}
}

func deleteArtifact(host, jobID, filename string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/artifacts/%s", host, url.PathEscape(jobID), url.PathEscape(filename))

	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to delete artifact: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully deleted artifact %s for job %s\n", filename, jobID)
}
