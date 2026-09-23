package analysis

import (
	"recac/internal/utils"
	"strings"
)

type DockerFinding struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "info", "warning", "error"
	Advice   string `json:"advice"`
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

		if strings.EqualFold(instr.Command, "USER") {
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
	var instructions []dockerInstruction
	var currentLine int = 1
	var inContinuation bool
	var currentCommand string
	var currentArgs string
	var startLine int

	for len(content) > 0 {
		idx := strings.IndexByte(content, '\n')
		var line string
		if idx == -1 {
			line = content
			content = ""
		} else {
			line = content[:idx]
			content = content[idx+1:]
		}

		trimmed := strings.TrimSpace(line)

		if !inContinuation {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				currentLine++
				continue
			}
			startLine = currentLine
		}

		hasContinuation := strings.HasSuffix(trimmed, "\\")
		if hasContinuation {
			trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "\\"))
		}

		if !inContinuation {
			// Start of a new command
			// Use fast field splitting
			spaceIdx := strings.IndexAny(trimmed, " \t")

			if spaceIdx != -1 {
				currentCommand = trimmed[:spaceIdx]
				currentArgs = strings.TrimSpace(trimmed[spaceIdx+1:])
			} else {
				currentCommand = trimmed
				currentArgs = ""
			}
		} else {
			// Continuation of previous command
			if len(currentArgs) > 0 {
				currentArgs += " "
			}
			currentArgs += trimmed
		}

		if hasContinuation {
			inContinuation = true
		} else {
			inContinuation = false
			if currentCommand != "" {
				instructions = append(instructions, dockerInstruction{
					Command: currentCommand,
					Args:    currentArgs,
					Line:    startLine,
				})
			}
			currentCommand = ""
			currentArgs = ""
		}

		currentLine++
	}

	return instructions
}

func checkLatestTag(instr dockerInstruction, findings *[]DockerFinding) {
	if strings.EqualFold(instr.Command, "FROM") {
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
	if strings.EqualFold(instr.Command, "RUN") {
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
	if strings.EqualFold(instr.Command, "RUN") {
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

func checkSecretsEnv(instr dockerInstruction, findings *[]DockerFinding) {
	if strings.EqualFold(instr.Command, "ENV") {
		// ENV MY_PASSWORD=...
		// ⚡ Bolt: Replaced regex with zero-allocation utils.ContainsFold for massive performance improvement
		if utils.ContainsFold(instr.Args, "PASSWORD") ||
			utils.ContainsFold(instr.Args, "SECRET") ||
			utils.ContainsFold(instr.Args, "KEY") ||
			utils.ContainsFold(instr.Args, "TOKEN") {
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
	if strings.EqualFold(prev.Command, "RUN") && strings.EqualFold(curr.Command, "RUN") {
		*findings = append(*findings, DockerFinding{
			Line:     curr.Line,
			Rule:     "combine_run",
			Message:  "Consecutive RUN instructions detected",
			Severity: "info",
			Advice:   "Combine with '&& \\' to reduce image layers and size.",
		})
	}
}
