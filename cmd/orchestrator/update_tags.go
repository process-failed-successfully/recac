package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func updateTags(host, jobID string, tags []string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/tags", host, url.PathEscape(jobID))

	reqBody := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal tags data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
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
		fmt.Fprintf(stdout, "Failed to update tags: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s tags updated to: %s\n", jobID, strings.Join(tags, ", "))
}

func updateBulkTags(host, match, tag string, tags []string) {
	reqBody := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode request: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/tags", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(jsonData))
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
		fmt.Fprintf(stdout, "Failed to update bulk tags: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated tags for %d pending jobs to: %s\n", result.Updated, strings.Join(tags, ", "))
}

func addTags(host, jobID string, tags []string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/tags/add", host, url.PathEscape(jobID))

	reqBody := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal tags data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
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
		fmt.Fprintf(stdout, "Failed to add tags: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s tags added: %s\n", jobID, strings.Join(tags, ", "))
}

func removeTags(host, jobID string, tags []string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/tags/remove", host, url.PathEscape(jobID))

	reqBody := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal tags data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
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
		fmt.Fprintf(stdout, "Failed to remove tags: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s tags removed: %s\n", jobID, strings.Join(tags, ", "))
}

func addBulkTags(host, match, tag string, tags []string) {
	reqBody := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode request: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/tags/add", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(jsonData))
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
		fmt.Fprintf(stdout, "Failed to add bulk tags: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully added tags for %d pending jobs: %s\n", result.Updated, strings.Join(tags, ", "))
}

func removeBulkTags(host, match, tag string, tags []string) {
	reqBody := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode request: %v\n", err)
		exitFunc(1)
		return
	}

	u, err := url.Parse(fmt.Sprintf("%s/jobs/tags/remove", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(jsonData))
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
		fmt.Fprintf(stdout, "Failed to remove bulk tags: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully removed tags for %d pending jobs: %s\n", result.Updated, strings.Join(tags, ", "))
}
