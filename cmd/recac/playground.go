package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"recac/internal/docker"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// PlaygroundDockerClient defines the interface used by playground.
// This matches *docker.Client methods.
type PlaygroundDockerClient interface {
	RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, cmd []string, user string) (string, error)
	WaitContainer(ctx context.Context, containerID string) (int64, error)
	ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	Close() error
}

// Factory variables for mocking
var (
	playgroundDockerFactory = func(project string) (PlaygroundDockerClient, error) {
		return docker.NewClient(project)
	}
	startPlaygroundTUIFunc = func(m tea.Model) error {
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	}
)

var playgroundCmd = &cobra.Command{
	Use:   "playground",
	Short: "Interactive AI Playground (Code Scratchpad)",
	Long: `Starts an interactive TUI playground where you can write code snippets,
execute them in an isolated container, and ask AI for help or fixes.

Supported languages: Go, Python, JavaScript (Node), Shell.`,
	RunE: runPlayground,
}

func init() {
	rootCmd.AddCommand(playgroundCmd)
}

func runPlayground(cmd *cobra.Command, args []string) error {
	// Initialize TUI
	model := initialPlaygroundModel()
	return startPlaygroundTUIFunc(model)
}

// --- TUI Implementation ---

type playgroundMode int

const (
	modeCode playgroundMode = iota
	modeAI
)

type playgroundLanguage string

const (
	langGo     playgroundLanguage = "go"
	langPython playgroundLanguage = "python"
	langJS     playgroundLanguage = "javascript"
	langShell  playgroundLanguage = "shell"
)

type playgroundModel struct {
	mode            playgroundMode
	language        playgroundLanguage
	codeArea        textarea.Model
	aiInput         textarea.Model
	outputView      viewport.Model
	chatView        viewport.Model
	outputContent   string
	chatHistory     string
	running         bool
	thinking        bool
	err             error
	codeAreaContent string // to track content
	width           int
	height          int
}

func initialPlaygroundModel() playgroundModel {
	ta := textarea.New()
	ta.Placeholder = "Write your code here..."
	ta.Focus()
	ta.SetHeight(20)
	ta.ShowLineNumbers = true

	aiTa := textarea.New()
	aiTa.Placeholder = "Ask AI to explain or fix code..."
	aiTa.SetHeight(3)
	aiTa.ShowLineNumbers = false

	vp := viewport.New(80, 10)
	chatVp := viewport.New(80, 10)

	initialLang := langGo
	initialCode := getInitialCode(initialLang)

	ta.SetValue(initialCode)

	return playgroundModel{
		mode:            modeCode,
		language:        initialLang,
		codeArea:        ta,
		aiInput:         aiTa,
		outputView:      vp,
		chatView:        chatVp,
		codeAreaContent: initialCode,
	}
}

var codeTemplates = map[playgroundLanguage]string{
	langGo: `package main

import "fmt"

func main() {
	fmt.Println("Hello from Playground!")
}`,
	langPython: `print("Hello from Playground!")`,
	langJS:     `console.log("Hello from Playground!");`,
	langShell:  `echo "Hello from Playground!"`,
}

func getInitialCode(l playgroundLanguage) string {
	return codeTemplates[l]
}

func (m playgroundModel) Init() tea.Cmd {
	return textarea.Blink
}

type runCodeMsg struct {
	output string
	err    error
}

type aiResponseMsg struct {
	response string
	err      error
}

func (m playgroundModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.mode == modeCode {
				m.mode = modeAI
				m.codeArea.Blur()
				m.aiInput.Focus()
			} else {
				m.mode = modeCode
				m.aiInput.Blur()
				m.codeArea.Focus()
			}
		case "ctrl+r":
			// Run Code
			if !m.running {
				m.running = true
				m.outputContent = "Running...\n"
				m.outputView.SetContent(m.outputContent)
				return m, runCodeCmd(m.language, m.codeArea.Value())
			}
		case "ctrl+l":
			// Switch Language (cycle)
			m.language = nextLanguage(m.language)
			newCode := getInitialCode(m.language)
			m.codeArea.SetValue(newCode)
			m.codeAreaContent = newCode
			m.outputContent = "Language switched to " + string(m.language)
			m.outputView.SetContent(m.outputContent)
		case "enter":
			if m.mode == modeAI && !m.thinking {
				prompt := m.aiInput.Value()
				if strings.TrimSpace(prompt) != "" {
					m.thinking = true
					m.chatHistory += "\nYou: " + prompt + "\n"
					m.chatView.SetContent(m.chatHistory)
					m.aiInput.Reset()
					return m, askAICmd(prompt, m.codeArea.Value(), m.outputContent)
				}
				// Intercept Enter to avoid newline in single-line mode if we wanted, but let's allow multiline AI prompts
				// So we don't return here, we let textarea handle it (newlines)
				// Wait, if I want Shift+Enter for newline and Enter for send, I need to check modifier.
				// Bubbletea KeyMsg doesn't expose modifiers easily in String(), need checking Alt/Ctrl etc.
				// For simplicity, let's assume Enter is newline, Ctrl+Enter or something is send?
				// Or just button? TUI has no buttons.
				// Let's use Tab to focus then Enter?
				// Actually, earlier logic handles "enter" only if focused on AI.
				// If we want "Enter" to send, we must NOT call aiInput.Update(msg).
				// But we returned `m, askAICmd` above, so we are good.
				// We just need to ensure we don't also add newline.
				return m, nil
			}
		}

	case runCodeMsg:
		m.running = false
		if msg.err != nil {
			m.outputContent = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.outputContent = msg.output
		}
		m.outputView.SetContent(m.outputContent)

	case aiResponseMsg:
		m.thinking = false
		if msg.err != nil {
			m.chatHistory += fmt.Sprintf("\nError: %v\n", msg.err)
		} else {
			m.chatHistory += "\nAI: " + msg.response + "\n"
		}
		m.chatView.SetContent(m.chatHistory)
		m.chatView.GotoBottom()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
	}

	// Update components
	if m.mode == modeCode {
		m.codeArea, cmd = m.codeArea.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.aiInput, cmd = m.aiInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.outputView, cmd = m.outputView.Update(msg)
	cmds = append(cmds, cmd)

	m.chatView, cmd = m.chatView.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *playgroundModel) layout() {
	if m.height == 0 || m.width == 0 {
		return
	}

	// Layout:
	// Code: 50%
	// Output: 30%
	// Chat: 20% (including input)

	totalH := float64(m.height)
	codeH := int(totalH * 0.5)
	outH := int(totalH * 0.3)

	// Remaining for chat
	// Input fixed 3 lines
	inputH := 3
	chatH := m.height - codeH - outH - inputH - 6 // margins/borders approx

	if chatH < 1 {
		chatH = 1
	}

	m.codeArea.SetWidth(m.width)
	m.codeArea.SetHeight(codeH)

	m.outputView.Width = m.width
	m.outputView.Height = outH

	m.chatView.Width = m.width
	m.chatView.Height = chatH

	m.aiInput.SetWidth(m.width)
	m.aiInput.SetHeight(inputH)
}

func (m playgroundModel) View() string {
	s := lipgloss.NewStyle().Padding(0)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)

	status := fmt.Sprintf("Language: %s (Ctrl+L) | Run: Ctrl+R | Switch: Tab | Quit: Ctrl+C", strings.ToUpper(string(m.language)))
	if m.running {
		status += " | RUNNING..."
	}
	if m.thinking {
		status += " | AI THINKING..."
	}

	header := headerStyle.Render("PLAYGROUND") + " " + status

	codeView := m.codeArea.View()
	outputView := m.outputView.View()
	chatView := m.chatView.View()
	aiInputView := m.aiInput.View()

	return s.Render(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n> %s",
		header,
		codeView,
		sectionStyle.Render("-- Output --"),
		outputView,
		sectionStyle.Render("-- Chat --"),
		chatView,
		aiInputView))
}

func nextLanguage(curr playgroundLanguage) playgroundLanguage {
	switch curr {
	case langGo:
		return langPython
	case langPython:
		return langJS
	case langJS:
		return langShell
	case langShell:
		return langGo
	default:
		return langGo
	}
}

func runCodeCmd(lang playgroundLanguage, code string) tea.Cmd {
	return func() tea.Msg {
		tmpDir, err := os.MkdirTemp("", "recac-playground-*")
		if err != nil {
			return runCodeMsg{err: err}
		}
		defer os.RemoveAll(tmpDir)

		filename := "main.go"
		runArgs := []string{"go", "run", "main.go"}

		switch lang {
		case langPython:
			filename = "script.py"
			runArgs = []string{"python3", "script.py"}
		case langJS:
			filename = "index.js"
			runArgs = []string{"node", "index.js"}
		case langShell:
			filename = "script.sh"
			runArgs = []string{"/bin/sh", "script.sh"}
		}

		if err := os.WriteFile(tmpDir+"/"+filename, []byte(code), 0644); err != nil {
			return runCodeMsg{err: err}
		}

		client, err := playgroundDockerFactory("playground")
		if err != nil {
			return runCodeMsg{err: err}
		}
		defer client.Close()

		ctx := context.Background()
		imageRef := viper.GetString("config.image")
		if imageRef == "" {
			imageRef = "ghcr.io/process-failed-successfully/recac-agent:latest"
		}

		// Run
		id, err := client.RunContainer(ctx, imageRef, tmpDir, nil, nil, runArgs, "root")
		if err != nil {
			return runCodeMsg{err: err}
		}
		defer client.RemoveContainer(ctx, id, true)

		// Wait
		_, err = client.WaitContainer(ctx, id)
		if err != nil {
			// Even if wait fails, logs might be available
		}

		// Logs
		logs, err := client.ContainerLogs(ctx, id)
		if err != nil {
			return runCodeMsg{err: err}
		}
		defer logs.Close()

		outBytes, err := io.ReadAll(logs)
		if err != nil {
			return runCodeMsg{err: err}
		}

		return runCodeMsg{output: string(outBytes)}
	}
}

func askAICmd(prompt, code, output string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		cwd, _ := os.Getwd()
		provider := viper.GetString("provider")
		model := viper.GetString("model")

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-playground")
		if err != nil {
			return aiResponseMsg{err: err}
		}

		fullPrompt := fmt.Sprintf(`I am working in the playground.
Here is my code:
%s

Here is the output:
%s

My question:
%s`, code, output, prompt)

		resp, err := ag.Send(ctx, fullPrompt)
		return aiResponseMsg{response: resp, err: err}
	}
}
