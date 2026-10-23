package tui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	accent        = lipgloss.NewStyle().Foreground(lipgloss.Color("#e6ad69"))
	strong        = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0e3e8")).Bold(true)
	normal        = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d0da"))
	muted         = lipgloss.NewStyle().Foreground(lipgloss.Color("#818b98"))
	dim           = lipgloss.NewStyle().Foreground(lipgloss.Color("#656f7c"))
	green         = lipgloss.NewStyle().Foreground(lipgloss.Color("#8eb897"))
	warning       = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0b57e"))
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("#382d20")).Foreground(lipgloss.Color("#efbd7b"))
	sectionStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#23262b")).Foreground(lipgloss.Color("#cbd0d7")).Bold(true)
)

// Exported text is data. Make terminal controls visible rather than allowing
// policy names/values to emit escape sequences, OSC commands or cursor motion.
func safe(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r == '\n':
			out.WriteRune(r)
		case r == '\t':
			out.WriteString("    ")
		case unicode.IsControl(r):
			fmt.Fprintf(&out, "\\u%04x", r)
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func one(value string) string { return strings.ReplaceAll(safe(value), "\n", " ↵ ") }
func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = ansi.Truncate(value, width, "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}
func plain(value string) string { return ansi.Strip(value) }

func newInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.CharLimit = 4096
	style := textinput.DefaultDarkStyles()
	style.Focused.Text = normal
	style.Blurred.Text = normal
	style.Focused.Placeholder = dim
	style.Blurred.Placeholder = dim
	style.Cursor.Color = lipgloss.Color("#e6ad69")
	input.SetStyles(style)
	return input
}

func shown(value string) string {
	if value == "" {
		return "\"\""
	}
	return value
}
