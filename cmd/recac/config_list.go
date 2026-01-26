package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ModelItem struct to hold model information, consistent with the TUI.
type ModelItem struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	DisplayName        string `json:"displayName"`
	DescriptionField   string `json:"description"`
	DescriptionDetails string `json:"descriptionDetails"`
}

// listKeys lists all the configuration keys and their values.
func listKeys(cmd *cobra.Command, args []string) error {
	keys := viper.AllKeys()
	sort.Strings(keys)

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "KEY\tVALUE")
	fmt.Fprintln(w, "---\t-----")

	for _, key := range keys {
		value := viper.Get(key)
		if isSensitive(key) {
			value = "[REDACTED]"
		}
		fmt.Fprintf(w, "%s\t%v\n", key, value)
	}
	return nil
}

// listModels lists all available models, grouped by provider.
func listModels(cmd *cobra.Command, args []string) error {
	agentModels := loadAllModels()
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Sort providers for consistent output
	providers := make([]string, 0, len(agentModels))
	for provider := range agentModels {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	for _, provider := range providers {
		title := provider
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:]
		}
		fmt.Fprintf(w, "Provider: %s\n", title)
		fmt.Fprintln(w, "  NAME\tMODEL ID\tDESCRIPTION")
		fmt.Fprintln(w, "  ----\t--------\t-----------")

		models := agentModels[provider]
		for _, model := range models {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", model.Name, model.Value, model.DescriptionDetails)
		}
		fmt.Fprintln(w) // Add a blank line between providers
	}

	return nil
}

// loadAllModels loads all model definitions, mirroring the TUI's logic.
func loadAllModels() map[string][]ModelItem {
	agentModels := make(map[string][]ModelItem)

	agentModels["openai"] = []ModelItem{
		{Name: "GPT-4o", Value: "gpt-4o", DescriptionDetails: "Omni model, high intelligence"},
		{Name: "GPT-4 Turbo", Value: "gpt-4-turbo", DescriptionDetails: "High intelligence"},
		{Name: "GPT-3.5 Turbo", Value: "gpt-3.5-turbo", DescriptionDetails: "Fastest and cheap"},
	}

	if orModels, err := loadModelsFromFile("openrouter-models.json"); err == nil && len(orModels) > 0 {
		agentModels["openrouter"] = orModels
	} else {
		agentModels["openrouter"] = []ModelItem{
			{Name: "Anthropic Claude 3.5 Sonnet", Value: "anthropic/claude-3.5-sonnet", DescriptionDetails: "High intelligence"},
			{Name: "Google Gemini Pro 1.5", Value: "google/gemini-pro-1.5", DescriptionDetails: "Long context"},
			{Name: "Meta Llama 3 70B", Value: "meta-llama/llama-3-70b-instruct", DescriptionDetails: "Open source"},
		}
	}

	if geminiModels, err := loadModelsFromFile("gemini-models.json"); err == nil && len(geminiModels) > 0 {
		agentModels["gemini"] = geminiModels
	} else {
		agentModels["gemini"] = []ModelItem{
			{Name: "Gemini 2.0 Flash (Auto)", Value: "gemini-2.0-flash-auto", DescriptionDetails: "Best for most tasks"},
			{Name: "Gemini 2.0 Pro", Value: "gemini-2.0-pro", DescriptionDetails: "High reasoning capability"},
			{Name: "Gemini 2.0 Flash", Value: "gemini-2.0-flash", DescriptionDetails: "Fastest response time"},
			{Name: "Gemini 2.0 Flash Exp", Value: "gemini-2.0-flash-exp", DescriptionDetails: "Experimental features"},
			{Name: "Gemini 2.5 Flash", Value: "gemini-2.5-flash", DescriptionDetails: "Mid-size multimodal model"},
			{Name: "Gemini 2.5 Pro", Value: "gemini-2.5-pro", DescriptionDetails: "Stable release (June 2025)"},
			{Name: "Gemini 1.5 Pro", Value: "gemini-1.5-pro", DescriptionDetails: "Legacy stable model"},
		}
	}

	agentModels["ollama"] = []ModelItem{
		{Name: "Llama 3", Value: "llama3", DescriptionDetails: "Meta's Llama 3"},
		{Name: "Mistral", Value: "mistral", DescriptionDetails: "Mistral AI"},
		{Name: "Gemma 2", Value: "gemma2", DescriptionDetails: "Google's Gemma"},
		{Name: "Codellama", Value: "codellama", DescriptionDetails: "Code specialized"},
	}

	agentModels["anthropic"] = []ModelItem{
		{Name: "Claude 3.5 Sonnet", Value: "claude-3-5-sonnet-20240620", DescriptionDetails: "Balanced"},
		{Name: "Claude 3 Opus", Value: "claude-3-opus-20240229", DescriptionDetails: "Most powerful"},
		{Name: "Claude 3 Haiku", Value: "claude-3-haiku-20240307", DescriptionDetails: "Fastest"},
	}

	agentModels["cursor-cli"] = []ModelItem{
		{Name: "Auto", Value: "auto", DescriptionDetails: "Cursor Default"},
		{Name: "Claude 3.5 Sonnet", Value: "claude-3.5-sonnet", DescriptionDetails: "Specific Model"},
		{Name: "GPT-4o", Value: "gpt-4o", DescriptionDetails: "OpenAI via Cursor"},
	}

	agentModels["gemini-cli"] = []ModelItem{
		{Name: "Auto", Value: "auto", DescriptionDetails: "Gemini CLI Auto Selection"},
		{Name: "Pro", Value: "pro", DescriptionDetails: "Gemini 1.5 Pro"},
	}
	return agentModels
}

// isSensitive checks if a key is sensitive.
func isSensitive(key string) bool {
	lowerKey := strings.ToLower(key)
	return strings.Contains(lowerKey, "key") ||
		strings.Contains(lowerKey, "token") ||
		strings.Contains(lowerKey, "secret")
}

// loadModelsFromFile loads model definitions from a JSON file.
func loadModelsFromFile(filename string) ([]ModelItem, error) {
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

	var items []ModelItem
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

		items = append(items, ModelItem{
			Name:               name,
			Value:              value,
			DescriptionDetails: desc,
		})
	}
	return items, nil
}
