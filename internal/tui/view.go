package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type sidebarPart struct {
	Text       string
	Input      int
	Toggle     bool
	Suggestion int
}

func (m *Model) sidebarParts() []sidebarPart {
	part := func(text string) sidebarPart { return sidebarPart{Text: text, Input: -1, Suggestion: -1} }
	width := m.sidebarWidth() - 2
	title := "POLICY TREE"
	if m.focus == treeFocus {
		title = "POLICY TREE  •"
	}
	parts := []sidebarPart{part(strong.Render(title))}
	parts = append(parts, sidebarPart{Text: accent.Render("/ ") + m.inputs[0].View(), Input: 0, Suggestion: -1})
	toggle := "▸ Filters"
	if m.filterOpen {
		toggle = "▾ Filters"
	}
	if count := m.activeFilters(); count > 0 {
		toggle += fmt.Sprintf(" (%d)", count)
	}
	parts = append(parts, sidebarPart{Text: muted.Render(toggle + "  [/]"), Input: -1, Toggle: true, Suggestion: -1})
	if m.focus == inputFocus && m.bodyHeight() >= 12 {
		parts = append(parts, part(dim.Render("Tab field · Enter apply · Esc back")))
	}
	labels := []string{"Text", "Property", "Group", "Action", "Value"}
	addInput := func(i int) {
		parts = append(parts, sidebarPart{Text: muted.Render(fmt.Sprintf("%-9s ", labels[i])) + m.inputs[i].View(), Input: i, Suggestion: -1})
		if m.focus == inputFocus && m.input == i {
			for index, option := range m.suggestions() {
				value := one(option.Name)
				if option.ID != "" {
					value += " · " + one(option.ID)
				}
				prefix := "  "
				style := muted
				if index == m.suggestion {
					prefix = "› "
					style = accent
				}
				parts = append(parts, sidebarPart{Text: style.Render(fit(prefix+value, width)), Input: -1, Suggestion: index})
			}
		}
	}
	if m.filterOpen {
		if m.bodyHeight() < 18 {
			index := 1
			if m.focus == inputFocus && m.input > 0 {
				index = m.input
			}
			addInput(index)
		} else {
			for i := 1; i < 5; i++ {
				addInput(i)
			}
		}
	} else if m.activeFilters() > 0 {
		for i := 1; i < 5; i++ {
			if m.inputs[i].Value() != "" {
				parts = append(parts, part(accent.Render(labels[i]+": ")+muted.Render(one(m.inputs[i].Value()))))
			}
		}
	}
	status := fmt.Sprintf("All policies · %d rules", m.doc.Rules)
	if m.filtering() {
		status = fmt.Sprintf("%d / %d matches  n/N", len(m.matchIDs), m.doc.Rules)
	}
	if m.searchPending {
		status = "Searching…"
	}
	parts = append(parts, part(muted.Render(status)))
	// Short terminals retain the active field and at least one tree row.
	if len(parts) > m.bodyHeight()-2 {
		limit := max(1, m.bodyHeight()-2)
		parts = parts[:min(limit, len(parts))]
	}
	return parts
}
func (m *Model) sidebarHeader() []string {
	parts := m.sidebarParts()
	lines := make([]string, len(parts))
	for i, part := range parts {
		lines[i] = part.Text
	}
	return lines
}

func (m *Model) sidebarLines() []string {
	width, height := m.sidebarWidth(), m.bodyHeight()
	var lines []string
	for _, line := range m.sidebarHeader() {
		lines = append(lines, " "+fit(line, max(0, width-2))+" ")
	}
	for row := 0; row < m.treeHeight(); row++ {
		at := m.treeOffset + row
		text := ""
		if at < len(m.tree) {
			entry := m.tree[at]
			n := m.doc.Nodes[entry.Key]
			depth := min(entry.Depth, max(0, (width-12)/2))
			arrow := " "
			if len(m.doc.Children[n.Key]) > 0 {
				arrow = "▸"
				if m.expanded[n.Key] {
					arrow = "▾"
				}
			}
			mark := "○"
			style := muted
			if n.Kind == "folder" {
				mark = "▰"
				style = accent
			} else if n.Rule.EffectiveEnabled {
				mark = "●"
				style = green
			}
			if entry.Pin {
				mark = "★"
				style = accent
			}
			suffix := ""
			if m.filtering() && m.matches[n.ID] && n.Rule != nil {
				suffix = " ·"
			}
			text = strings.Repeat("  ", depth) + dim.Render(arrow) + " " + style.Render(mark) + " " + normal.Render(one(n.Name)) + accent.Render(suffix)
			if at == m.cursor {
				if m.focus == treeFocus {
					text = selectedStyle.Render(fit(plain(text), width-2))
				} else {
					text = accent.Render("›") + fit(text, max(0, width-3))
				}
			}
		} else if len(m.tree) == 0 && row == 0 {
			text = muted.Render("No matching policies")
		}
		lines = append(lines, " "+fit(text, max(0, width-2))+" ")
	}
	for len(lines) < height-1 {
		lines = append(lines, strings.Repeat(" ", width))
	}
	footer := "● Local policy definitions"
	if len(m.tree) > 0 {
		footer = fmt.Sprintf("● Local · %d–%d / %d", m.treeOffset+1, min(len(m.tree), m.treeOffset+m.treeHeight()), len(m.tree))
	}
	lines = append(lines, " "+fit(dim.Render(footer), max(0, width-2))+" ")
	return lines[:min(len(lines), height)]
}

func (m *Model) detailToolbar() string {
	mode := "Visual"
	if m.textMode {
		mode = "Text"
	}
	diagnostics := "off"
	if m.diagnostics {
		diagnostics = "on"
	}
	return "Policy details   " + mode + " [v]   Diagnostics " + diagnostics + " [d]"
}
func (m *Model) breadcrumb() string {
	var names []string
	for n := m.doc.Nodes[m.selected]; n != nil; n = m.doc.Nodes[n.Parent] {
		names = append(names, one(n.Name))
		if n.Parent == "" {
			break
		}
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return strings.Join(names, " / ")
}
func (m *Model) detailLines() []string {
	width, height := m.paneWidth(), m.bodyHeight()
	lines := []string{
		" " + fit(accent.Render(m.detailToolbar()), max(0, width-2)) + " ",
		" " + fit(dim.Render(m.breadcrumb()), max(0, width-2)) + " ",
	}
	for i := 0; i < m.detailHeight(); i++ {
		at := m.detailOffset + i
		text := ""
		cursor := "  "
		if at < len(m.detail) {
			text = m.paintSelection(m.detail[at].Text, at, policyText)
		}
		if m.focus == detailFocus && at == m.detailCursor {
			cursor = accent.Render("› ")
		}
		lines = append(lines, " "+cursor+fit(text, m.contentWidth())+" ")
	}
	return lines[:min(len(lines), height)]
}

func (m *Model) View() tea.View {
	width, height := max(1, m.width), max(1, m.height)
	var lines []string
	brand := accent.Bold(true).Render("P · polisee") + muted.Render("   EyeSight Policy Viewer")
	lines = append(lines, balanced(" "+brand, selectedStyle.Render(" [Ctrl+O] Open policy "), width))
	counts := fmt.Sprintf("%d folders · %d policies · %d rules", m.doc.Folders, m.doc.Policies, m.doc.Rules)
	name := m.doc.Name
	if m.loading {
		name = "Loading policy…"
	}
	lines = append(lines, balanced(" "+normal.Render(one(name)), dim.Render(counts)+" ", width))
	lines = append(lines, dim.Render(strings.Repeat("─", width)))
	if width < 30 || height < 10 {
		for len(lines) < height-2 {
			lines = append(lines, fit(muted.Render("Resize to 30×10 or larger"), width))
		}
	} else if m.quitPending {
		lines = append(lines, m.quitLines()...)
	} else if m.overlay != noOverlay {
		for _, line := range m.overlayLines() {
			lines = append(lines, fit(line, width))
		}
	} else if width < 78 {
		var pane []string
		if m.focus == detailFocus {
			pane = m.detailLines()
		} else {
			pane = m.sidebarLines()
		}
		lines = append(lines, pane...)
	} else {
		left, right := m.sidebarLines(), m.detailLines()
		for i := 0; i < m.bodyHeight(); i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			lines = append(lines, fit(l, m.sidebarWidth())+dim.Render("│")+fit(r, m.paneWidth()))
		}
	}
	footer := "/ filters · ? help"
	if width >= 110 {
		footer = "Alt+H tree · Alt+L details · / filters · j/k move · Enter open · Esc back · ? help"
	}
	status := muted.Render(one(m.message))
	if m.failed {
		status = warning.Render(one(m.message))
	}
	quitHint := dim.Render(" Ctrl+Q quit ")
	if m.quitPending {
		footer = "Enter quit · Esc cancel"
		status, quitHint = "", ""
	}
	lines = append(lines, dim.Render(strings.Repeat("─", width)))
	lines = append(lines, balanced(" "+dim.Render(footer), status+" ", max(0, width-ansi.StringWidth(quitHint)))+quitHint)
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = fit(lines[i], width)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	if m.mouseEnabled {
		v.MouseMode = tea.MouseModeCellMotion
	}
	v.BackgroundColor = lipgloss.Color("#191b1e")
	v.ForegroundColor = lipgloss.Color("#c9d0da")
	v.WindowTitle = "polisee · EyeSight Policy Viewer"
	return v
}

func (m *Model) quitLines() []string {
	lines := make([]string, m.bodyHeight())
	content := []string{accent.Bold(true).Render("Quit Polisee?"), "", normal.Render("Enter to quit"), muted.Render("Esc to cancel")}
	top := max(0, (len(lines)-len(content))/2)
	for i, text := range content {
		if top+i >= len(lines) {
			break
		}
		lines[top+i] = strings.Repeat(" ", max(0, (m.width-ansi.StringWidth(text))/2)) + text
	}
	return lines
}

func balanced(left, right string, width int) string {
	right = ansi.Truncate(right, max(0, width/2), "…")
	available := max(0, width-ansi.StringWidth(right))
	return fit(left, available) + right
}
