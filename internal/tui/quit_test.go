package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestQuitConfirmationPreservesViewsAndEditors(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		setup func(*Model)
	}{
		{"tree", func(m *Model) {}},
		{"details", func(m *Model) { m.focusPane(detailFocus) }},
		{"policy filter", func(m *Model) { m.focusInput(2); m.inputs[2].SetValue("typed group") }},
		{"file path", func(m *Model) { m.openFile(); m.dialog.SetValue("typed path") }},
		{"fields", func(m *Model) { m.openFields() }},
		{"help", func(m *Model) { m.overlay = helpOverlay }},
		{"segments", func(m *Model) { m.openSegments() }},
		{"ranges", func(m *Model) { m.openSegments(); m.openRange() }},
		{"range filter", func(m *Model) {
			m.openSegments()
			m.openRange()
			m.startBrowseFilter()
			m.dialog.SetValue("typed range")
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			m := segmented(t)
			scenario.setup(m)
			before, dialog := m.View().Content, m.dialog.Value()
			if cmd := press(m, 'q', tea.ModCtrl); cmd != nil || !m.quitPending {
				t.Fatal("Ctrl+Q did not ask for confirmation")
			}
			if !strings.Contains(ansi.Strip(m.View().Content), "Quit Polisee?") {
				t.Fatal("quit confirmation is not visible")
			}
			if cmd := press(m, 'q', tea.ModCtrl); cmd != nil {
				t.Fatal("repeating Ctrl+Q bypassed confirmation")
			}
			press(m, 'e')
			press(m, 'j')
			m.Update(tea.PasteMsg{Content: "unexpected input"})
			m.Update(tea.MouseWheelMsg{X: 90, Y: 14, Button: tea.MouseWheelDown})
			press(m, tea.KeyEscape)
			if m.quitPending || m.View().Content != before || m.dialog.Value() != dialog {
				t.Fatal("cancelling quit changed the current view or editor")
			}
			press(m, 'q', tea.ModCtrl)
			cmd := press(m, tea.KeyEnter)
			if cmd == nil {
				t.Fatal("Enter did not confirm quitting")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("confirmation did not quit")
			}
		})
	}
}

func TestQuitHintStaysVisibleWithLongStatusMessages(t *testing.T) {
	for _, width := range []int{30, 40, 80, 120, 180} {
		m := demo(t)
		m.Update(tea.WindowSizeMsg{Width: width, Height: 12})
		m.message = strings.Repeat("Long status message ", 30)
		rows := strings.Split(ansi.Strip(m.View().Content), "\n")
		if !strings.Contains(rows[len(rows)-1], "Ctrl+Q quit") {
			t.Fatalf("%d columns: quit shortcut is missing from the bottom bar", width)
		}
		press(m, 'q', tea.ModCtrl)
		rows = strings.Split(ansi.Strip(m.View().Content), "\n")
		if !strings.Contains(rows[len(rows)-1], "Enter quit") || !strings.Contains(rows[len(rows)-1], "Esc cancel") {
			t.Fatalf("%d columns: quit confirmation controls are missing", width)
		}
		for _, row := range rows {
			if ansi.StringWidth(row) != width {
				t.Fatalf("%d columns: quit confirmation overflows the terminal", width)
			}
		}
	}
}

func TestExpandAllUsesOneToggleInTheFocusedPane(t *testing.T) {
	m := demo(t)
	for _, want := range []bool{true, false} {
		press(m, 'e')
		for key := range m.doc.Nodes {
			if len(m.doc.Children[key]) > 0 && m.expanded[key] != want {
				t.Fatalf("tree branch %s expanded = %t, want %t", key, m.expanded[key], want)
			}
		}
	}
	m = demo(t)
	m.focusPane(detailFocus)
	for _, want := range []bool{true, false} {
		press(m, 'e')
		for key, open := range m.sectionOpen[m.selected] {
			if open != want {
				t.Fatalf("section %s expanded = %t, want %t", key, open, want)
			}
		}
		if m.expanded["rule:child"] {
			t.Fatal("expanding details also expanded the tree")
		}
	}
	if strings.Contains(ansi.Strip(m.View().Content), "e/E") {
		t.Fatal("duplicate e/E header labels remain")
	}
}
