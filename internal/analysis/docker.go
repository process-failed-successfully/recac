package analysis

import (
	"regexp"
	"strings"
)

type DockerFinding struct {
	Line      int    `json:"line"`
	Rule      string `json:"rule"`
	Message   string `json:"message"`
	Severity  string `json:"severity"` // "info", "warning", "error"
	Advice    string `json:"advice"`
}

type dockerInstruction struct {
	Command string
	Args    string
	Line    int
}

// AnalyzeDockerfile analyzes a Dockerfile content for optimization and security issues.
func AnalyzeDockerfile(content string) ([]DockerFinding, error) {
	instructions := parseDockerfile(content)
	var findings []DockerFinding

	hasUser := false

	for i, instr := range instructions {
		checkLatestTag(instr, &findings)
		checkAptGetUpgrade(instr, &findings)
		checkWorkDir(instr, &findings)
		checkSecretsEnv(instr, &findings)

		if instr.Command == "USER" {
			hasUser = true
		}

		// Context-aware checks
		if i > 0 {
			checkCombineRun(instructions[i-1], instr, &findings)
		}
	}

	if !hasUser {
		findings = append(findings, DockerFinding{
			Line:     0, // Global check
			Rule:     "user_check",
			Message:  "No USER instruction found",
			Severity: "warning",
			Advice:   "Switch to a non-root user (e.g., USER node) towards the end of the Dockerfile.",
		})
	}

	return findings, nil
}

func parseDockerfile(content string) []dockerInstruction {
	lines := strings.Split(content, "\n")
	var instructions []dockerInstruction

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		currentLine := i + 1
		fullCommand := line

		// Handle line continuations
		for strings.HasSuffix(fullCommand, "\\") && i+1 < len(lines) {
			fullCommand = strings.TrimSuffix(fullCommand, "\\")
			i++
			fullCommand += strings.TrimSpace(lines[i])
		}

		parts := strings.Fields(fullCommand)
		if len(parts) > 0 {
			cmd := strings.ToUpper(parts[0])
			args := ""
			if len(parts) > 1 {
				args = strings.TrimSpace(fullCommand[len(parts[0]):])
			}
			instructions = append(instructions, dockerInstruction{
				Command: cmd,
				Args:    args,
				Line:    currentLine,
			})
		}
	}
	return instructions
}

func checkLatestTag(instr dockerInstruction, findings *[]DockerFinding) {
	if instr.Command == "FROM" {
		if !strings.Contains(instr.Args, ":") || strings.HasSuffix(instr.Args, ":latest") {
			*findings = append(*findings, DockerFinding{
				Line:     instr.Line,
				Rule:     "explicit_tag",
				Message:  "Base image uses 'latest' tag or no tag",
				Severity: "warning",
				Advice:   "Pin to a specific version (e.g., node:18-alpine) for reproducibility.",
			})
		}
	}
}

func checkAptGetUpgrade(instr dockerInstruction, findings *[]DockerFinding) {
	if instr.Command == "RUN" {
		if strings.Contains(instr.Args, "apt-get upgrade") || strings.Contains(instr.Args, "apt upgrade") {
			*findings = append(*findings, DockerFinding{
				Line:     instr.Line,
				Rule:     "no_upgrade",
				Message:  "Avoid 'apt-get upgrade' in Dockerfiles",
				Severity: "error",
				Advice:   "Images should be immutable. Upgrade the base image tag instead.",
			})
		}
	}
}

func checkWorkDir(instr dockerInstruction, findings *[]DockerFinding) {
	if instr.Command == "RUN" {
		trimmed := strings.TrimSpace(instr.Args)
		if strings.HasPrefix(trimmed, "cd ") && (strings.Contains(trimmed, " && ") || strings.Contains(trimmed, ";")) {
			// This is a heuristic. Simple "cd" without chaining is useless anyway in Docker.
			*findings = append(*findings, DockerFinding{
				Line:     instr.Line,
				Rule:     "prefer_workdir",
				Message:  "Using 'cd' inside RUN instructions",
				Severity: "info",
				Advice:   "Use WORKDIR instruction to change directories globally and persist state.",
			})
		}
	}
}

var secretRegex = regexp.MustCompile(`(?i)(PASSWORD|SECRET|KEY|TOKEN)`)

func checkSecretsEnv(instr dockerInstruction, findings *[]DockerFinding) {
	if instr.Command == "ENV" {
		// ENV MY_PASSWORD=...
		if secretRegex.MatchString(instr.Args) {
			*findings = append(*findings, DockerFinding{
				Line:     instr.Line,
				Rule:     "secrets_env",
				Message:  "Possible secret in ENV instruction",
				Severity: "error",
				Advice:   "Do not bake secrets into images. Use build args or mount secrets at runtime.",
			})
		}
	}
}

func checkCombineRun(prev, curr dockerInstruction, findings *[]DockerFinding) {
	if prev.Command == "RUN" && curr.Command == "RUN" {
		*findings = append(*findings, DockerFinding{
			Line:     curr.Line,
			Rule:     "combine_run",
			Message:  "Consecutive RUN instructions detected",
			Severity: "info",
			Advice:   "Combine with '&& \\' to reduce image layers and size.",
		})
	}
}
