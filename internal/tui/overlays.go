package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-lib"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

var helpText = []string{
	"Keyboard shortcuts",
	"",
	"Alt+H / Alt+L     Focus tree / details",
	"j/k or ↑/↓        Move in the focused pane; type normally in editors",
	"h/l or ←/→        Collapse / expand a branch or section heading",
	"Enter             Open or inspect the selected row",
	"Esc               Close filters or an inspector; return to the tree",
	"Page Up/Down      Page in the focused pane",
	"Home/End          Go to the start / end",
	"[ / ]             Jump between detail headings",
	"e                 Toggle expand/collapse all in the focused pane",
	"",
	"/                 Open Text, Property, Group, Action and Value filters",
	"Tab / Shift+Tab   Next / previous filter field",
	"↑/↓ then Enter    Choose a suggestion; raw IDs also work",
	"Ctrl+R            Clear all policy filters",
	"n / N             Next / previous direct match",
	"",
	"p                 Pin / unpin the selected policy",
	"Ctrl+C            Copy selected text, otherwise the selected ID",
	"d                 Toggle contextual diagnostic strips",
	"v                 Switch Visual / Text conditions",
	"Ctrl+O            Open a local XML export; Tab completes paths",
	"Ctrl+Q            Quit confirmation: Enter quits, Esc cancels",
	"r                 Reload the explicitly opened file",
	"",
	"In segment/range browsers:",
	"/                 Edit the literal name, address, ID or value filter",
	"Enter             Open selected segment / range fields",
	"PgUp/PgDn         Previous / next page (8 segments, 25 ranges)",
	"←/→               Previous / next segment in the range inspector",
	"m                 Inspect segment metadata",
	"Esc               Leave filter, then return to the previous view",
	"",
	"Click rows and headings; the mouse wheel scrolls the owning pane.",
	"Drag or hold and drag in details/fields to select text.",
	"Ctrl+C copies the selection; Esc clears it.",
	"",
	"Policy matching and field labels come from polisee-lib.",
	"External expressions remain unavailable. Criteria are not evaluated.",
	"Range search matches literal exported text; it does not infer containment.",
	"Exports are read locally and never modified or uploaded.",
}

func (m *Model) showFields(context *fields) {
	if context == nil {
		return
	}
	m.inspectReturn = m.overlay
	m.overlay = fieldsOverlay
	m.inspected = context
	m.overlayOffset = 0
	m.inspectLines = fieldLines(context, max(1, m.width-6))
}
func (m *Model) openFields() {
	context := m.nodeFields(m.doc.Nodes[m.selected])
	if m.focus == detailFocus && m.detailCursor < len(m.detail) && m.detail[m.detailCursor].Fields != nil {
		context = m.detail[m.detailCursor].Fields
	}
	m.showFields(context)
}
func fieldLines(context *fields, width int) []string {
	var lines []string
	add := func(value string) {
		for _, line := range strings.Split(ansi.Wrap(safe(value), width, " "), "\n") {
			lines = append(lines, normal.Render(line))
		}
	}
	var visit func(polisee.Element, int)
	visit = func(n polisee.Element, depth int) {
		prefix := strings.Repeat("  ", min(depth, 16))
		add(prefix + n.Name)
		if n.Namespace != "" {
			add(prefix + "  namespace = " + strconv.Quote(n.Namespace))
		}
		for _, a := range diagnosticAttributes(n.Attributes) {
			name := a.Name
			if a.Namespace != "" {
				name = "{" + a.Namespace + "}" + name
			}
			add(prefix + "  " + name + " = " + strconv.Quote(a.Value))
		}
		if n.Text != "" {
			add(prefix + "  text = " + strconv.Quote(n.Text))
		}
		for _, path := range n.ExternalExpressions {
			add(prefix + "  External expression unavailable: " + strconv.Quote(path))
		}
		for _, child := range n.Children {
			visit(child, depth+1)
		}
	}
	visit(polisee.Element{Name: context.Name, Attributes: context.Attributes, Text: context.Text, Children: context.Children, ExternalExpressions: context.External}, 0)
	for _, note := range context.Notes {
		add(note)
	}
	return lines
}

func ExpandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
		}
	}
	return filepath.Clean(path)
}
func (m *Model) openFile() tea.Cmd {
	m.overlay = fileOverlay
	m.browseEditing = false
	m.suggestion = -1
	m.fileOptions = nil
	m.dialog = newInput("Path to a local policy XML export")
	m.dialog.SetWidth(max(1, m.width-10))
	m.failed = false
	m.message = "Open a local XML export"
	m.pathSequence++
	return tea.Batch(m.dialog.Focus(), m.pathCommand())
}
func (m *Model) pathCommand() tea.Cmd {
	m.pathSequence++
	sequence := m.pathSequence
	value := m.dialog.Value()
	return func() tea.Msg {
		path := ExpandPath(value)
		dir, base := filepath.Split(path)
		if strings.HasSuffix(value, "/") || value == "" {
			dir = path
			base = ""
		}
		if dir == "" {
			dir = "."
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return pathResult{sequence: sequence}
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return policy.NaturalLess(entries[i].Name(), entries[j].Name())
		})
		var options []string
		for _, entry := range entries {
			if !strings.HasPrefix(strings.ToLower(entry.Name()), strings.ToLower(base)) {
				continue
			}
			if !entry.IsDir() && !strings.EqualFold(filepath.Ext(entry.Name()), ".xml") {
				continue
			}
			candidate := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				candidate += "/"
			}
			options = append(options, candidate)
			if len(options) == 8 {
				break
			}
		}
		return pathResult{sequence, options}
	}
}
func (m *Model) closeOverlay() tea.Cmd {
	m.overlay = noOverlay
	m.browseEditing = false
	m.dialog.Blur()
	m.suggestion = -1
	if m.focus == inputFocus {
		return m.inputs[m.input].Focus()
	}
	return nil
}
func (m *Model) fileKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		return m.closeOverlay()
	case "up":
		if len(m.fileOptions) > 0 {
			m.suggestion = (m.suggestion - 1 + len(m.fileOptions)) % len(m.fileOptions)
		}
		return nil
	case "down":
		if len(m.fileOptions) > 0 {
			m.suggestion = (m.suggestion + 1) % len(m.fileOptions)
		}
		return nil
	case "tab", "enter":
		value := m.dialog.Value()
		if (msg.String() == "tab" || m.suggestion >= 0) && len(m.fileOptions) > 0 {
			value = m.fileOptions[max(0, m.suggestion)]
			m.dialog.SetValue(value)
			m.dialog.CursorEnd()
			m.suggestion = -1
			if msg.String() == "tab" || strings.HasSuffix(value, "/") {
				return m.pathCommand()
			}
		}
		if strings.TrimSpace(value) == "" {
			m.failed = true
			m.message = "Enter a path to a policy XML export"
			return nil
		}
		path := ExpandPath(value)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			m.dialog.SetValue(path + "/")
			m.dialog.CursorEnd()
			return m.pathCommand()
		}
		m.closeOverlay()
		return m.load(path)
	}
	old := m.dialog.Value()
	var cmd tea.Cmd
	m.dialog, cmd = m.dialog.Update(msg)
	if old != m.dialog.Value() {
		m.suggestion = -1
		return tea.Batch(cmd, m.pathCommand())
	}
	return cmd
}

func (m *Model) openSegments() tea.Cmd {
	n := m.doc.Nodes[m.selected]
	if n == nil || n.Rule == nil || len(m.doc.Segments[n.ID]) == 0 {
		m.message = "No exported segments for this policy"
		return nil
	}
	m.overlay = segmentsOverlay
	m.browseEditing = false
	m.dialog.Blur()
	m.segmentState()
	return nil
}
func (m *Model) segments() []policy.Segment {
	n := m.doc.Nodes[m.selected]
	if n == nil {
		return nil
	}
	return m.doc.Segments[n.ID]
}
func (m *Model) segmentState() *browseState {
	state := m.segmentStates[m.selected]
	if state == nil {
		state = &browseState{Matches: policy.MatchingSegments(m.segments(), "")}
		m.segmentStates[m.selected] = state
	}
	return state
}
func (m *Model) rangeState() *browseState {
	segments := m.segments()
	if len(segments) == 0 {
		return &browseState{}
	}
	m.segment = max(0, min(m.segment, len(segments)-1))
	key := m.selected + "\x00" + segments[m.segment].Key
	state := m.rangeStates[key]
	if state == nil {
		state = &browseState{Matches: segments[m.segment].MatchingRanges("")}
		m.rangeStates[key] = state
	}
	return state
}
func (m *Model) browseState() *browseState {
	if m.overlay == rangesOverlay {
		return m.rangeState()
	}
	return m.segmentState()
}
func (m *Model) browseSize() int {
	if m.overlay == rangesOverlay {
		return policy.RangePageSize
	}
	return policy.SegmentPageSize
}
func (m *Model) browseHeaderHeight() int {
	if m.bodyHeight() < 7 {
		return 3
	}
	if m.bodyHeight() < 12 {
		return 4
	}
	return 6
}
func (m *Model) browseFooterHeight() int {
	if m.bodyHeight() < 7 {
		return 1
	}
	return 2
}
func (m *Model) browseRows() int {
	return max(1, m.bodyHeight()-m.browseHeaderHeight()-m.browseFooterHeight())
}
func (m *Model) clampBrowse(state *browseState) {
	state.Cursor = max(0, min(state.Cursor, len(state.Matches)-1))
	page := state.Cursor / m.browseSize()
	if page != state.Page {
		state.Page = page
		state.Offset = 0
	}
	relative := state.Cursor - state.Page*m.browseSize()
	if relative < state.Offset {
		state.Offset = relative
	}
	if relative >= state.Offset+m.browseRows() {
		state.Offset = relative - m.browseRows() + 1
	}
	_, start, end := policy.PageBounds(state.Page, len(state.Matches), m.browseSize())
	state.Offset = max(0, min(state.Offset, max(0, end-start-m.browseRows())))
}
func (m *Model) browseMove(delta int) {
	state := m.browseState()
	state.Cursor += delta
	m.clampBrowse(state)
}
func (m *Model) browsePage(delta int) {
	state := m.browseState()
	page, _, _ := policy.PageBounds(state.Page+delta, len(state.Matches), m.browseSize())
	state.Cursor = page * m.browseSize()
	m.clampBrowse(state)
}
func (m *Model) startBrowseFilter() tea.Cmd {
	m.browseEditing = true
	m.dialog = newInput("Literal exported text…")
	m.dialog.SetWidth(max(1, m.width-20))
	m.dialog.SetValue(m.browseState().Filter)
	m.dialog.CursorEnd()
	return m.dialog.Focus()
}
func (m *Model) filterBrowse() {
	state := m.browseState()
	state.Filter = m.dialog.Value()
	state.Cursor = 0
	state.Page = 0
	state.Offset = 0
	if m.overlay == rangesOverlay {
		state.Matches = m.segments()[m.segment].MatchingRanges(state.Filter)
	} else {
		state.Matches = policy.MatchingSegments(m.segments(), state.Filter)
	}
	m.clampBrowse(state)
}
func (m *Model) openRange() {
	state := m.segmentState()
	if len(state.Matches) == 0 {
		return
	}
	m.segment = state.Matches[state.Cursor]
	m.overlay = rangesOverlay
	m.browseEditing = false
	m.clampBrowse(m.rangeState())
}
func (m *Model) browseInspect() {
	if m.overlay == segmentsOverlay {
		state := m.segmentState()
		if len(state.Matches) == 0 {
			return
		}
		m.showFields(elementFields(m.segments()[state.Matches[state.Cursor]].Metadata))
	} else {
		state := m.rangeState()
		if len(state.Matches) == 0 {
			return
		}
		m.showFields(elementFields(m.segments()[m.segment].Ranges[state.Matches[state.Cursor]]))
	}
}
func (m *Model) overlayKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if m.overlay == fileOverlay {
		return m.fileKey(msg)
	}
	if m.overlay == helpOverlay || m.overlay == fieldsOverlay {
		_, content := m.overlayContent()
		count := len(content)
		switch key {
		case "esc":
			if m.overlay == fieldsOverlay {
				m.overlay = m.inspectReturn
				return nil
			}
			return m.closeOverlay()
		case "j", "down":
			m.overlayOffset++
		case "k", "up":
			m.overlayOffset--
		case "pgdown":
			m.overlayOffset += max(1, m.bodyHeight()-3)
		case "pgup":
			m.overlayOffset -= max(1, m.bodyHeight()-3)
		case "home":
			m.overlayOffset = 0
		case "end":
			m.overlayOffset = count
		}
		m.overlayOffset = max(0, min(m.overlayOffset, max(0, count-(m.bodyHeight()-3))))
		return nil
	}
	if m.browseEditing {
		switch key {
		case "esc", "enter", "tab":
			m.browseEditing = false
			m.dialog.Blur()
			return nil
		case "ctrl+r":
			m.dialog.Reset()
			m.filterBrowse()
			return nil
		}
		var cmd tea.Cmd
		old := m.dialog.Value()
		m.dialog, cmd = m.dialog.Update(msg)
		if old != m.dialog.Value() {
			m.filterBrowse()
		}
		return cmd
	}
	switch key {
	case "esc":
		if m.overlay == rangesOverlay {
			m.overlay = segmentsOverlay
			return nil
		}
		return m.closeOverlay()
	case "/":
		return m.startBrowseFilter()
	case "j", "down":
		m.browseMove(1)
	case "k", "up":
		m.browseMove(-1)
	case "pgup":
		m.browsePage(-1)
	case "pgdown":
		m.browsePage(1)
	case "home":
		m.browseMove(-len(m.browseState().Matches))
	case "end":
		m.browseMove(len(m.browseState().Matches))
	case "left", "right":
		if m.overlay == rangesOverlay {
			delta := 1
			if key == "left" {
				delta = -1
			}
			m.segment = (m.segment + delta + len(m.segments())) % len(m.segments())
			m.clampBrowse(m.rangeState())
		}
	case "enter":
		if m.overlay == segmentsOverlay {
			m.openRange()
		} else {
			m.browseInspect()
		}
	case "m":
		if m.overlay == segmentsOverlay {
			m.browseInspect()
		} else {
			m.showFields(elementFields(m.segments()[m.segment].Metadata))
		}
	case "d":
		m.diagnostics = !m.diagnostics
		m.rebuildDetails()
	}
	return nil
}

func (m *Model) overlayContent() (string, []string) {
	width := m.width
	title := "Keyboard shortcuts"
	var content []string
	switch m.overlay {
	case helpOverlay:
		for _, line := range helpText[1:] {
			for _, wrapped := range strings.Split(ansi.Wrap(line, max(1, width-6), " "), "\n") {
				content = append(content, normal.Render(wrapped))
			}
		}
	case fieldsOverlay:
		title = "Exported fields · " + m.inspected.Name + " · exact quoted values"
		content = fieldLines(m.inspected, max(1, width-6))
	case fileOverlay:
		title = "Open policy XML · local files"
		content = []string{"", m.dialog.View(), "", muted.Render("Enter opens · Tab completes · ↑/↓ suggestions · Esc returns"), ""}
		for i, option := range m.fileOptions {
			style := muted
			prefix := "  "
			if i == m.suggestion {
				style = accent
				prefix = "› "
			}
			content = append(content, style.Render(prefix+one(option)))
		}
		content = append(content, "", dim.Render("Exports up to 64 MiB · failed opens keep the current document."))
	}
	return title, content
}

func (m *Model) overlayLines() []string {
	width, height := m.width, m.bodyHeight()
	if m.overlay == segmentsOverlay || m.overlay == rangesOverlay {
		return m.browseLines()
	}
	title, content := m.overlayContent()
	lines := []string{" " + accent.Bold(true).Render(one(title)), " " + dim.Render(strings.Repeat("─", max(0, width-2)))}
	offset := max(0, min(m.overlayOffset, max(0, len(content)-(height-3))))
	for i := 0; i < height-3; i++ {
		at := offset + i
		text := ""
		if at < len(content) {
			text = content[at]
			if m.overlay == fieldsOverlay {
				text = m.paintSelection(text, at, exportedText)
			}
		}
		lines = append(lines, "  "+fit(text, max(0, width-4))+"  ")
	}
	lines = append(lines, " "+fit(dim.Render("Esc returns · j/k scroll · PgUp/PgDn page"), max(0, width-2))+" ")
	return lines
}

func (m *Model) browseLines() []string {
	width, height := m.width, m.bodyHeight()
	state := m.browseState()
	segments := m.segments()
	title := "Segments · " + m.doc.Nodes[m.selected].Name
	total := len(segments)
	metadata := "Select a segment to inspect its ranges."
	if m.overlay == rangesOverlay {
		s := segments[m.segment]
		name := s.Name
		if name == "" {
			name = "Unnamed segment"
		}
		title = "Segment ranges · " + name + "  · ID " + shown(s.ID)
		total = len(s.Ranges)
		metadata = "Metadata: " + readable(s.Metadata) + "  [m] complete fields"
	}
	query := one(state.Filter) + "  [/]"
	if state.Filter == "" {
		query = "All exported values  [/]"
	}
	if m.browseEditing {
		query = m.dialog.View()
	}
	pages := max(1, (len(state.Matches)+m.browseSize()-1)/m.browseSize())
	lines := []string{
		" " + accent.Bold(true).Render(one(title)),
		" " + muted.Render("Rule ID "+m.doc.Nodes[m.selected].ID),
		" " + muted.Render("Find: ") + query,
		" " + muted.Render(fmt.Sprintf("%d / %d · page %d / %d", len(state.Matches), total, state.Page+1, pages)),
		" " + dim.Render(one(metadata)),
		" " + strong.Render(m.tableHeader(width-2)),
	}
	if m.browseHeaderHeight() == 3 {
		lines = []string{lines[0], lines[2], lines[5]}
	} else if m.browseHeaderHeight() == 4 {
		lines = []string{lines[0], lines[2], lines[3], lines[5]}
	}
	_, start, end := policy.PageBounds(state.Page, len(state.Matches), m.browseSize())
	available := m.browseRows()
	for row := 0; row < available; row++ {
		at := start + state.Offset + row
		text := ""
		if at < end {
			source := state.Matches[at]
			if m.overlay == segmentsOverlay {
				s := segments[source]
				name := s.Name
				if name == "" {
					name = "Unnamed segment"
				}
				text = tableRow(one(name), one(shown(s.ID)), fmt.Sprintf("%d ranges", len(s.Ranges)), width-4)
			} else {
				r := segments[m.segment].Ranges[source]
				from, to := policy.Attr(r.Attributes, "FROM"), policy.Attr(r.Attributes, "TO")
				if !policy.HasAttr(r.Attributes, "FROM") {
					from = "—"
				} else {
					from = shown(from)
				}
				if !policy.HasAttr(r.Attributes, "TO") {
					to = "—"
				} else {
					to = shown(to)
				}
				id := shown(policy.Attr(r.Attributes, "ID"))
				if m.diagnostics {
					id += " · " + brief(r.Attributes, "FROM", "TO", "ID")
				}
				text = tableRow(one(from), one(to), one(id), width-4)
			}
			prefix := "  "
			if at == state.Cursor {
				prefix = "› "
				text = selectedStyle.Render(fit(plain(text), width-4))
			}
			text = accent.Render(prefix) + text
		} else if len(state.Matches) == 0 && row == 0 {
			text = muted.Render("No matching exported values. / edits the filter.")
		}
		lines = append(lines, " "+fit(text, max(0, width-2))+" ")
	}
	for len(lines) < height-m.browseFooterHeight() {
		lines = append(lines, strings.Repeat(" ", width))
	}
	lines = append(lines, " "+fit(muted.Render("PgUp/PgDn page · / find · Enter open · m metadata · Esc back"), max(0, width-2))+" ")
	hint := "8 segments per page · state retained for this policy"
	if m.overlay == rangesOverlay {
		hint = "25 ranges per page · ←/→ segment · m metadata · literal text search"
	}
	if m.browseFooterHeight() > 1 {
		lines = append(lines, " "+fit(dim.Render(hint), max(0, width-2))+" ")
	}
	return lines
}
func (m *Model) tableHeader(width int) string {
	if m.overlay == rangesOverlay {
		return "  " + tableRow("From", "To", "ID", max(1, width-2))
	}
	return "  " + tableRow("Name", "ID", "Ranges", max(1, width-2))
}
func tableRow(a, b, c string, width int) string {
	width = max(3, width)
	col3 := max(5, width/4)
	col1 := max(1, (width-col3-6)/2)
	col2 := max(1, width-col1-col3-6)
	return fit(a, col1) + dim.Render(" │ ") + fit(b, col2) + dim.Render(" │ ") + muted.Render(fit(c, col3))
}
