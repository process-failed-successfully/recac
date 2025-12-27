package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"recac/internal/docker"
)

var (
	buildContextPath string
	buildDockerfile  string
	buildTag         string
	buildNoCache     bool
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a Docker image",
	Long:  `Build a Docker image from a build context directory and Dockerfile.`,
	Run: func(cmd *cobra.Command, args []string) {
		if buildContextPath == "" {
			fmt.Println("Error: build context path is required (--context)")
			os.Exit(1)
		}
		if buildTag == "" {
			fmt.Println("Error: image tag is required (--tag)")
			os.Exit(1)
		}
		if buildDockerfile == "" {
			buildDockerfile = "Dockerfile"
		}

		// Create tar stream from build context
		tarStream, err := createTarStream(buildContextPath)
		if err != nil {
			fmt.Printf("Error creating tar stream: %v\n", err)
			os.Exit(1)
		}

		// Create Docker client
		client, err := docker.NewClient()
		if err != nil {
			fmt.Printf("Error creating docker client: %v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		// Build image
		opts := docker.ImageBuildOptions{
			BuildContext: tarStream,
			Dockerfile:    buildDockerfile,
			Tag:          buildTag,
			NoCache:      buildNoCache,
		}

		fmt.Printf("Building image %s from %s...\n", buildTag, buildContextPath)
		imageID, err := client.ImageBuild(context.Background(), opts)
		if err != nil {
			fmt.Printf("Error building image: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully built image: %s\n", imageID)
		fmt.Printf("Tag: %s\n", buildTag)
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildContextPath, "context", "", "Path to build context directory")
	buildCmd.Flags().StringVar(&buildDockerfile, "dockerfile", "Dockerfile", "Path to Dockerfile (relative to context)")
	buildCmd.Flags().StringVar(&buildTag, "tag", "", "Image tag (e.g., myimage:latest)")
	buildCmd.Flags().BoolVar(&buildNoCache, "no-cache", false, "Disable build cache")
	rootCmd.AddCommand(buildCmd)
}

// createTarStream creates a tar archive from a directory and returns it as a Reader.
func createTarStream(dirPath string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the directory itself, only include contents
		if path == dirPath {
			return nil
		}

		// Calculate relative path from build context
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && filepath.Base(relPath) == ".git" {
			return filepath.SkipDir
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = linkTarget
		}

		// Use relative path in tar archive
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}
