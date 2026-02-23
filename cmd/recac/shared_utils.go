package main

import (
	"os"
	"os/exec"
)

// execCommand is a package-level variable to allow mocking in tests.
var execCommand = exec.Command

// writeFileFunc is a package-level variable to allow mocking in tests.
var writeFileFunc = os.WriteFile

// mkdirAllFunc is a package-level variable to allow mocking in tests.
var mkdirAllFunc = os.MkdirAll

// readFileFunc is a package-level variable to allow mocking in tests.
var readFileFunc = os.ReadFile
