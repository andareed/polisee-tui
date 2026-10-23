package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-tui/internal/policy"
)

func (m *Model) click(mouse tea.Mouse) tea.Cmd {
	if mouse.Button != tea.MouseLeft {
		return nil
	}
	if mouse.Y == 0 && mouse.X >= m.width-18 {
		return m.openFile()
	}
	if m.overlay != noOverlay {
		if m.overlay == segmentsOverlay || m.overlay == rangesOverlay {
			findRow := 5
			if m.browseHeaderHeight() < 6 {
				findRow = 4
			}
			if mouse.Y == findRow {
				return m.startBrowseFilter()
			}
			row := mouse.Y - 3 - m.browseHeaderHeight()
			if row >= 0 && row < m.browseRows() {
				state := m.browseState()
				_, start, end := policy.PageBounds(state.Page, len(state.Matches), m.browseSize())
				index := start + state.Offset + row
				if index < end {
					state.Cursor = index
					if m.overlay == segmentsOverlay {
						m.openRange()
					} else {
						m.browseInspect()
					}
				}
			}
		} else if m.overlay == fileOverlay && mouse.Y >= 10 {
			index := mouse.Y - 10
			if index < len(m.fileOptions) {
				m.suggestion = index
				return m.fileKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
		}
		return nil
	}
	if mouse.Y < 3 || mouse.Y >= m.height-2 {
		return nil
	}
	tree := mouse.X < m.sidebarWidth()
	if m.width < 78 {
		tree = m.focus != detailFocus
	}
	if tree {
		parts := m.sidebarParts()
		at := mouse.Y - 3
		if at < len(parts) {
			part := parts[at]
			if part.Toggle {
				m.filterOpen = !m.filterOpen
				m.clampTree()
				return nil
			}
			if part.Input >= 0 {
				return m.focusInput(part.Input)
			}
			if part.Suggestion >= 0 {
				options := m.suggestions()
				if part.Suggestion < len(options) {
					m.inputs[m.input].SetValue(options[part.Suggestion].Value)
					m.focusPane(treeFocus)
					return m.scheduleSearch()
				}
			}
			return nil
		}
		index := m.treeOffset + at - len(parts)
		if index < len(m.tree) && index >= 0 {
			m.focusPane(treeFocus)
			m.cursor = index
			entry := m.tree[index]
			m.selectKey(entry.Key)
			m.cursor = index
			if mouse.X <= 3+entry.Depth*2 && len(m.doc.Children[entry.Key]) > 0 {
				m.expanded[entry.Key] = !m.expanded[entry.Key]
				m.rebuildTree()
			}
		}
		return nil
	}
	origin := m.sidebarWidth() + 1
	if m.width < 78 {
		origin = 0
	}
	x := mouse.X - origin - 1
	if mouse.Y == 3 {
		toolbar := m.detailToolbar()
		diagnostics := strings.Index(toolbar, "Diagnostics")
		mode := strings.Index(toolbar, "Visual")
		if mode < 0 {
			mode = strings.Index(toolbar, "Text")
		}
		if x >= diagnostics && x < diagnostics+22 {
			m.diagnostics = !m.diagnostics
			m.rebuildDetails()
		} else if x >= mode && x < mode+10 {
			m.textMode = !m.textMode
			m.rebuildDetails()
		}
		return nil
	}
	index := m.detailOffset + mouse.Y - 5
	if index >= 0 && index < len(m.detail) {
		m.focusPane(detailFocus)
		m.detailCursor = index
		line := m.detail[index]
		switch {
		case line.Section != "":
			m.toggleSection(nil)
		case line.RuleKey != "":
			m.reveal(line.RuleKey)
			m.selectKey(line.RuleKey)
		case line.Browse:
			return m.openSegments()
		case line.Fields != nil && x > m.contentWidth()*2/3:
			m.openFields()
		}
	}
	return nil
}

func (m *Model) wheel(mouse tea.Mouse) {
	delta := -3
	if mouse.Button == tea.MouseWheelDown {
		delta = 3
	}
	if m.overlay == segmentsOverlay || m.overlay == rangesOverlay {
		m.browseMove(delta)
		return
	}
	if m.overlay == fieldsOverlay || m.overlay == helpOverlay {
		_, content := m.overlayContent()
		count := len(content)
		m.overlayOffset = max(0, min(m.overlayOffset+delta, max(0, count-(m.bodyHeight()-3))))
		return
	}
	if m.overlay != noOverlay {
		return
	}
	if (m.width >= 78 && mouse.X < m.sidebarWidth()) || (m.width < 78 && m.focus != detailFocus) {
		m.treeOffset = max(0, min(m.treeOffset+delta, max(0, len(m.tree)-m.treeHeight())))
	} else {
		m.detailOffset = max(0, min(m.detailOffset+delta, max(0, len(m.detail)-m.detailHeight())))
		m.detailCursor = max(m.detailOffset, min(m.detailCursor, m.detailOffset+m.detailHeight()-1))
	}
}
