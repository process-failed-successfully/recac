package analysis

import (
	"testing"
)

func TestAnalyzeDockerfile(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedRules []string
	}{
		{
			name: "Clean Dockerfile",
			content: `
FROM node:18-alpine
WORKDIR /app
COPY . .
RUN npm install
USER node
CMD ["npm", "start"]
`,
			expectedRules: []string{},
		},
		{
			name: "Latest Tag",
			content: `
FROM node:latest
USER node
`,
			expectedRules: []string{"explicit_tag"},
		},
		{
			name: "No Tag",
			content: `
FROM node
USER node
`,
			expectedRules: []string{"explicit_tag"},
		},
		{
			name: "Apt Get Upgrade",
			content: `
FROM ubuntu:22.04
RUN apt-get update && apt-get upgrade -y
USER ubuntu
`,
			expectedRules: []string{"no_upgrade"},
		},
		{
			name: "Consecutive RUNs",
			content: `
FROM alpine
RUN apk add curl
RUN apk add git
USER guest
`,
			expectedRules: []string{"combine_run"},
		},
		{
			name: "CD in RUN",
			content: `
FROM alpine
RUN cd /tmp && echo "test"
USER guest
`,
			expectedRules: []string{"prefer_workdir"},
		},
		{
			name: "Secrets in ENV",
			content: `
FROM alpine
ENV AWS_SECRET_KEY=12345
USER guest
`,
			expectedRules: []string{"secrets_env"},
		},
		{
			name: "No User",
			content: `
FROM alpine
`,
			expectedRules: []string{"user_check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := AnalyzeDockerfile(tt.content)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			foundRules := make(map[string]bool)
			for _, f := range findings {
				foundRules[f.Rule] = true
			}

			for _, rule := range tt.expectedRules {
				if !foundRules[rule] {
					t.Errorf("Expected rule %s to be triggered, but it wasn't", rule)
				}
			}

			if len(tt.expectedRules) == 0 && len(findings) > 0 {
				t.Errorf("Expected no findings, but got %d: %v", len(findings), findings)
			}
		})
	}
}
