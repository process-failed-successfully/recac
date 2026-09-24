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
	currentLine := 1

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

		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			currentLine++
			continue
		}

		startLine := currentLine
		fullCommand := line
		currentLine++

		// Handle line continuations
		for strings.HasSuffix(fullCommand, "\\") && len(content) > 0 {
			fullCommand = strings.TrimSuffix(fullCommand, "\\")
			idx2 := strings.IndexByte(content, '\n')
			var nextLine string
			if idx2 == -1 {
				nextLine = content
				content = ""
			} else {
				nextLine = content[:idx2]
				content = content[idx2+1:]
			}
			fullCommand += strings.TrimSpace(nextLine)
			currentLine++
		}

		// ⚡ Bolt: Avoid strings.Fields allocation by finding the first space/tab
		spaceIdx := strings.IndexAny(fullCommand, " \t")
		if spaceIdx != -1 {
			cmd := fullCommand[:spaceIdx]
			args := strings.TrimSpace(fullCommand[spaceIdx+1:])
			instructions = append(instructions, dockerInstruction{
				Command: cmd,
				Args:    args,
				Line:    startLine,
			})
		} else {
			instructions = append(instructions, dockerInstruction{
				Command: fullCommand,
				Args:    "",
				Line:    startLine,
			})
		}
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
