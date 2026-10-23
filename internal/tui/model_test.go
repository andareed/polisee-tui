package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

func demo(t *testing.T) *Model {
	t.Helper()
	doc, err := policy.LoadDemo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return New(doc)
}
func press(m *Model, code rune, mods ...tea.KeyMod) tea.Cmd {
	mod := tea.KeyMod(0)
	if len(mods) > 0 {
		mod = mods[0]
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: code, Mod: mod, Text: keyText(code, mod)})
	return cmd
}
func applyCurrentSearch(m *Model) {
	_, cmd := m.Update(searchDue{m.searchSequence})
	if cmd != nil {
		m.Update(cmd())
	}
}
func TestPaneNavigationAndSections(t *testing.T) {
	m := demo(t)
	selected := m.selected
	press(m, 'l', tea.ModAlt)
	press(m, tea.KeyDown)
	if m.focus != detailFocus || m.selected != selected || m.detailCursor != 1 {
		t.Fatal("detail navigation changed policy")
	}
	press(m, ']')
	if m.detail[m.detailCursor].Section != "scope" {
		t.Fatal("heading navigation missed scope")
	}
	press(m, tea.KeyLeft)
	if m.sectionOpen[selected]["scope"] {
		t.Fatal("scope did not close")
	}
	press(m, tea.KeyRight)
	if !m.sectionOpen[selected]["scope"] {
		t.Fatal("scope did not open")
	}
	press(m, 'h', tea.ModAlt)
	press(m, tea.KeyRight)
	press(m, tea.KeyDown)
	if m.selected != "rule:child" {
		t.Fatalf("tree did not navigate to subrule: %s", m.selected)
	}
	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "External expression unavailable") {
		t.Fatal("unavailable expression notice missing")
	}
	press(m, tea.KeyDown)
	if m.selected != "rule:child" {
		t.Fatal("collapsed subrule's descendants should stay hidden")
	}
	press(m, tea.KeyRight)
	press(m, tea.KeyDown)
	if m.selected != "rule:grandchild" {
		t.Fatal(m.selected)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Disabled by parent") {
		t.Fatal("effective state lost")
	}
}
func TestCombinedSearchAncestorsAndStaleMessages(t *testing.T) {
	m := demo(t)
	for i, value := range []string{"", "Operating system", "staff", "send", "ops@example.test"} {
		m.inputs[i].SetValue(value)
	}
	m.scheduleSearch()
	applyCurrentSearch(m)
	if !slices.Equal(m.matchIDs, []string{"office"}) {
		t.Fatalf("combined filters: %v", m.matchIDs)
	}
	if len(m.tree) != 2 {
		t.Fatalf("expected folder and direct match: %v", m.tree)
	}
	old := m.searchCommand()
	m.clearSearch()
	m.Update(old())
	if len(m.matchIDs) != 3 {
		t.Fatal("stale search replaced cleared filters")
	}
	m.inputs[0].SetValue(`"Nested disabled"`)
	m.scheduleSearch()
	applyCurrentSearch(m)
	if !slices.Equal(m.matchIDs, []string{"grandchild"}) {
		t.Fatalf("quoted text match: %v", m.matchIDs)
	}
	for _, key := range []string{"folder:folder", "rule:office", "rule:child", "rule:grandchild"} {
		if !m.visible[key] {
			t.Errorf("missing ancestor %s", key)
		}
	}
	press(m, 'n')
	if m.selected != "rule:grandchild" {
		t.Fatal("result jump selected context instead of direct match")
	}
	m.inputs[0].SetValue("no-such-policy-098765")
	m.scheduleSearch()
	applyCurrentSearch(m)
	if len(m.tree) != 0 || m.selected != "" {
		t.Fatal("empty results retained unrelated policy details")
	}
	m.clearSearch()
	if m.selected != "rule:office" {
		t.Fatal("clear search did not restore first displayed policy")
	}
}
func TestDiagnosticsPreserveLayoutAndInspectOwningFilter(t *testing.T) {
	m := demo(t)
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 32})
	before := len(m.detail)
	headings := append([]int{}, m.headings...)
	m.focusPane(detailFocus)
	filter := -1
	for i, line := range m.detail {
		if line.Fields != nil && line.Fields.Name == "FILTER" {
			filter = i
			break
		}
	}
	if filter < 0 {
		t.Fatal("missing filter context")
	}
	if text := ansi.Strip(m.detail[filter].Text); strings.Contains(text, "FILTER_ID") || strings.Contains(text, "Filter ID") {
		t.Fatal("filter ID is visible with diagnostics off")
	}
	m.detailCursor = filter
	m.clampDetail()
	offset := m.detailOffset
	press(m, 'd')
	if len(m.detail) != before || !slices.Equal(headings, m.headings) || m.detailOffset != offset {
		t.Fatal("diagnostics changed geometry or scroll")
	}
	if !strings.Contains(ansi.Strip(m.detail[filter].Text), `FILTER_ID="first"`) {
		t.Fatal("filter ID is missing from diagnostics")
	}
	for _, line := range m.detail {
		if line.Fields != nil && line.Fields.Name == "CONDITION" && strings.Contains(ansi.Strip(line.Text), "Operating system") {
			inspection := strings.Join(fieldLines(line.Fields, 10000), "\n")
			if strings.Index(inspection, "PLUGIN_NAME") > strings.Index(inspection, "FIELD_NAME") || !strings.Contains(ansi.Strip(line.Text), `PLUGIN_NAME="Inventory"`) {
				t.Fatal("plugin name is not prioritized in diagnostics")
			}
		}
	}
	press(m, tea.KeyEnter)
	if m.overlay != fieldsOverlay || m.inspected.Name != "FILTER" {
		t.Fatal("wrong inspector owner")
	}
	content := strings.Join(m.inspectLines, "\n")
	if !strings.Contains(content, "CASE_SENSITIVE") || !strings.Contains(content, `"false"`) {
		t.Fatal("complete exported filter values missing")
	}
	press(m, tea.KeyEscape)
	if m.overlay != noOverlay || m.detailCursor != filter {
		t.Fatal("inspector did not restore location")
	}
}
func TestPinsFailedLoadsAndReplacement(t *testing.T) {
	m := demo(t)
	oldDoc := m.doc
	press(m, 'p')
	if len(m.pins) != 1 || !m.tree[0].Pin {
		t.Fatal("pin shortcut missing")
	}
	m.loadSequence = 4
	m.Update(loadResult{sequence: 3, err: errors.New("stale")})
	if m.failed {
		t.Fatal("stale load error surfaced")
	}
	m.Update(loadResult{sequence: 4, err: errors.New("synthetic invalid XML")})
	if m.doc != oldDoc || len(m.pins) != 1 || !m.failed {
		t.Fatal("failed load changed current document")
	}
	m.segmentStates["old"] = &browseState{Filter: "old"}
	fresh, err := policy.LoadDemo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m.Update(loadResult{sequence: 4, doc: fresh, path: "synthetic.xml"})
	if m.doc != fresh || len(m.pins) != 0 || len(m.segmentStates) != 0 || m.failed {
		t.Fatal("document replacement retained stale state")
	}
}
func TestFilterTypingAndRawGroupIDs(t *testing.T) {
	m := demo(t)
	press(m, '/')
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if m.focus != inputFocus || m.input != 2 || !m.filterOpen {
		t.Fatal("/ then Tab did not reach the group filter")
	}
	press(m, 's')
	press(m, 't')
	press(m, 'a')
	press(m, 'f')
	press(m, 'f')
	press(m, tea.KeyEnter)
	applyCurrentSearch(m)
	if m.focus != treeFocus || m.inputs[2].Value() != "staff" || !slices.Equal(m.matchIDs, []string{"office"}) {
		t.Fatal("raw group ID was not applied")
	}
	press(m, 'r', tea.ModCtrl)
	if m.filtering() {
		t.Fatal("Ctrl+R did not clear filters")
	}
	press(m, '/')
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter)
	applyCurrentSearch(m)
	if m.inputs[2].Value() != "guest" || m.focus != treeFocus {
		t.Fatalf("group suggestion did not apply ID: %q", m.inputs[2].Value())
	}
}

func TestFiltersRemainReachableWithSuggestionsAndInShortTerminals(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 24}, {140, 42}} {
		m := demo(t)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		press(m, '/')
		for index := 1; index <= 5; index++ {
			press(m, tea.KeyTab)
			if m.focus != inputFocus || m.input != index%5 || !m.filterOpen {
				t.Fatalf("%v: Tab did not reach filter %d", size, index%5)
			}
			if m.inputs[m.input].Value() != "" {
				t.Fatal("Tab unexpectedly accepted a suggestion")
			}
			visible := false
			for _, part := range m.sidebarParts() {
				visible = visible || part.Input == m.input
			}
			if !visible {
				t.Fatalf("%v: active filter %d is hidden", size, m.input)
			}
		}
		press(m, tea.KeyTab, tea.ModShift)
		if m.input != 4 {
			t.Fatal("Shift+Tab did not reach the previous field")
		}
		press(m, tea.KeyEscape)
		if m.focus != treeFocus || m.filterOpen {
			t.Fatal("Escape did not close the filter editor")
		}
	}
}

func TestEnterActivatesAndNavigationAliasesDoNot(t *testing.T) {
	m := demo(t)
	selected := m.selected
	for _, key := range []rune{'i', 'b', 'g', ' ', 'f'} {
		press(m, key)
		if m.focus != treeFocus || m.selected != selected || m.overlay != noOverlay || m.filterOpen {
			t.Fatalf("removed alias %q still performs an action", key)
		}
	}
	press(m, tea.KeyEnter)
	if m.focus != detailFocus {
		t.Fatal("Enter did not open policy details")
	}
	press(m, tea.KeyRight)
	if m.overlay != noOverlay {
		t.Fatal("Right opened an inspector from a data row")
	}
	press(m, tea.KeyEnter)
	if m.overlay != fieldsOverlay {
		t.Fatal("Enter did not inspect the selected data row")
	}
}

func TestHorizontalVimKeysOnlyExpandAndCollapse(t *testing.T) {
	m := demo(t)
	press(m, 'h')
	if m.expanded["rule:office"] || m.focus != treeFocus {
		t.Fatal("h did not collapse the branch while keeping tree focus")
	}
	press(m, 'l')
	if !m.expanded["rule:office"] || m.focus != treeFocus {
		t.Fatal("l did not expand the branch while keeping tree focus")
	}
	m.reveal("rule:grandchild")
	m.selectKey("rule:grandchild")
	press(m, 'l')
	if m.focus != treeFocus || m.overlay != noOverlay || m.selected != "rule:grandchild" {
		t.Fatal("l on a leaf opened details instead of remaining in the tree")
	}
	press(m, 'l', tea.ModAlt)
	press(m, ']')
	section := m.detail[m.detailCursor].Section
	press(m, 'h')
	if m.sectionOpen[m.selected][section] || m.focus != detailFocus {
		t.Fatal("h did not collapse the detail section")
	}
	press(m, 'l')
	if !m.sectionOpen[m.selected][section] || m.overlay != noOverlay {
		t.Fatal("l opened an inspector instead of expanding the section")
	}
	press(m, tea.KeyHome)
	press(m, 'l')
	if m.overlay != noOverlay || m.detailCursor != 0 {
		t.Fatal("l activated a data row")
	}
	press(m, tea.KeyEnter)
	if m.overlay != fieldsOverlay {
		t.Fatal("Enter did not inspect the data row")
	}
}

func TestVimMovementRespectsPanesInspectorsAndEditors(t *testing.T) {
	m := demo(t)
	selected := m.selected
	press(m, 'j')
	if m.selected != "rule:child" || m.focus != treeFocus {
		t.Fatal("j did not move within the tree")
	}
	press(m, 'k')
	if m.selected != selected {
		t.Fatal("k did not return to the previous tree row")
	}
	for _, modifier := range []tea.KeyMod{tea.ModAlt, tea.ModCtrl, tea.ModShift} {
		press(m, 'j', modifier)
		press(m, 'k', modifier)
		if m.selected != selected || m.focus != treeFocus {
			t.Fatal("modified j/k clashed with pane navigation")
		}
	}
	press(m, 'l', tea.ModAlt)
	press(m, 'j')
	if m.detailCursor != 1 || m.selected != selected || m.focus != detailFocus {
		t.Fatal("j moved the tree while details had focus")
	}
	press(m, 'k')
	if m.detailCursor != 0 {
		t.Fatal("k did not move up within details")
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 12})
	press(m, tea.KeyEnter)
	press(m, 'j')
	if m.overlay != fieldsOverlay || m.overlayOffset != 1 || m.detailCursor != 0 {
		t.Fatal("j did not stay within the field inspector")
	}
	press(m, 'k')
	if m.overlayOffset != 0 {
		t.Fatal("k did not scroll the inspector up")
	}

	for input := 0; input < 5; input++ {
		m := demo(t)
		m.focusInput(input)
		press(m, 'j')
		press(m, 'k')
		press(m, 'h')
		press(m, 'l')
		if m.inputs[input].Value() != "jkhl" || m.focus != inputFocus || m.suggestion != -1 {
			t.Fatalf("Vim keys were intercepted in filter %d", input)
		}
	}
	m = segmented(t)
	m.openSegments()
	for _, view := range []overlay{segmentsOverlay, rangesOverlay} {
		if view == rangesOverlay {
			press(m, tea.KeyEnter)
		}
		press(m, 'j')
		if m.overlay != view || m.browseState().Cursor != 1 {
			t.Fatal("j did not move within the browser")
		}
		press(m, 'k')
		if m.browseState().Cursor != 0 {
			t.Fatal("k did not return to the previous browser row")
		}
	}
	press(m, '/')
	press(m, 'j')
	press(m, 'k')
	press(m, 'h')
	press(m, 'l')
	if !m.browseEditing || m.dialog.Value() != "jkhl" {
		t.Fatal("Vim keys were intercepted in the range filter")
	}
	m.openFile()
	m.dialog.SetValue("")
	press(m, 'j')
	press(m, 'k')
	press(m, 'h')
	press(m, 'l')
	if m.overlay != fileOverlay || m.dialog.Value() != "jkhl" {
		t.Fatal("Vim keys were intercepted in the file path editor")
	}
}

func segmented(t *testing.T) *Model {
	t.Helper()
	var xml strings.Builder
	xml.WriteString(`<POLICY_FOLDER ID="f" NAME="Synthetic"><RULE ID="p" NAME="Segments">`)
	for s := 0; s < 12; s++ {
		fmt.Fprintf(&xml, `<SEGMENT ID="s%d" NAME="Segment %d" EXTRA="metadata">`, s, s)
		for r := 0; r < 80; r++ {
			fmt.Fprintf(&xml, `<RANGE FROM="192.0.2.%d" TO="198.51.100.%d" ID="range-%d-%03d" EXTRA="retained"/>`, r, r, s, r)
		}
		xml.WriteString("</SEGMENT>")
	}
	xml.WriteString("</RULE></POLICY_FOLDER>")
	doc, err := policy.Load(context.Background(), strings.NewReader(xml.String()), "Synthetic segments")
	if err != nil {
		t.Fatal(err)
	}
	return New(doc)
}
func TestSegmentPagingFilteringAndStateRestoration(t *testing.T) {
	m := segmented(t)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 35})
	m.focusPane(detailFocus)
	for i, line := range m.detail {
		if line.Browse {
			m.detailCursor = i
			break
		}
	}
	press(m, tea.KeyEnter)
	if m.overlay != segmentsOverlay {
		t.Fatal("segment browser did not open")
	}
	lines := strings.Join(m.browseLines(), "\n")
	if strings.Contains(lines, "Segment 8 ") {
		t.Fatal("segment page rendered more than eight rows")
	}
	press(m, tea.KeyPgDown)
	if m.segmentState().Page != 1 {
		t.Fatal("segment paging failed")
	}
	press(m, tea.KeyPgUp)
	press(m, tea.KeyEnter)
	if m.overlay != rangesOverlay || m.segment != 0 {
		t.Fatal("range inspector did not open")
	}
	press(m, tea.KeyPgDown)
	press(m, tea.KeyDown)
	state := m.rangeState()
	if state.Page != 1 || state.Cursor != 26 {
		t.Fatal("range page navigation failed")
	}
	if strings.Contains(strings.Join(m.browseLines(), "\n"), "range-0-050") {
		t.Fatal("unbounded range page")
	}
	press(m, tea.KeyRight)
	if m.segment != 1 || m.rangeState().Cursor != 0 {
		t.Fatal("next segment did not retain independent state")
	}
	press(m, tea.KeyLeft)
	if m.segment != 0 || m.rangeState().Page != 1 || m.rangeState().Cursor != 26 {
		t.Fatal("segment position was not restored")
	}
	press(m, '/')
	m.dialog.SetValue("range-0-007")
	m.filterBrowse()
	press(m, tea.KeyEnter)
	if len(m.rangeState().Matches) != 1 || m.rangeState().Matches[0] != 7 {
		t.Fatal("range ID filter failed")
	}
	press(m, tea.KeyEnter)
	if m.overlay != fieldsOverlay || !strings.Contains(strings.Join(m.inspectLines, "\n"), "EXTRA") {
		t.Fatal("range context lost")
	}
	press(m, tea.KeyEscape)
	if m.overlay != rangesOverlay || m.rangeState().Filter != "range-0-007" {
		t.Fatal("range inspector state not restored")
	}
	press(m, tea.KeyEscape)
	if m.overlay != segmentsOverlay {
		t.Fatal("Esc did not return to segment overview")
	}
	press(m, tea.KeyEscape)
	if m.overlay != noOverlay {
		t.Fatal("Esc did not return to policy")
	}
}
func TestFramesFitTerminalAcrossPanesAndInspectors(t *testing.T) {
	m := segmented(t)
	for _, size := range [][2]int{{1, 1}, {29, 9}, {40, 12}, {77, 24}, {80, 24}, {120, 40}, {180, 50}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, pane := range []focus{treeFocus, detailFocus} {
			m.focusPane(pane)
			for _, inspector := range []overlay{noOverlay, helpOverlay, segmentsOverlay, rangesOverlay} {
				m.overlay = inspector
				frame := m.View().Content
				lines := strings.Split(frame, "\n")
				if len(lines) != size[1] {
					t.Fatalf("%v/%v: %d rows, want %d", size, inspector, len(lines), size[1])
				}
				for i, line := range lines {
					if width := ansi.StringWidth(line); width != size[0] {
						t.Fatalf("%v/%v row %d: %d columns", size, inspector, i, width)
					}
				}
			}
		}
	}
}
func TestMouseWheelDoesNotChangePolicy(t *testing.T) {
	m := demo(t)
	m.focusPane(detailFocus)
	selected := m.selected
	m.Update(tea.MouseWheelMsg{X: 90, Y: 14, Button: tea.MouseWheelDown})
	if m.selected != selected {
		t.Fatal("scroll changed policy")
	}
	m.focusPane(treeFocus)
	m.Update(tea.MouseClickMsg{X: 2, Y: 4, Button: tea.MouseLeft})
	if m.focus != inputFocus || m.input != 0 {
		t.Fatal("search click did not focus input")
	}
}
func TestTerminalControlsAreVisibleData(t *testing.T) {
	payload := "Policy \x1b]52;c;ZXZpbA==\a \u009b31m"
	value := safe(payload)
	if strings.ContainsAny(value, "\x1b\a\u009b") {
		t.Fatal("terminal control was emitted")
	}
	if !strings.Contains(value, "\\u001b") {
		t.Fatal("raw control no longer inspectable")
	}
}

func keyText(code rune, mod tea.KeyMod) string {
	if mod == 0 && unicode.IsPrint(code) {
		return string(code)
	}
	return ""
}

func TestWrappedSubruleRowsRemainNavigable(t *testing.T) {
	m := demo(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.doc.Nodes["rule:child"].Name = strings.Repeat("Long synthetic name ", 12)
	m.sectionOpen[m.selected]["subrules"] = true
	m.rebuildDetails()
	first := -1
	for i, line := range m.detail {
		if line.RuleKey == "rule:child" {
			first = i
			break
		}
	}
	if first < 0 || first+1 >= len(m.detail) || m.detail[first+1].RuleKey != "rule:child" {
		t.Fatal("wrapped rows lost their navigation target")
	}
	m.focusPane(detailFocus)
	m.detailCursor = first
	press(m, tea.KeyEnter)
	if m.selected != "rule:child" {
		t.Fatal("Enter on first wrapped line did not navigate")
	}
}

func TestExportedNotesAndGroupingStayInContext(t *testing.T) {
	m := demo(t)
	context := m.nodeFields(m.doc.Nodes["rule:child"])
	if !strings.Contains(strings.Join(context.Notes, "\n"), "external-expression:") {
		t.Fatal("export note not attached to its owning rule")
	}
	var condition *fields
	for _, line := range m.detail {
		if line.Fields != nil && line.Fields.Name == "CONDITION" {
			condition = line.Fields
			break
		}
	}
	if condition == nil {
		t.Fatal("missing condition context")
	}
	lines := strings.Join(fieldLines(condition, 100), "\n")
	if !strings.Contains(lines, "LEFT_PARENTHESIS") || !strings.Contains(lines, "LOGIC") {
		t.Fatal("grouping metadata lost")
	}
}

func TestPasteTriggersSearchInEveryEditor(t *testing.T) {
	m := demo(t)
	m.focusInput(0)
	m.Update(tea.PasteMsg{Content: "Nested disabled"})
	if !m.searchPending {
		t.Fatal("pasted query did not schedule a search")
	}
	applyCurrentSearch(m)
	if !slices.Equal(m.matchIDs, []string{"grandchild"}) {
		t.Fatalf("paste did not search: %v", m.matchIDs)
	}
	m = segmented(t)
	m.openSegments()
	m.startBrowseFilter()
	m.Update(tea.PasteMsg{Content: "seg-no-such-name"})
	if len(m.segmentState().Matches) != 0 {
		t.Fatal("pasted segment filter was not applied")
	}
}
