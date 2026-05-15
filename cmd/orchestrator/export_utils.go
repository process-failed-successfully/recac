package main

import (
	"fmt"
	"io"
	"os"
)

func getExportWriter(outPath string) (io.Writer, *os.File, error) {
	var out io.Writer
	var f *os.File
	var err error

	if outPath == "-" || outPath == "" {
		out = stdout
	} else {
		f, err = os.Create(outPath)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to create output file: %v", err)
		}
		out = f
	}

	return out, f, nil
}
