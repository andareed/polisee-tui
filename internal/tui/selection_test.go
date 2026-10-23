package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func textPosition(t *testing.T, m *Model, text string) tea.Mouse {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	origin := 0
	if m.overlay == noOverlay && m.width >= 78 {
		origin = m.sidebarWidth() + 1
	}
	for y := 5; y < len(lines)-2; y++ {
		visible := ansi.Cut(lines[y], origin, m.width)
		if at := strings.Index(visible, text); at >= 0 {
			return tea.Mouse{X: origin + ansi.StringWidth(visible[:at]), Y: y, Button: tea.MouseLeft}
		}
	}
	t.Fatalf("visible text %q not found", text)
	return tea.Mouse{}
}
func mouseDown(m *Model, p tea.Mouse) tea.Cmd { _, cmd := m.Update(tea.MouseClickMsg(p)); return cmd }
func mouseMove(m *Model, p tea.Mouse)         { m.Update(tea.MouseMotionMsg(p)) }
func mouseUp(m *Model, p tea.Mouse) tea.Cmd   { _, cmd := m.Update(tea.MouseReleaseMsg(p)); return cmd }

func TestDragSelectsTextAndCopyPreservesPolicy(t *testing.T) {
	m := demo(t)
	original := m.selected
	start := textPosition(t, m, "Office access")
	mouseDown(m, start)
	end := start
	end.X += 5
	mouseMove(m, end)
	mouseUp(m, end)
	if got := m.selectedText(); got != "Office" {
		t.Fatalf("selected %q, want Office", got)
	}
	if m.selected != original || m.overlay != noOverlay {
		t.Fatal("text drag activated navigation")
	}
	if !strings.Contains(m.View().Content, "48;2;102;80;54") {
		t.Fatal("selection has no visible highlight")
	}
	cmd := press(m, 'c', tea.ModCtrl)
	if cmd == nil || fmt.Sprint(cmd()) != "Office" {
		t.Fatal("Ctrl+C did not emit the selected text to the clipboard")
	}
	if !m.hasTextSelection() {
		t.Fatal("copy unexpectedly cleared the highlight")
	}
	press(m, tea.KeyEscape)
	if m.hasTextSelection() || m.focus != detailFocus {
		t.Fatal("Escape did not clear selection while preserving the pane")
	}
	if cmd := press(m, 'c', tea.ModCtrl); cmd == nil {
		t.Fatal("Ctrl+C without selection should copy the policy ID")
	} else if got := fmt.Sprint(cmd()); got != "office" {
		t.Fatalf("Ctrl+C copied %q instead of the policy ID", got)
	}
}

func TestShortClicksStillToggleAndHeldDragDoesNot(t *testing.T) {
	m := demo(t)
	start := textPosition(t, m, "Scope & Filters")
	mouseDown(m, start)
	sequence := m.mouseSequence
	if !m.sectionOpen[m.selected]["scope"] {
		t.Fatal("heading activated before mouse release")
	}
	mouseUp(m, start)
	if m.sectionOpen[m.selected]["scope"] {
		t.Fatal("normal click did not toggle the heading")
	}
	m.Update(selectionHeld{sequence})
	if m.hasTextSelection() {
		t.Fatal("stale hold timer selected a completed click")
	}

	start = textPosition(t, m, "Scope & Filters")
	mouseDown(m, start)
	m.Update(selectionHeld{m.mouseSequence})
	if !m.hasTextSelection() {
		t.Fatal("hold did not start text selection")
	}
	end := start
	end.X += 4
	mouseMove(m, end)
	mouseUp(m, end)
	if m.sectionOpen[m.selected]["scope"] {
		t.Fatal("held drag toggled the heading")
	}
	if got := m.selectedText(); got != "Scope" {
		t.Fatal(got)
	}
}

func TestSelectionAcrossRowsOmitsFramesAndPadding(t *testing.T) {
	m := demo(t)
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	start := textPosition(t, m, "Groups")
	end := textPosition(t, m, "192.0.2.0/24")
	end.X += len("192.0.2.0/24") - 1
	mouseDown(m, start)
	mouseMove(m, end)
	mouseUp(m, end)
	want := "Groups  Office devices · ID staff\nSubnet  192.0.2.0/24"
	if got := m.selectedText(); got != want {
		t.Fatalf("selected:\n%q\nwant:\n%q", got, want)
	}
	if strings.ContainsAny(m.selectedText(), "│╭╮╰╯") {
		t.Fatal("copied box decoration")
	}
}

func TestSelectionSnapsToWholeUnicodeGraphemes(t *testing.T) {
	m := demo(t)
	m.doc.Nodes[m.selected].Name = "A界e\u0301🙂Z"
	m.rebuildDetails()
	start := textPosition(t, m, "A界e\u0301🙂Z")
	start.X += 2 // second display cell of 界
	mouseDown(m, start)
	end := start
	end.X += 3 // second display cell of 🙂
	mouseMove(m, end)
	mouseUp(m, end)
	if got := m.selectedText(); got != "界e\u0301🙂" {
		t.Fatalf("split a grapheme: %q", got)
	}
	for _, line := range strings.Split(m.View().Content, "\n") {
		if ansi.StringWidth(line) != m.width {
			t.Fatal("highlight changed terminal geometry")
		}
	}
}

func TestReverseDragAndFieldInspectorSelection(t *testing.T) {
	m := demo(t)
	start := textPosition(t, m, "Office access")
	start.X += 5
	end := start
	end.X -= 5
	mouseDown(m, start)
	mouseMove(m, end)
	mouseUp(m, end)
	if got := m.selectedText(); got != "Office" {
		t.Fatalf("reverse selection: %q", got)
	}
	press(m, tea.KeyEscape)
	press(m, tea.KeyEnter)
	if m.overlay != fieldsOverlay {
		t.Fatal("inspector not open")
	}
	start = textPosition(t, m, "Office access")
	end = start
	end.X += 5
	mouseDown(m, start)
	mouseMove(m, end)
	mouseUp(m, end)
	if got := m.selectedText(); got != "Office" {
		t.Fatalf("inspector selection: %q", got)
	}
	cmd := press(m, 'c', tea.ModCtrl)
	if cmd == nil || fmt.Sprint(cmd()) != "Office" {
		t.Fatal("Ctrl+C did not copy selected inspector text")
	}
	press(m, tea.KeyEscape)
	if m.overlay != fieldsOverlay || m.hasTextSelection() {
		t.Fatal("first Escape should clear the inspector selection")
	}
	press(m, tea.KeyEscape)
	if m.overlay != noOverlay {
		t.Fatal("second Escape should close the inspector")
	}
}

func TestResizeAndDocumentReplacementCancelSelection(t *testing.T) {
	m := demo(t)
	start := textPosition(t, m, "Office access")
	mouseDown(m, start)
	sequence := m.mouseSequence
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(selectionHeld{sequence})
	mouseUp(m, start)
	if m.hasTextSelection() || m.selection.Down {
		t.Fatal("resize left a stale gesture")
	}

	start = textPosition(t, m, "Office access")
	mouseDown(m, start)
	sequence = m.mouseSequence
	replacement := demo(t).doc
	m.setDocument(replacement, "")
	m.Update(selectionHeld{sequence})
	mouseUp(m, start)
	if m.hasTextSelection() || m.selection.Down {
		t.Fatal("document replacement left a stale gesture")
	}
}

func TestTreeMouseNavigationStaysImmediate(t *testing.T) {
	m := demo(t)
	frame := strings.Split(ansi.Strip(m.View().Content), "\n")
	found := false
	for y, line := range frame {
		left := ansi.Cut(line, 0, m.sidebarWidth())
		if at := strings.Index(left, "Guest access"); at >= 0 {
			mouseDown(m, tea.Mouse{X: ansi.StringWidth(left[:at]), Y: y, Button: tea.MouseLeft})
			found = true
			break
		}
	}
	if !found || m.selected != "rule:child" {
		t.Fatal("tree click navigation changed")
	}
}
