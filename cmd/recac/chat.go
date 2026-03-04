package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/utils"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var chatPersona string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat with the AI agent",
	Long: `Start an interactive chat session with the AI agent.
You can choose a specific persona to roleplay different stakeholders.
Type '/help' during the chat for available commands.`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	// We can't easily list dynamic personas in help text here without loading them first.
	// So we keep the default help text simple.
	chatCmd.Flags().StringVarP(&chatPersona, "persona", "p", "default", "Initial persona ID")
}

type ChatSession struct {
	History        string
	CurrentPersona agent.Persona
	ContextFiles   map[string]string // path -> content
	PM             *agent.PersonaManager
}

func runChat(cmd *cobra.Command, args []string) error {
	// Initialize Persona Manager
	pm := agent.NewPersonaManager()
	if err := pm.LoadPersonas(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to load custom personas: %v\n", err)
	}

	// Initialize Session
	p, ok := pm.GetPersona(chatPersona)
	if !ok {
		// Fallback to default if unknown, but warn
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Persona '%s' not found. Using 'default'.\n", chatPersona)
		p, _ = pm.GetPersona("default")
	}

	session := &ChatSession{
		CurrentPersona: p,
		ContextFiles:   make(map[string]string),
		PM:             pm,
	}

	// Print Welcome
	fmt.Fprintln(cmd.OutOrStdout(), "💬 RECAC Chat Session Started")
	fmt.Fprintf(cmd.OutOrStdout(), "👤 Persona: %s - %s\n", p.Name, p.Description)
	fmt.Fprintln(cmd.OutOrStdout(), "Type '/help' for commands, or just start typing.")
	fmt.Fprintln(cmd.OutOrStdout(), "--------------------------------------------------")

	// Initialize Agent
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-chat")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	scanner := bufio.NewScanner(cmd.InOrStdin())
	for {
		fmt.Fprint(cmd.OutOrStdout(), "\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		// Handle Commands
		if strings.HasPrefix(input, "/") {
			if handleChatCommand(cmd, session, input, ag) {
				continue // Command handled, skip sending to agent
			}
			// If handleChatCommand returns false (e.g. for /quit), we might want to break
			if input == "/quit" || input == "/exit" {
				break
			}
		}

		// Construct Prompt
		prompt := buildChatPrompt(session, input)

		// Send to Agent
		fmt.Fprint(cmd.OutOrStdout(), "🤖 ")
		resp, err := ag.SendStream(ctx, prompt, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
		})
		fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			continue
		}

		// Update History
		// We append the interaction to history so the agent remembers context
		session.History += fmt.Sprintf("\nUser: %s\nAgent: %s\n", input, resp)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error reading input: %v\n", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Chat session ended.")
	return nil
}

func handleChatCommand(cmd *cobra.Command, session *ChatSession, input string, ag agent.Agent) bool {
	parts := strings.Fields(input)
	command := parts[0]

	switch command {
	case "/help":
		fmt.Fprintln(cmd.OutOrStdout(), "Available commands:")
		fmt.Fprintln(cmd.OutOrStdout(), "  /persona <name>  - Switch persona")
		fmt.Fprintln(cmd.OutOrStdout(), "  /add <file>      - Add file content to context")
		fmt.Fprintln(cmd.OutOrStdout(), "  /edit <file> <instructions> - Edit a file using AI")
		fmt.Fprintln(cmd.OutOrStdout(), "  /context         - List current context files")
		fmt.Fprintln(cmd.OutOrStdout(), "  /clear           - Clear chat history (keeps context files)")
		fmt.Fprintln(cmd.OutOrStdout(), "  /save <file>     - Save chat session to a file")
		fmt.Fprintln(cmd.OutOrStdout(), "  /load <file>     - Load chat session from a file")
		fmt.Fprintln(cmd.OutOrStdout(), "  /exec <cmd>      - Execute a shell command and add output to context")
		fmt.Fprintln(cmd.OutOrStdout(), "  /quit, /exit     - End session")
		return true

	case "/edit":
		if len(parts) < 3 {
			fmt.Fprintln(cmd.OutOrStdout(), "Usage: /edit <file_path> <instructions>")
			return true
		}
		path := parts[1]
		instructions := strings.Join(parts[2:], " ")

		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to stat file %s: %v\n", path, err)
			return true
		}

		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to read file %s: %v\n", path, err)
			return true
		}

		prompt := fmt.Sprintf(`You are an expert software engineer.
Edit the following file based on these instructions: "%s"

File: %s
Current Content:
%s

Return ONLY the full modified file content wrapped in a Markdown code block. Do not include any explanations.`, instructions, path, string(content))

		fmt.Fprintf(cmd.OutOrStdout(), "🤖 Editing %s...\n", path)

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		resp, err := ag.SendStream(ctx, prompt, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
		})
		fmt.Fprintln(cmd.OutOrStdout(), "") // Newline
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to edit file: %v\n", err)
			return true
		}

		newContent := utils.CleanCodeBlock(resp)
		if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to save edited file %s: %v\n", path, err)
			return true
		}

		session.History += fmt.Sprintf("\n[System: Edited %s with instructions '%s']\n", path, instructions)
		fmt.Fprintf(cmd.OutOrStdout(), "✅ File %s updated.\n", path)
		return true

	case "/quit", "/exit":
		return false // Signal to break loop

	case "/clear":
		session.History = ""
		fmt.Fprintln(cmd.OutOrStdout(), "🧹 History cleared.")
		return true

	case "/persona":
		if len(parts) < 2 {
			fmt.Fprintln(cmd.OutOrStdout(), "Usage: /persona <name>")
			fmt.Print("Available personas: ")
			// Use PM to list
			for _, k := range session.PM.ListSorted() {
				fmt.Printf("%s ", k)
			}
			fmt.Println()
			return true
		}
		name := parts[1]
		if p, ok := session.PM.GetPersona(name); ok {
			session.CurrentPersona = p
			session.History += fmt.Sprintf("\n[System: Persona changed to %s (%s)]\n", p.Name, p.Description)
			fmt.Fprintf(cmd.OutOrStdout(), "🎭 Switched persona to: %s\n", p.Name)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Unknown persona '%s'.\n", name)
		}
		return true

	case "/add":
		if len(parts) < 2 {
			fmt.Fprintln(cmd.OutOrStdout(), "Usage: /add <file_path>")
			return true
		}
		path := parts[1]
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to read file: %v\n", err)
			return true
		}
		session.ContextFiles[path] = string(content)
		fmt.Fprintf(cmd.OutOrStdout(), "➕ Added %s to context (%d bytes).\n", path, len(content))
		return true

	case "/context":
		if len(session.ContextFiles) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No files in context.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Current context files:")
			for path, content := range session.ContextFiles {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d bytes)\n", path, len(content))
			}
		}
		return true

	case "/save":
		if len(parts) < 2 {
			fmt.Fprintln(cmd.OutOrStdout(), "Usage: /save <file_path>")
			return true
		}
		path := parts[1]
		export := map[string]interface{}{
			"history":       session.History,
			"persona":       session.CurrentPersona.Name, // We could save ID if we have it, but Name is fine, wait no, let's look at how get works
			"context_files": session.ContextFiles,
		}

		// Map persona Name back to ID if possible, else save ID directly. Wait, PM maps ID -> Persona.
		// Let's find the ID.
		var personaID string
		for id, p := range session.PM.ListPersonas() {
			if p.Name == session.CurrentPersona.Name {
				personaID = id
				break
			}
		}
		if personaID != "" {
			export["persona"] = personaID
		}

		data, err := json.MarshalIndent(export, "", "  ")
		if err == nil {
			err = os.WriteFile(path, data, 0644)
		}
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to save chat: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "💾 Chat saved to %s\n", path)
		}
		return true

	case "/load":
		if len(parts) < 2 {
			fmt.Fprintln(cmd.OutOrStdout(), "Usage: /load <file_path>")
			return true
		}
		path := parts[1]
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to read file: %v\n", err)
			return true
		}
		var export struct {
			History      string            `json:"history"`
			Persona      string            `json:"persona"`
			ContextFiles map[string]string `json:"context_files"`
		}
		if err := json.Unmarshal(data, &export); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to parse chat file: %v\n", err)
			return true
		}
		session.History = export.History
		if export.ContextFiles != nil {
			session.ContextFiles = export.ContextFiles
		} else {
			session.ContextFiles = make(map[string]string)
		}
		if p, ok := session.PM.GetPersona(export.Persona); ok {
			session.CurrentPersona = p
		}
		fmt.Fprintf(cmd.OutOrStdout(), "📂 Chat loaded from %s\n", path)
		return true

	case "/exec":
		if len(parts) < 2 {
			fmt.Fprintln(cmd.OutOrStdout(), "Usage: /exec <command>")
			return true
		}
		cmdStr := strings.Join(parts[1:], " ")
		fmt.Fprintf(cmd.OutOrStdout(), "⏳ Executing: %s\n", cmdStr)
		out, err := execCommand("sh", "-c", cmdStr).CombinedOutput()
		output := string(out)
		if err != nil {
			output += fmt.Sprintf("\nError: %v", err)
		}
		session.History += fmt.Sprintf("\n[System: Executed command '%s']\n%s\n", cmdStr, output)
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Output added to context (%d bytes).\n", len(output))
		return true

	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Unknown command '%s'. Type /help for assistance.\n", command)
		return true
	}
}

func buildChatPrompt(session *ChatSession, input string) string {
	var sb strings.Builder

	// 1. System Prompt (Persona)
	sb.WriteString(session.CurrentPersona.SystemPrompt)
	sb.WriteString("\n\n")

	// 2. Context Files
	if len(session.ContextFiles) > 0 {
		sb.WriteString("Context Files:\n")
		for path, content := range session.ContextFiles {
			// Truncate if huge? For now assuming user adds reasonable files.
			sb.WriteString(fmt.Sprintf("--- %s ---\n%s\n--- End of %s ---\n\n", path, content, path))
		}
	}

	// 3. History
	sb.WriteString("Chat History:\n")
	sb.WriteString(session.History)
	sb.WriteString("\n")

	// 4. Current Input
	sb.WriteString("User: " + input + "\n")
	sb.WriteString("Agent:") // Prompt for completion

	return sb.String()
}
