package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func archiveJob(host, jobID, outPath string) {
	url := fmt.Sprintf("%s/jobs/%s/archive", host, jobID)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to archive job: %s\n", string(body))
		exitFunc(1)
		return
	}

	if outPath == "" {
		outPath = fmt.Sprintf("%s.tar.gz", jobID)
	}

	// Ensure the directory exists
	dir := filepath.Dir(outPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(stdout, "Failed to create directory %s: %v\n", dir, err)
			exitFunc(1)
			return
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create archive file %s: %v\n", outPath, err)
		exitFunc(1)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		fmt.Fprintf(stdout, "Failed to write archive to %s: %v\n", outPath, err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully saved job archive to %s\n", outPath)
}

func archiveBulkJobs(host, tag, match, outPath string) {
	urlStr := fmt.Sprintf("%s/jobs/archive/bulk?", host)
	if tag != "" {
		urlStr += "tag=" + url.QueryEscape(tag)
	} else if match != "" {
		urlStr += "match=" + url.QueryEscape(match)
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
		fmt.Fprintf(stdout, "Failed to bulk archive jobs: %s\n", string(body))
		exitFunc(1)
		return
	}

	if outPath == "" {
		outPath = "bulk_archive.tar.gz"
	}

	// Ensure the directory exists
	dir := filepath.Dir(outPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(stdout, "Failed to create directory %s: %v\n", dir, err)
			exitFunc(1)
			return
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create archive file %s: %v\n", outPath, err)
		exitFunc(1)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		fmt.Fprintf(stdout, "Failed to write archive to %s: %v\n", outPath, err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully saved bulk archive to %s\n", outPath)
}
