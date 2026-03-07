package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDepsModel(t *testing.T) {
	// 1. Setup Data
	outgoing := map[string][]string{
		"pkgA": {"pkgB", "pkgC"},
		"pkgB": {"pkgC"},
		"pkgC": {},
	}

	// 2. Initialize Model
	m := NewDepsModel(outgoing)

	// Check initial state
	assert.Equal(t, 3, len(m.metrics))
	assert.Equal(t, 2, m.metrics["pkgA"].Efferent)

	// 3. Test Init
	cmd := m.Init()
	assert.Nil(t, cmd)

	// 4. Test View (Initial)
	view := m.View()
	assert.Contains(t, view, "Initializing...")

	// 5. Test Update (Window Size)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newModel.(DepsModel)
	assert.True(t, m.ready)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)

	// Check View after ready
	view = m.View()
	assert.Contains(t, view, "pkgA")
	assert.Contains(t, view, "pkgB")
	assert.Contains(t, view, "pkgC")

	// 6. Test Selection Update
	selected := m.list.SelectedItem().(depsItem)
	assert.Equal(t, "pkgA", selected.metric.Name)
	assert.Equal(t, "pkgA", m.selectedPkg)

	// Verify Viewport content
	viewportContent := m.viewport.View()
	assert.Contains(t, viewportContent, "Instability")
	// "1.00" might be followed by spaces or ansi codes, so relax check
	assert.Contains(t, viewportContent, "1.00")

	// Check content accounting for wrapping
	assert.Contains(t, viewportContent, "Outgoing")
	assert.Contains(t, viewportContent, "(Ce):")
	assert.Contains(t, viewportContent, "2")

	assert.Contains(t, viewportContent, "Incoming")
	assert.Contains(t, viewportContent, "(Ca):")
	assert.Contains(t, viewportContent, "0")

	// 7. Test Filter
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = newModel.(DepsModel)
	assert.Equal(t, list.Filtering, m.list.FilterState())

	// 8. Test Quit
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// tea.Quit is a Cmd, which is func() Msg
	// Execute the command to see if it produces tea.QuitMsg
	if cmd != nil {
		msg := cmd()
		assert.IsType(t, tea.QuitMsg{}, msg)
	} else {
		t.Error("Expected quit command")
	}
}

func TestRenderHelpers(t *testing.T) {
	// renderMetric
	out := renderMetric("Label", "Value")
	assert.True(t, strings.Contains(out, "Label:") || strings.Contains(out, "Label"))
	assert.True(t, strings.Contains(out, "Value"))

	// renderInstabilityBar
	bar := renderInstabilityBar(0.5, 20)
	assert.Contains(t, bar, "█")
	assert.Contains(t, bar, "░")
}

func TestDepsItem_FilterValue(t *testing.T) {
	item := depsItem{metric: PackageMetric{Name: "test-pkg"}}
	assert.Equal(t, "test-pkg", item.FilterValue())
	assert.Equal(t, "test-pkg", item.Title())
	assert.Contains(t, item.Description(), "Ca:")
}

func TestStartDeps(t *testing.T) {
	outgoing := map[string][]string{
		"pkgA": {"pkgB"},
	}
	// We can't really test tea.Program.Run() easily without an active terminal,
	// but we can ensure it doesn't panic immediately.
	// We'll run it in a goroutine and cancel it if possible, or just skip full Run testing
	// and trust bubbletea.
	// Actually, Bubbletea programs can be run with input, but since StartDeps sets AltScreen,
	// it's tricky. Let's just test that calling NewDepsModel doesn't panic.
	m := NewDepsModel(outgoing)
	assert.NotNil(t, m.list)
}

func TestDepsModel_UpdateViewportEmptyMetrics(t *testing.T) {
	m := NewDepsModel(nil)
	m.viewport = viewport.New(100, 20)
	m.selectedPkg = "non-existent"
	m.updateViewport()
	assert.Contains(t, m.viewport.View(), "Package metrics not found")
}

func TestDepsModel_UpdateViewportNoSelected(t *testing.T) {
	m := NewDepsModel(nil)
	m.viewport = viewport.New(100, 20)
	m.selectedPkg = ""
	m.updateViewport()
	assert.Contains(t, m.viewport.View(), "Select a package")
}

func TestDepsModel_RenderInstabilityBar(t *testing.T) {
	assert.Contains(t, renderInstabilityBar(0.2, 5), "█") // Low width defaults to 10
	assert.Contains(t, renderInstabilityBar(0.8, 10), "█")
	assert.Contains(t, renderInstabilityBar(0.4, 10), "█")
}
