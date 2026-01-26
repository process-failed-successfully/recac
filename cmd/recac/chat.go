package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Persona defines a role for the AI agent.
type Persona struct {
	Name        string
	Description string
	SystemPrompt string
}

var defaultPersonas = map[string]Persona{
	"default": {
		Name:        "Default",
		Description: "A helpful and versatile software engineer assistant.",
		SystemPrompt: "You are a helpful software engineer assistant. Answer questions concisely and accurately.",
	},
	"security": {
		Name:        "Security Auditor",
		Description: "Focuses on identifying vulnerabilities and security best practices.",
		SystemPrompt: "You are a paranoid Security Auditor. You review every piece of code and idea for potential security vulnerabilities (OWASP Top 10, injection, etc.). You are critical and prioritize safety over convenience.",
	},
	"product": {
		Name:        "Product Manager",
		Description: "Focuses on user value, metrics, and business goals.",
		SystemPrompt: "You are a pragmatic Product Manager. You care about user value, business metrics, and trade-offs. You ask 'Why are we building this?' and 'How does this help the user?'. Avoid technical jargon where possible.",
	},
	"junior": {
		Name:        "Junior Developer",
		Description: "Needs simple explanations and mentorship.",
		SystemPrompt: "You are a Junior Developer who is eager to learn but often confused. You ask for clarification on complex topics and prefer simple, step-by-step explanations. You admit when you don't understand.",
	},
	"skeptic": {
		Name:        "The Skeptic",
		Description: "Challenges assumptions and looks for edge cases.",
		SystemPrompt: "You are a Senior Engineer who has seen it all fail. You are skeptical of new libraries, patterns, and 'happy path' thinking. You always ask 'What if this fails?' and 'Have you considered the edge case X?'.",
	},
	"teacher": {
		Name:        "The Teacher",
		Description: "Uses Socratic method to guide learning.",
		SystemPrompt: "You are an expert Computer Science Teacher. Instead of giving the answer directly, you often ask guiding questions to help the user derive the answer. You focus on first principles and clean code.",
	},
}

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
	chatCmd.Flags().StringVarP(&chatPersona, "persona", "p", "default", "Initial persona (default, security, product, junior, skeptic, teacher)")
}

type ChatSession struct {
	History        string
	CurrentPersona Persona
	ContextFiles   map[string]string // path -> content
}

func runChat(cmd *cobra.Command, args []string) error {
	// Initialize Session
	p, ok := defaultPersonas[chatPersona]
	if !ok {
		// Fallback to default if unknown, but warn
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Persona '%s' not found. Using 'default'.\n", chatPersona)
		p = defaultPersonas["default"]
	}

	session := &ChatSession{
		CurrentPersona: p,
		ContextFiles:   make(map[string]string),
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
			if handleChatCommand(cmd, session, input) {
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

func handleChatCommand(cmd *cobra.Command, session *ChatSession, input string) bool {
	parts := strings.Fields(input)
	command := parts[0]

	switch command {
	case "/help":
		fmt.Fprintln(cmd.OutOrStdout(), "Available commands:")
		fmt.Fprintln(cmd.OutOrStdout(), "  /persona <name>  - Switch persona")
		fmt.Fprintln(cmd.OutOrStdout(), "  /add <file>      - Add file content to context")
		fmt.Fprintln(cmd.OutOrStdout(), "  /context         - List current context files")
		fmt.Fprintln(cmd.OutOrStdout(), "  /clear           - Clear chat history (keeps context files)")
		fmt.Fprintln(cmd.OutOrStdout(), "  /quit, /exit     - End session")
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
			for k := range defaultPersonas {
				fmt.Printf("%s ", k)
			}
			fmt.Println()
			return true
		}
		name := parts[1]
		if p, ok := defaultPersonas[name]; ok {
			session.CurrentPersona = p
			// We might want to clear history or annotate the switch?
			// Let's annotate history so agent knows the role changed, or just rely on system prompt update in next turn.
			// Since we rebuild prompt every time with CurrentPersona.SystemPrompt, the agent will see the new role instructions.
			// But previous history might be confusing if role changes drastically.
			// Let's keep history but add a system note.
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
