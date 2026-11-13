package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-lib"
)

func TestExportHeaderClickAtDifferentWidths(t *testing.T) {
	for _, width := range []int{30, 60, 80, 140} {
		m := demo(t)
		m.width = width
		header := plain(strings.Split(m.View().Content, "\n")[0])
		x := strings.Index(header, "[Ctrl+E]")
		if x < 0 {
			t.Fatalf("export action missing at width %d", width)
		}
		m.click(tea.Mouse{X: x + 2, Y: 0, Button: tea.MouseLeft})
		if m.overlay != exportOverlay {
			t.Fatalf("export click missed at width %d", width)
		}
	}
}

func TestExportSelectedMainPolicy(t *testing.T) {
	m := demo(t)
	selected := m.selected
	press(m, 'e', tea.ModCtrl)
	if m.overlay != exportOverlay || !strings.HasSuffix(m.dialog.Value(), ".xml") {
		t.Fatal("export dialog did not open")
	}
	press(m, tea.KeyEscape)
	if m.overlay != noOverlay || m.selected != selected {
		t.Fatal("cancel changed selection")
	}
	for _, key := range []string{"folder:folder", "rule:child"} {
		m.selectKey(key)
		press(m, 'e', tea.ModCtrl)
		if m.overlay != noOverlay {
			t.Fatal("export opened for folder or subrule")
		}
	}
	m.selectKey(selected)
	press(m, 'e', tea.ModCtrl)
	path := filepath.Join(t.TempDir(), "selected")
	m.dialog.SetValue(path)
	cmd := press(m, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("no save command")
	}
	m.Update(cmd())
	if m.failed || m.selected != selected {
		t.Fatal(m.message)
	}
	catalog, err := polisee.Load(context.Background(), path+".xml")
	if err != nil || len(catalog.Policies) != 1 || len(catalog.Policies[0].Rules) != 3 {
		t.Fatal("incomplete export", err)
	}
	// Replacing an existing export requires a deliberate confirmation.
	press(m, 'e', tea.ModCtrl)
	m.dialog.SetValue(path + ".xml")
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.overlay != overwriteOverlay {
		t.Fatal("existing file not confirmed")
	}
	press(m, tea.KeyEscape)
	if m.overlay != exportOverlay {
		t.Fatal("cancel replacement did not return to save dialog")
	}
	press(m, tea.KeyEnter)
	cmd = press(m, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("confirmed replace did not save")
	}
	m.Update(cmd())
	if m.failed {
		t.Fatal(m.message)
	}
	// The loaded file, including aliases, must remain intact.
	m.StartPath(path + ".xml")
	before, _ := os.ReadFile(path + ".xml")
	press(m, 'e', tea.ModCtrl)
	m.dialog.SetValue(path + ".xml")
	if cmd := press(m, tea.KeyEnter); cmd != nil || !m.failed || m.overlay != exportOverlay {
		t.Fatal("loaded source overwrite allowed")
	}
	after, _ := os.ReadFile(path + ".xml")
	if string(before) != string(after) {
		t.Fatal("loaded source changed")
	}
}
