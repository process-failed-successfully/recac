package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ModelInfo struct to hold model information.
type ModelInfo struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	DescriptionDetails string `json:"descriptionDetails"`
}

// LoadAllModels loads all model definitions.
func LoadAllModels() map[string][]ModelInfo {
	agentModels := make(map[string][]ModelInfo)

	agentModels["openai"] = []ModelInfo{
		{Name: "GPT-4o", Value: "gpt-4o", DescriptionDetails: "Omni model, high intelligence"},
		{Name: "GPT-4 Turbo", Value: "gpt-4-turbo", DescriptionDetails: "High intelligence"},
		{Name: "GPT-3.5 Turbo", Value: "gpt-3.5-turbo", DescriptionDetails: "Fastest and cheap"},
	}

	if orModels, err := loadModelsFromFile("openrouter-models.json"); err == nil && len(orModels) > 0 {
		agentModels["openrouter"] = orModels
	} else {
		agentModels["openrouter"] = []ModelInfo{
			{Name: "NVIDIA Nemotron 3 Nano (Free)", Value: "nvidia/nemotron-3-nano-30b-a3b:free", DescriptionDetails: "Open source, free tier"},
			{Name: "Anthropic Claude 3.5 Sonnet", Value: "anthropic/claude-3.5-sonnet", DescriptionDetails: "High intelligence"},
			{Name: "Google Gemini Pro 1.5", Value: "google/gemini-pro-1.5", DescriptionDetails: "Long context"},
			{Name: "Meta Llama 3 70B", Value: "meta-llama/llama-3-70b-instruct", DescriptionDetails: "Open source"},
		}
	}

	if geminiModels, err := loadModelsFromFile("gemini-models.json"); err == nil && len(geminiModels) > 0 {
		agentModels["gemini"] = geminiModels
	} else {
		agentModels["gemini"] = []ModelInfo{
			{Name: "Gemini 2.0 Flash (Auto)", Value: "gemini-2.0-flash-auto", DescriptionDetails: "Best for most tasks"},
			{Name: "Gemini 2.0 Pro", Value: "gemini-2.0-pro", DescriptionDetails: "High reasoning capability"},
			{Name: "Gemini 2.0 Flash", Value: "gemini-2.0-flash", DescriptionDetails: "Fastest response time"},
			{Name: "Gemini 2.0 Flash Exp", Value: "gemini-2.0-flash-exp", DescriptionDetails: "Experimental features"},
			{Name: "Gemini 2.5 Flash", Value: "gemini-2.5-flash", DescriptionDetails: "Mid-size multimodal model"},
			{Name: "Gemini 2.5 Pro", Value: "gemini-2.5-pro", DescriptionDetails: "Stable release (June 2025)"},
			{Name: "Gemini 1.5 Pro", Value: "gemini-1.5-pro", DescriptionDetails: "Legacy stable model"},
		}
	}

	agentModels["ollama"] = []ModelInfo{
		{Name: "Llama 3", Value: "llama3", DescriptionDetails: "Meta's Llama 3"},
		{Name: "Mistral", Value: "mistral", DescriptionDetails: "Mistral AI"},
		{Name: "Gemma 2", Value: "gemma2", DescriptionDetails: "Google's Gemma"},
		{Name: "Codellama", Value: "codellama", DescriptionDetails: "Code specialized"},
	}

	agentModels["anthropic"] = []ModelInfo{
		{Name: "Claude 3.5 Sonnet", Value: "claude-3-5-sonnet-20240620", DescriptionDetails: "Balanced"},
		{Name: "Claude 3 Opus", Value: "claude-3-opus-20240229", DescriptionDetails: "Most powerful"},
		{Name: "Claude 3 Haiku", Value: "claude-3-haiku-20240307", DescriptionDetails: "Fastest"},
	}

	agentModels["cursor-cli"] = []ModelInfo{
		{Name: "Auto", Value: "auto", DescriptionDetails: "Cursor Default"},
		{Name: "Claude 3.5 Sonnet", Value: "claude-3.5-sonnet", DescriptionDetails: "Specific Model"},
		{Name: "GPT-4o", Value: "gpt-4o", DescriptionDetails: "OpenAI via Cursor"},
	}

	agentModels["gemini-cli"] = []ModelInfo{
		{Name: "Auto", Value: "auto", DescriptionDetails: "Gemini CLI Auto Selection"},
		{Name: "Pro", Value: "pro", DescriptionDetails: "Gemini 1.5 Pro"},
	}
	return agentModels
}

// loadModelsFromFile loads model definitions from a JSON file.
func loadModelsFromFile(filename string) ([]ModelInfo, error) {
	paths := []string{
		filepath.Join("internal", "data", filename),
		filename,
	}

	var data []byte
	var err error

	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	var root struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		} `json:"models"`
	}

	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var items []ModelInfo
	for _, m := range root.Models {
		name := m.DisplayName
		if name == "" {
			name = m.Name
		}
		value := m.Name
		desc := m.Description
		if desc == "" {
			desc = name
		}

		items = append(items, ModelInfo{
			Name:               name,
			Value:              value,
			DescriptionDetails: desc,
		})
	}
	return items, nil
}
