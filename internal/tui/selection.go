package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

const selectionHoldDelay = 350 * time.Millisecond
const selectionHint = "Text selected · Ctrl+C copy · Esc clear"

type selectionSurface uint8

const (
	policyText selectionSurface = iota + 1
	exportedText
)

type selectionPoint struct{ Row, Column int }
type selectionRow struct {
	Text        string
	Left, Right int
}
type selectionHeld struct{ sequence uint64 }

type textSelection struct {
	Surface              selectionSurface
	Rows                 []selectionRow
	Anchor, Head         selectionPoint
	Press                tea.Mouse
	Document             *policy.Document
	Node                 string
	OriginX, Top, Height int
	Down, Active         bool
	PreviousMessage      string
}

var textSelectionStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#665036")).
	Foreground(lipgloss.Color("#fff5df"))

func (m *Model) clearTextSelection() {
	if m.selection.Active && (m.message == selectionHint || m.message == "Selected text sent to terminal clipboard") {
		m.message = m.selection.PreviousMessage
	}
	m.mouseSequence++
	m.selection = textSelection{}
}

func (m *Model) selectionSnapshot() (textSelection, bool) {
	if !m.mouseEnabled || m.width < 30 || m.height < 10 {
		return textSelection{}, false
	}
	s := textSelection{Document: m.doc, Node: m.selected, Top: 5, PreviousMessage: m.message}
	switch {
	case m.overlay == noOverlay && (m.width >= 78 || m.focus == detailFocus):
		s.Surface = policyText
		s.OriginX = 3
		if m.width >= 78 {
			s.OriginX += m.sidebarWidth() + 1
		}
		s.Height = m.detailHeight()
		s.Rows = make([]selectionRow, len(m.detail))
		for i, line := range m.detail {
			s.Rows[i] = selectionRow{Text: line.Text, Left: line.TextStart, Right: line.TextEnd}
		}
	case m.overlay == fieldsOverlay:
		s.Surface = exportedText
		s.OriginX = 2
		s.Height = max(1, m.bodyHeight()-3)
		_, content := m.overlayContent()
		s.Rows = make([]selectionRow, len(content))
		for i, line := range content {
			s.Rows[i] = selectionRow{Text: line, Right: ansi.StringWidth(line)}
		}
	default:
		return textSelection{}, false
	}
	return s, true
}

func (m *Model) selectionOffset(s textSelection) int {
	if s.Surface == policyText {
		return m.detailOffset
	}
	return max(0, min(m.overlayOffset, max(0, len(s.Rows)-s.Height)))
}

func (m *Model) selectionPointAt(s textSelection, mouse tea.Mouse, clamp bool) (selectionPoint, bool) {
	if len(s.Rows) == 0 {
		return selectionPoint{}, false
	}
	y := mouse.Y - s.Top
	if !clamp && (y < 0 || y >= s.Height) {
		return selectionPoint{}, false
	}
	y = max(0, min(y, s.Height-1))
	index := max(0, min(m.selectionOffset(s)+y, len(s.Rows)-1))
	row := s.Rows[index]
	column := mouse.X - s.OriginX
	if !clamp && (row.Right <= row.Left || column < row.Left || column >= row.Right) {
		return selectionPoint{}, false
	}
	if row.Right <= row.Left {
		return selectionPoint{index, 0}, true
	}
	column = max(row.Left, min(column, row.Right-1))
	left, _ := glyphBounds(ansi.Strip(row.Text), column)
	return selectionPoint{index, left}, true
}

// A mouse cell may be the second half of a wide character. Snap both selection
// ends to grapheme boundaries, retaining CJK, emoji and combining sequences.
func glyphBounds(text string, column int) (int, int) {
	at := 0
	for text != "" {
		cluster, width := ansi.FirstGraphemeCluster(text, ansi.GraphemeWidth)
		if cluster == "" {
			break
		}
		if column < at+width {
			return at, at + width
		}
		at += width
		text = text[len(cluster):]
	}
	return at, at
}

func (m *Model) mousePress(mouse tea.Mouse) tea.Cmd {
	m.clearTextSelection()
	if mouse.Button != tea.MouseLeft {
		return m.click(mouse)
	}
	s, ok := m.selectionSnapshot()
	if !ok {
		return m.click(mouse)
	}
	point, ok := m.selectionPointAt(s, mouse, false)
	if !ok {
		return m.click(mouse)
	}
	s.Anchor = point
	s.Head = point
	s.Press = mouse
	s.Down = true
	m.selection = s
	if s.Surface == policyText {
		m.focusPane(detailFocus)
		m.detailCursor = point.Row
	}
	sequence := m.mouseSequence
	return tea.Tick(selectionHoldDelay, func(time.Time) tea.Msg { return selectionHeld{sequence} })
}

func (m *Model) mouseMotion(mouse tea.Mouse) {
	if !m.selection.Down || mouse.Button != tea.MouseLeft {
		return
	}
	point, ok := m.selectionPointAt(m.selection, mouse, true)
	if !ok {
		return
	}
	m.selection.Head = point
	if point != m.selection.Anchor {
		m.selection.Active = true
	}
	if m.selection.Active {
		m.message = selectionHint
	}
}

func (m *Model) mouseRelease(mouse tea.Mouse) tea.Cmd {
	if !m.selection.Down || (mouse.Button != tea.MouseLeft && mouse.Button != tea.MouseNone) {
		return nil
	}
	if mouse.X != m.selection.Press.X || mouse.Y != m.selection.Press.Y {
		move := mouse
		move.Button = tea.MouseLeft
		m.mouseMotion(move)
	}
	if m.selection.Active {
		m.selection.Down = false
		m.mouseSequence++
		m.message = selectionHint
		return nil
	}
	press := m.selection.Press
	m.clearTextSelection()
	return m.click(press)
}

func (m *Model) holdSelection(msg selectionHeld) {
	if msg.sequence == m.mouseSequence && m.selection.Down {
		m.selection.Active = true
		m.message = selectionHint
	}
}

func (m *Model) hasTextSelection() bool {
	s := m.selection
	if !s.Active || s.Document != m.doc || s.Node != m.selected {
		return false
	}
	return (s.Surface == policyText && m.overlay == noOverlay) || (s.Surface == exportedText && m.overlay == fieldsOverlay)
}

func pointBefore(a, b selectionPoint) bool {
	return a.Row < b.Row || (a.Row == b.Row && a.Column < b.Column)
}
func (m *Model) selectionRange(index int) (int, int) {
	if !m.hasTextSelection() || index < 0 || index >= len(m.selection.Rows) {
		return 0, 0
	}
	first, last := m.selection.Anchor, m.selection.Head
	if pointBefore(last, first) {
		first, last = last, first
	}
	if index < first.Row || index > last.Row {
		return 0, 0
	}
	row := m.selection.Rows[index]
	left, right := row.Left, row.Right
	if index == first.Row {
		left = max(left, first.Column)
	}
	if index == last.Row {
		_, end := glyphBounds(ansi.Strip(row.Text), last.Column)
		right = min(right, end)
	}
	if right <= left {
		return 0, 0
	}
	return left, right
}

func (m *Model) selectedText() string {
	if !m.hasTextSelection() {
		return ""
	}
	first, last := m.selection.Anchor, m.selection.Head
	if pointBefore(last, first) {
		first, last = last, first
	}
	var rows []string
	for i := first.Row; i <= last.Row && i < len(m.selection.Rows); i++ {
		left, right := m.selectionRange(i)
		if right > left {
			rows = append(rows, ansi.Cut(ansi.Strip(m.selection.Rows[i].Text), left, right))
		}
	}
	return strings.Join(rows, "\n")
}

func (m *Model) paintSelection(text string, index int, surface selectionSurface) string {
	if m.selection.Surface != surface {
		return text
	}
	left, right := m.selectionRange(index)
	if right <= left {
		return text
	}
	return ansi.Cut(text, 0, left) +
		textSelectionStyle.Render(ansi.Strip(ansi.Cut(text, left, right))) +
		ansi.Cut(text, right, ansi.StringWidth(text))
}

func (m *Model) copyTextSelection() tea.Cmd {
	value := m.selectedText()
	if value == "" {
		return nil
	}
	m.selection.Down = false
	m.mouseSequence++
	m.message = "Selected text sent to terminal clipboard"
	return tea.SetClipboard(value)
}
