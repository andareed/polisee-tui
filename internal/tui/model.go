// Package tui implements the GUI's policy browsing workflow in Bubble Tea v2.
package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-lib"
	"github.com/andareed/polisee-tui/internal/policy"
)

type focus int

const (
	treeFocus focus = iota
	detailFocus
	inputFocus
)

type overlay int

const (
	noOverlay overlay = iota
	helpOverlay
	fieldsOverlay
	fileOverlay
	segmentsOverlay
	rangesOverlay
)

type treeRow struct {
	Key   string
	Depth int
	Pin   bool
}
type searchDue struct{ sequence uint64 }
type searchResult struct {
	sequence uint64
	doc      *policy.Document
	ids      []string
}
type loadResult struct {
	sequence uint64
	path     string
	doc      *policy.Document
	err      error
}
type pathResult struct {
	sequence uint64
	options  []string
}

type browseState struct {
	Filter               string
	Page, Cursor, Offset int
	Matches              []int
}

type Model struct {
	doc                        *policy.Document
	width, height              int
	focus                      focus
	inputs                     [5]textinput.Model
	input                      int
	filterOpen                 bool
	suggestion                 int
	expanded                   map[string]bool
	pins                       []string
	tree                       []treeRow
	cursor, treeOffset         int
	selected                   string
	visible                    map[string]bool
	matches                    map[string]bool
	matchIDs                   []string
	searchSequence             uint64
	searchPending              bool
	loadSequence               uint64
	loading                    bool
	path                       string
	sectionOpen                map[string]map[string]bool
	detail                     []detailLine
	detailCursor, detailOffset int
	headings                   []int
	diagnostics, textMode      bool
	mouseEnabled               bool
	selection                  textSelection
	mouseSequence              uint64
	overlay                    overlay
	quitPending                bool
	inspectReturn              overlay
	dialog                     textinput.Model
	inspected                  *fields
	inspectLines               []string
	overlayOffset              int
	segmentStates              map[string]*browseState
	rangeStates                map[string]*browseState
	segment                    int
	browseEditing              bool
	fileOptions                []string
	pathSequence               uint64
	message                    string
	failed                     bool
}

func New(doc *policy.Document) *Model {
	m := &Model{width: 120, height: 40, dialog: newInput("Path to a local policy XML export"), suggestion: -1, mouseEnabled: true}
	placeholders := []string{"Search policies, values…", "Property or field label", "Group name or ID", "Action name", "Action parameter value"}
	for i, text := range placeholders {
		m.inputs[i] = newInput(text)
	}
	m.setDocument(doc, "")
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) setDocument(doc *policy.Document, path string) {
	m.clearTextSelection()
	m.doc = doc
	m.path = path
	m.expanded = map[string]bool{}
	m.pins = nil
	m.sectionOpen = map[string]map[string]bool{}
	m.segmentStates = map[string]*browseState{}
	m.rangeStates = map[string]*browseState{}
	m.visible = nil
	m.matches = map[string]bool{}
	m.matchIDs = append([]string{}, doc.RuleOrder...)
	m.searchSequence++
	m.searchPending = false
	m.selected = ""
	m.cursor = 0
	m.treeOffset = 0
	m.detailCursor = 0
	m.detailOffset = 0
	for _, n := range doc.Nodes {
		if n.Kind == "folder" || n.Kind == "policy" {
			m.expanded[n.Key] = true
		}
	}
	if len(doc.RuleOrder) > 0 {
		m.selected = "rule:" + doc.InitialID
	} else if len(doc.Children[""]) > 0 {
		m.selected = doc.Children[""][0]
	}
	for i := range m.inputs {
		m.inputs[i].Reset()
		m.inputs[i].Blur()
	}
	m.focus = treeFocus
	m.overlay = noOverlay
	m.suggestion = -1
	m.dialog.Blur()
	m.inspectLines = nil
	m.inspected = nil
	m.overlayOffset = 0
	m.message = "Ready · local policy definitions"
	m.failed = false
	m.rebuildTree()
	m.resizeInputs()
	m.rebuildDetails()
}

func (m *Model) query() polisee.Query {
	return polisee.Query{Text: m.inputs[0].Value(), Property: m.inputs[1].Value(), Group: m.inputs[2].Value(), Action: m.inputs[3].Value(), ActionValue: m.inputs[4].Value()}
}
func (m *Model) filtering() bool {
	for _, input := range m.inputs {
		if strings.TrimSpace(input.Value()) != "" {
			return true
		}
	}
	return false
}
func (m *Model) activeFilters() int {
	n := 0
	for i := 1; i < 5; i++ {
		if strings.TrimSpace(m.inputs[i].Value()) != "" {
			n++
		}
	}
	return n
}

func (m *Model) scheduleSearch() tea.Cmd {
	m.searchSequence++
	sequence := m.searchSequence
	m.searchPending = true
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return searchDue{sequence} })
}
func (m *Model) searchCommand() tea.Cmd {
	doc, query, sequence := m.doc, m.query(), m.searchSequence
	return func() tea.Msg {
		matches := doc.Engine.Search(query)
		set := map[string]bool{}
		for _, match := range matches {
			set[match.RuleID] = true
		}
		var ids []string
		for _, id := range doc.RuleOrder {
			if set[id] {
				ids = append(ids, id)
			}
		}
		return searchResult{sequence, doc, ids}
	}
}
func (m *Model) applySearch(result searchResult) {
	m.matchIDs = result.ids
	m.matches = map[string]bool{}
	for _, id := range result.ids {
		m.matches[id] = true
	}
	m.visible = nil
	if m.filtering() {
		m.visible = m.doc.Visible(result.ids)
		for key := range m.visible {
			if parent := m.doc.Nodes[key].Parent; parent != "" {
				m.expanded[parent] = true
			}
		}
	}
	m.searchPending = false
	m.rebuildTree()
	if m.filtering() && len(result.ids) > 0 {
		m.selectKey("rule:" + result.ids[0])
	}
	if len(m.tree) == 0 {
		m.selected = ""
		m.detailCursor, m.detailOffset = 0, 0
		m.rebuildDetails()
	}
	if len(m.tree) > 0 {
		found := false
		for _, row := range m.tree {
			if row.Key == m.selected {
				found = true
				break
			}
		}
		if !found {
			if len(result.ids) > 0 {
				m.selectKey("rule:" + result.ids[0])
			} else {
				m.selectKey(m.tree[0].Key)
			}
		}
	}
}

func (m *Model) clearSearch() tea.Cmd {
	for i := range m.inputs {
		m.inputs[i].Reset()
	}
	m.suggestion = -1
	m.searchSequence++
	m.applySearch(searchResult{m.searchSequence, m.doc, append([]string{}, m.doc.RuleOrder...)})
	m.message = "Filters cleared"
	return nil
}

func (m *Model) rebuildTree() {
	oldKey, oldPin := m.selected, false
	if m.cursor >= 0 && m.cursor < len(m.tree) {
		oldPin = m.tree[m.cursor].Pin
	}
	var rows []treeRow
	for _, id := range m.pins {
		key := "rule:" + id
		if m.doc.Nodes[key] != nil && (m.visible == nil || m.visible[key]) {
			rows = append(rows, treeRow{Key: key, Pin: true})
		}
	}
	var visit func(string, int)
	visit = func(parent string, depth int) {
		for _, key := range m.doc.Children[parent] {
			if m.visible != nil && !m.visible[key] {
				continue
			}
			rows = append(rows, treeRow{Key: key, Depth: depth})
			if m.expanded[key] {
				visit(key, depth+1)
			}
		}
	}
	visit("", 0)
	m.tree = rows
	m.cursor = max(0, min(m.cursor, len(rows)-1))
	for i, row := range rows {
		if row.Key == oldKey {
			m.cursor = i
			if row.Pin == oldPin {
				break
			}
		}
	}
	m.clampTree()
}

func (m *Model) selectKey(key string) {
	if m.doc.Nodes[key] == nil {
		return
	}
	if m.selected != key {
		m.selected = key
		m.detailCursor = 0
		m.detailOffset = 0
		m.rebuildDetails()
	}
	for i, row := range m.tree {
		if row.Key == key {
			m.cursor = i
			break
		}
	}
	m.clampTree()
}
func (m *Model) moveTree(delta int) {
	if len(m.tree) == 0 {
		return
	}
	m.cursor = max(0, min(m.cursor+delta, len(m.tree)-1))
	row := m.tree[m.cursor]
	m.selected = row.Key
	m.detailCursor = 0
	m.detailOffset = 0
	m.rebuildDetails()
	m.clampTree()
}
func (m *Model) reveal(key string) {
	for n := m.doc.Nodes[key]; n != nil && n.Parent != ""; n = m.doc.Nodes[n.Parent] {
		m.expanded[n.Parent] = true
	}
	m.rebuildTree()
}
func (m *Model) branch(open bool) {
	if len(m.tree) == 0 {
		return
	}
	key := m.tree[m.cursor].Key
	if len(m.doc.Children[key]) > 0 && (m.expanded[key] || open) {
		m.expanded[key] = open
		m.rebuildTree()
	} else if !open {
		parent := m.doc.Nodes[key].Parent
		if parent != "" {
			m.selectKey(parent)
		}
	}
}
func (m *Model) jumpResult(delta int) {
	if len(m.matchIDs) == 0 {
		return
	}
	id := strings.TrimPrefix(m.selected, "rule:")
	index := slices.Index(m.matchIDs, id)
	if index < 0 {
		index = -1
		if delta < 0 {
			index = 0
		}
	}
	index = (index + delta + len(m.matchIDs)) % len(m.matchIDs)
	key := "rule:" + m.matchIDs[index]
	m.reveal(key)
	m.selectKey(key)
	m.message = fmt.Sprintf("Result %d of %d", index+1, len(m.matchIDs))
}
func (m *Model) pin() {
	n := m.doc.Nodes[m.selected]
	if n == nil || n.Rule == nil {
		return
	}
	if at := slices.Index(m.pins, n.ID); at >= 0 {
		m.pins = slices.Delete(m.pins, at, at+1)
		m.message = "Policy unpinned"
	} else {
		m.pins = append(m.pins, n.ID)
		m.message = "Policy pinned"
	}
	m.rebuildTree()
}
func (m *Model) copyID() tea.Cmd {
	n := m.doc.Nodes[m.selected]
	if n == nil {
		return nil
	}
	m.message = "ID sent to terminal clipboard (OSC 52)"
	return tea.SetClipboard(n.ID)
}

func (m *Model) blur() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.dialog.Blur()
	m.suggestion = -1
}
func (m *Model) focusPane(pane focus) {
	m.blur()
	m.focus = pane
}
func (m *Model) focusInput(index int) tea.Cmd {
	m.blur()
	m.focus = inputFocus
	m.input = index
	m.filterOpen = true
	m.resizeInputs()
	m.clampTree()
	return m.inputs[index].Focus()
}
func (m *Model) suggestions() []policy.Option {
	if m.focus != inputFocus || m.input == 0 {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(m.inputs[m.input].Value()))
	var options []policy.Option
	for _, option := range m.doc.Options[m.input] {
		if query == "" || strings.Contains(strings.ToLower(option.Name+" "+option.ID), query) {
			options = append(options, option)
			if len(options) == 5 {
				break
			}
		}
	}
	return options
}

func (m *Model) load(path string) tea.Cmd {
	m.loadSequence++
	sequence := m.loadSequence
	m.loading = true
	m.message = "Loading policy…"
	m.failed = false
	return func() tea.Msg {
		doc, err := policy.LoadFile(context.Background(), path)
		return loadResult{sequence, path, doc, err}
	}
}
func (m *Model) LoadPath(path string) tea.Cmd { return m.load(path) }

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitPending {
		switch msg := message.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "enter":
				return m, tea.Quit
			case "esc":
				m.quitPending = false
			}
			return m, nil
		case tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg, tea.MouseWheelMsg, tea.PasteMsg, selectionHeld:
			return m, nil
		}
	}
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.clearTextSelection()
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		m.resizeInputs()
		m.rebuildDetails()
		m.clampTree()
		if m.overlay == segmentsOverlay || m.overlay == rangesOverlay {
			m.clampBrowse(m.browseState())
		}
		return m, nil
	case searchDue:
		if msg.sequence == m.searchSequence {
			return m, m.searchCommand()
		}
		return m, nil
	case searchResult:
		if msg.sequence == m.searchSequence && msg.doc == m.doc {
			m.clearTextSelection()
			m.applySearch(msg)
		}
		return m, nil
	case loadResult:
		if msg.sequence != m.loadSequence {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.message = "Open failed: " + msg.err.Error()
			m.failed = true
			return m, nil
		}
		m.setDocument(msg.doc, msg.path)
		m.message = "Opened " + msg.doc.Name
		return m, nil
	case pathResult:
		if msg.sequence == m.pathSequence && m.overlay == fileOverlay {
			m.fileOptions = msg.options
		}
		return m, nil
	case selectionHeld:
		m.holdSelection(msg)
		return m, nil
	case tea.MouseClickMsg:
		return m, m.mousePress(msg.Mouse())
	case tea.MouseMotionMsg:
		m.mouseMotion(msg.Mouse())
		return m, nil
	case tea.MouseReleaseMsg:
		return m, m.mouseRelease(msg.Mouse())
	case tea.MouseWheelMsg:
		m.clearTextSelection()
		m.wheel(msg.Mouse())
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+q" {
			m.quitPending = true
			return m, nil
		}
		if m.hasTextSelection() {
			if key == "ctrl+c" {
				return m, m.copyTextSelection()
			}
			if key == "esc" {
				m.clearTextSelection()
				return m, nil
			}
		}
		m.clearTextSelection()
		if key == "ctrl+c" && m.focus != inputFocus && m.overlay != fileOverlay && !m.browseEditing {
			return m, m.copyID()
		}
		if key == "ctrl+o" {
			return m, m.openFile()
		}
		if m.overlay != noOverlay {
			return m, m.overlayKey(msg)
		}
		switch key {
		case "alt+h":
			m.focusPane(treeFocus)
			return m, nil
		case "alt+l":
			m.focusPane(detailFocus)
			return m, nil
		case "ctrl+r":
			return m, m.clearSearch()
		}
		if m.focus == inputFocus {
			return m, m.inputKey(msg)
		}
		switch key {
		case "?":
			m.overlay = helpOverlay
			m.overlayOffset = 0
			return m, nil
		case "/":
			return m, m.focusInput(0)
		case "esc":
			m.filterOpen = false
			m.focusPane(treeFocus)
			m.clampTree()
			m.failed = false
			m.message = "Ready"
			return m, nil
		case "n":
			m.jumpResult(1)
			return m, nil
		case "N", "shift+n":
			m.jumpResult(-1)
			return m, nil
		case "p":
			m.pin()
			return m, nil
		case "d":
			m.diagnostics = !m.diagnostics
			m.rebuildDetails()
			m.message = "Diagnostics hidden"
			if m.diagnostics {
				m.message = "Diagnostics shown · Enter inspects a row"
			}
			return m, nil
		case "v":
			m.textMode = !m.textMode
			m.rebuildDetails()
			return m, nil
		case "r":
			if m.path != "" {
				return m, m.load(m.path)
			}
			m.message = "Demo is already loaded"
			return m, nil
		}
		if m.focus == treeFocus {
			switch key {
			case "j", "down":
				m.moveTree(1)
			case "k", "up":
				m.moveTree(-1)
			case "pgdown":
				m.moveTree(m.treeHeight())
			case "pgup":
				m.moveTree(-m.treeHeight())
			case "home":
				m.moveTree(-len(m.tree))
			case "end":
				m.moveTree(len(m.tree))
			case "h", "left":
				m.branch(false)
			case "l", "right":
				m.branch(true)
			case "enter":
				if len(m.tree) > 0 {
					m.focusPane(detailFocus)
				}
			case "e":
				open := false
				for k := range m.doc.Nodes {
					if len(m.doc.Children[k]) > 0 && !m.expanded[k] {
						open = true
						break
					}
				}
				for k := range m.doc.Nodes {
					m.expanded[k] = open
				}
				m.rebuildTree()
				if len(m.tree) > 0 {
					m.selectKey(m.tree[m.cursor].Key)
				}
			}
		} else {
			m.detailKey(key)
		}
		return m, nil
	default:
		if m.overlay == fileOverlay || m.browseEditing {
			old := m.dialog.Value()
			var cmd tea.Cmd
			m.dialog, cmd = m.dialog.Update(message)
			if old != m.dialog.Value() {
				if m.overlay == fileOverlay {
					return m, tea.Batch(cmd, m.pathCommand())
				}
				m.filterBrowse()
			}
			return m, cmd
		}
		if m.focus == inputFocus {
			old := m.inputs[m.input].Value()
			var cmd tea.Cmd
			m.inputs[m.input], cmd = m.inputs[m.input].Update(message)
			if old != m.inputs[m.input].Value() {
				m.suggestion = -1
				return m, tea.Batch(cmd, m.scheduleSearch())
			}
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) inputKey(msg tea.KeyPressMsg) tea.Cmd {
	options := m.suggestions()
	switch msg.String() {
	case "esc":
		m.filterOpen = false
		m.focusPane(treeFocus)
		m.clampTree()
		return nil
	case "down":
		if len(options) > 0 {
			m.suggestion = (m.suggestion + 1) % len(options)
		}
		return nil
	case "up":
		if len(options) > 0 {
			m.suggestion = (m.suggestion - 1 + len(options)) % len(options)
		}
		return nil
	case "enter":
		var cmd tea.Cmd
		if m.suggestion >= 0 && m.suggestion < len(options) {
			m.inputs[m.input].SetValue(options[m.suggestion].Value)
			cmd = m.scheduleSearch()
		}
		m.focusPane(treeFocus)
		m.clampTree()
		return cmd
	case "tab":
		return m.focusInput((m.input + 1) % 5)
	case "shift+tab":
		return m.focusInput((m.input + 4) % 5)
	}
	old := m.inputs[m.input].Value()
	var cmd tea.Cmd
	m.inputs[m.input], cmd = m.inputs[m.input].Update(msg)
	if old != m.inputs[m.input].Value() {
		m.suggestion = -1
		return tea.Batch(cmd, m.scheduleSearch())
	}
	return cmd
}

func (m *Model) moveDetail(delta int) {
	m.detailCursor = max(0, min(m.detailCursor+delta, len(m.detail)-1))
	m.clampDetail()
}
func (m *Model) clampDetail() {
	m.detailCursor = max(0, min(m.detailCursor, len(m.detail)-1))
	height := m.detailHeight()
	if m.detailCursor < m.detailOffset {
		m.detailOffset = m.detailCursor
	}
	if m.detailCursor >= m.detailOffset+height {
		m.detailOffset = m.detailCursor - height + 1
	}
	m.detailOffset = max(0, min(m.detailOffset, max(0, len(m.detail)-height)))
}
func (m *Model) clampTree() {
	height := m.treeHeight()
	if m.cursor < m.treeOffset {
		m.treeOffset = m.cursor
	}
	if m.cursor >= m.treeOffset+height {
		m.treeOffset = m.cursor - height + 1
	}
	m.treeOffset = max(0, min(m.treeOffset, max(0, len(m.tree)-height)))
}
func (m *Model) nearestSection() string {
	for i := min(m.detailCursor, len(m.detail)-1); i >= 0; i-- {
		if m.detail[i].Section != "" {
			return m.detail[i].Section
		}
	}
	return ""
}
func (m *Model) toggleSection(set *bool) {
	key := m.nearestSection()
	if key == "" {
		return
	}
	state := m.sectionOpen[m.selected]
	for i, line := range m.detail {
		if line.Section == key {
			m.detailCursor = i
			break
		}
	}
	if set != nil {
		state[key] = *set
	} else {
		state[key] = !state[key]
	}
	m.rebuildDetails()
}
func (m *Model) detailKey(key string) {
	switch key {
	case "j", "down":
		m.moveDetail(1)
	case "k", "up":
		m.moveDetail(-1)
	case "pgdown":
		m.moveDetail(m.detailHeight())
	case "pgup":
		m.moveDetail(-m.detailHeight())
	case "home":
		m.moveDetail(-len(m.detail))
	case "end":
		m.moveDetail(len(m.detail))
	case "[", "]":
		target := -1
		if key == "]" {
			for _, at := range m.headings {
				if at > m.detailCursor {
					target = at
					break
				}
			}
		} else {
			for _, at := range m.headings {
				if at < m.detailCursor {
					target = at
				}
			}
		}
		if target >= 0 {
			m.detailCursor = target
			m.clampDetail()
		}
	case "h", "left", "l", "right":
		if m.detailCursor < len(m.detail) && m.detail[m.detailCursor].Section != "" {
			open := key == "l" || key == "right"
			m.toggleSection(&open)
		}
	case "enter":
		if m.detailCursor >= len(m.detail) {
			return
		}
		line := m.detail[m.detailCursor]
		switch {
		case line.Section != "":
			m.toggleSection(nil)
		case line.RuleKey != "":
			m.reveal(line.RuleKey)
			m.selectKey(line.RuleKey)
		case line.Browse:
			m.openSegments()
		case line.Fields != nil:
			m.openFields()
		}
	case "e":
		state := m.sectionOpen[m.selected]
		open := false
		for _, expanded := range state {
			open = open || !expanded
		}
		for section := range state {
			state[section] = open
		}
		m.rebuildDetails()
	}
}

func (m *Model) resizeInputs() {
	for i := range m.inputs {
		label := 2
		if i > 0 {
			label = 11
		}
		m.inputs[i].SetWidth(max(1, m.sidebarWidth()-label-3))
	}
	m.dialog.SetWidth(max(1, m.width-12))
}
func (m *Model) bodyHeight() int { return max(1, m.height-5) }
func (m *Model) sidebarWidth() int {
	if m.width < 78 {
		return m.width
	}
	return min(42, max(27, m.width/3))
}
func (m *Model) paneWidth() int {
	if m.width < 78 {
		return m.width
	}
	return max(1, m.width-m.sidebarWidth()-1)
}
func (m *Model) contentWidth() int { return max(1, m.paneWidth()-4) }
func (m *Model) detailHeight() int { return max(1, m.bodyHeight()-2) }
func (m *Model) treeHeight() int   { return max(1, m.bodyHeight()-len(m.sidebarHeader())-1) }

// StartPath records only the caller's explicit command-line file; no startup
// directory is scanned for private exports.
func (m *Model) StartPath(path string) { m.path = filepath.Clean(path) }

func (m *Model) SetMouseEnabled(enabled bool) { m.mouseEnabled = enabled }
