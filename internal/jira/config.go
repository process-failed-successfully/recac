package jira

import (
	"encoding/json"
	"os"
)

// Config represents Jira configuration
type Config struct {
	JiraURL      string `json:"jira_url"`
	JiraUsername string `json:"jira_username"`
	JiraAPIToken string `json:"jira_api_token"`
	ProjectKey   string `json:"project_key"`
	DefaultQuery string `json:"default_query"`
}

func loadConfig() (*Config, error) {
	file, err := os.ReadFile("config/jira_config.json")
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
