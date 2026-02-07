package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Persona represents a virtual participant in the brainstorming session.
type Persona struct {
	Name         string
	Role         string
	SystemPrompt string
}

// BrainstormSession manages the state of the session.
type BrainstormSession struct {
	Topic    string
	History  []string // Stores the conversation history
	Personas []Persona
}

var defaultPersonas = []Persona{
	{
		Name: "Product Manager",
		Role: "Product Strategy & Requirements",
		SystemPrompt: `You are an experienced Product Manager.
Your goal is to ensure the product solves real user problems and has clear value.
Focus on:
- User needs and pain points.
- Feature prioritization (MVP vs nice-to-have).
- Success metrics.
- Market fit.
Ask clarifying questions about the user base and the "why".`,
	},
	{
		Name: "Architect",
		Role: "System Design & Technical Feasibility",
		SystemPrompt: `You are a Senior Software Architect.
Your goal is to design a scalable, maintainable, and robust system.
Focus on:
- System components and interactions.
- Technology choices (databases, languages, frameworks).
- Scalability, performance, and reliability.
- Security implications.
Identify potential technical risks and trade-offs.`,
	},
	{
		Name: "QA Engineer",
		Role: "Quality Assurance & Testing",
		SystemPrompt: `You are a Lead QA Engineer.
Your goal is to ensure the system is testable and bug-free.
Focus on:
- Edge cases and error handling.
- Testability of the design.
- Integration points.
- Performance bottlenecks.
Challenge assumptions and ask "what if" questions.`,
	},
}

var brainstormCmd = &cobra.Command{
	Use:   "brainstorm [topic]",
	Short: "Run a multi-persona brainstorming session",
	Long: `Starts an interactive brainstorming session where multiple AI personas (Product Manager, Architect, QA) discuss the given topic.
The output is saved as a Markdown file.

Example:
  recac brainstorm "A decentralized social media platform for cats"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		topic := strings.Join(args, " ")
		outputFile, _ := cmd.Flags().GetString("out")

		if outputFile == "" {
			// Generate filename from topic
			slug := strings.ToLower(strings.ReplaceAll(topic, " ", "-"))
			if len(slug) > 30 {
				slug = slug[:30]
			}
			outputFile = fmt.Sprintf("brainstorm_%s_%d.md", slug, time.Now().Unix())
		}

		session := &BrainstormSession{
			Topic:    topic,
			Personas: defaultPersonas,
		}

		if err := runBrainstorm(cmd, session, outputFile); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(brainstormCmd)
	brainstormCmd.Flags().StringP("out", "o", "", "Output file path (default: auto-generated based on topic)")
	brainstormCmd.Flags().IntP("rounds", "r", 1, "Number of rounds of discussion")
}

func runBrainstorm(cmd *cobra.Command, session *BrainstormSession, outputFile string) error {
	ctx := cmd.Context()

	// Get AI Provider
	provider := viper.GetString("agent_provider")
	model := viper.GetString("agent_model")
	if provider == "" {
		provider = "openai"
	}
	// Fallback model if not set (though usually handled by config)
	if model == "" {
		model = "gpt-4o"
	}

	// 1. Initialize Agent
	ag, err := agentClientFactory(ctx, provider, model, ".", "recac-brainstorm")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🧠 Starting brainstorming session on: %s\n", session.Topic)
	fmt.Fprintf(cmd.OutOrStdout(), "👥 Participants: %s\n\n", listPersonas(session.Personas))

	rounds, _ := cmd.Flags().GetInt("rounds")

	// Initial Context
	session.History = append(session.History, fmt.Sprintf("Topic: %s", session.Topic))

	// Interaction Loop
	for i := 1; i <= rounds; i++ {
		fmt.Fprintf(cmd.OutOrStdout(), "--- Round %d ---\n", i)
		for _, p := range session.Personas {
			fmt.Fprintf(cmd.OutOrStdout(), "👉 %s is thinking...\n", p.Name)

			// Construct Prompt
			prompt := fmt.Sprintf(`You are %s.
Your Role: %s
%s

The Topic is: %s

Here is the conversation so far:
%s

Please provide your input based on your role. Keep it concise (under 200 words) and constructive.
Respond directly as if you are speaking in the meeting.`,
				p.Name, p.Role, p.SystemPrompt, session.Topic, strings.Join(session.History, "\n\n"))

			// Call Agent
			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Error from %s: %v\n", p.Name, err)
				continue
			}

			// Output
			fmt.Fprintf(cmd.OutOrStdout(), "\n👤 %s:\n%s\n\n", p.Name, resp)

			// Update History
			session.History = append(session.History, fmt.Sprintf("**%s**: %s", p.Name, resp))
		}
	}

	// Summarization
	fmt.Fprintf(cmd.OutOrStdout(), "📝 Generating summary...\n")
	scribePrompt := fmt.Sprintf(`You are the Scribe for a technical brainstorming session.
Your goal is to summarize the discussion into a clear, structured Markdown document.

Topic: %s

Discussion History:
%s

Please produce a document with the following sections:
1. Executive Summary
2. Key Requirements (from Product Manager)
3. Technical Architecture (from Architect)
4. QA Strategy & Risks (from QA Engineer)
5. Next Steps

Do not include the raw conversation transcript.`, session.Topic, strings.Join(session.History, "\n\n"))

	summary, err := ag.Send(ctx, scribePrompt)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Save to file
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputFile, []byte(summary), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Brainstorming complete! Summary saved to: %s\n", outputFile)

	return nil
}

func listPersonas(personas []Persona) string {
	names := make([]string, len(personas))
	for i, p := range personas {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
