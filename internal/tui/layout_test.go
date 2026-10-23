package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-lib"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

// Multiple pretty-printed operands accumulate formatting-only character data
// on FILTER. It must stay inspectable without becoming blank rows in the UI.
func layoutModel(t *testing.T, width int) *Model {
	t.Helper()
	var xml strings.Builder
	xml.WriteString("<POLICY_FOLDER ID=\"f\" NAME=\"Synthetic layout\"><RULE ID=\"p\" NAME=\"Condition layout\">\n")
	xml.WriteString("  <EXPRESSION EXPR_TYPE=\"OR\">\n    <CONDITION FIELD_NAME=\"demo.category\" LABEL=\"Device category\">\n      <FILTER FILTER_ID=\"filter-a\" CASE_SENSITIVE=\"false\">\n")
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&xml, "        <VALUE VALUE=\"Example category/Branch %02d/Managed endpoints\"/>\n", i)
	}
	xml.WriteString("      </FILTER>\n    </CONDITION>\n    <CONDITION FIELD_NAME=\"in-group\" LABEL=\"Member of group\">\n      <FILTER FILTER_ID=\"filter-b\">\n        <GROUP ID=\"group-a\" NAME=\"Example lab devices\"/>\n      </FILTER>\n    </CONDITION>\n  </EXPRESSION>\n</RULE></POLICY_FOLDER>")
	doc, err := policy.Load(context.Background(), strings.NewReader(xml.String()), "Synthetic layout")
	if err != nil {
		t.Fatal(err)
	}
	m := New(doc)
	m.Update(tea.WindowSizeMsg{Width: width, Height: 60})
	return m
}

func TestPrettyPrintedConditionsHaveNoWhitespaceGaps(t *testing.T) {
	for _, width := range []int{40, 80, 120, 180, 240} {
		m := layoutModel(t, width)
		var filterRows []string
		var context *fields
		for _, line := range m.detail {
			if line.Fields != nil && line.Fields.Name == "FILTER" {
				body := strings.Trim(ansi.Strip(line.Text), " │")
				if body == "" {
					t.Fatalf("%d columns: formatting-only filter row became a blank line", width)
				}
				filterRows = append(filterRows, body)
				if context == nil {
					context = line.Fields
				}
			}
		}
		compact := strings.Join(strings.Fields(strings.Join(filterRows, " ")), " ")
		for _, value := range []string{"Branch 00", "Branch 11", "Example lab devices", "group-a"} {
			if !strings.Contains(compact, value) {
				t.Fatalf("%d columns: condition value %q lost in %q", width, value, compact)
			}
		}
		if strings.Contains(compact, "filter-a") || strings.Contains(compact, "filter-b") {
			t.Fatalf("%d columns: filter IDs are visible with diagnostics off", width)
		}
		if context == nil || !strings.Contains(context.Text, "\n") {
			t.Fatal("raw filter formatting was discarded")
		}
		if inspection := strings.Join(fieldLines(context, 10000), "\n"); !strings.Contains(inspection, strconv.Quote(context.Text)) {
			t.Fatal("exact filter text is no longer inspectable")
		}
		if !strings.Contains(strings.Join(fieldLines(context, 10000), "\n"), `FILTER_ID = "filter-a"`) {
			t.Fatal("filter ID is no longer inspectable")
		}
		original, ok := m.doc.Engine.Rule("p")
		if !ok || original.Expression.Children[0].Condition.Filters[0].Text != context.Text {
			t.Fatal("presentation changed library-owned values")
		}
	}
}

func TestSectionFramesCloseOnBothSides(t *testing.T) {
	for _, width := range []int{40, 80, 120, 180, 240} {
		m := layoutModel(t, width)
		open := false
		for i, line := range m.detail {
			text := ansi.Strip(line.Text)
			if line.Section != "" {
				if open {
					t.Fatalf("%d columns row %d: previous section never closed", width, i)
				}
				if !strings.HasPrefix(text, "╭") || !strings.HasSuffix(text, "╮") {
					t.Fatalf("%d columns: section heading lacks a complete frame", width)
				}
				open = true
			} else if open {
				if strings.HasPrefix(text, "╰") {
					if !strings.HasSuffix(text, "╯") {
						t.Fatalf("%d columns: bottom right corner missing", width)
					}
					open = false
				} else if !strings.HasPrefix(text, "│ ") || !strings.HasSuffix(text, " │") {
					t.Fatalf("%d columns row %d: section body lacks a right edge", width, i)
				}
			} else {
				continue
			}
			if got := ansi.StringWidth(line.Text); got != m.contentWidth() {
				t.Fatalf("%d columns row %d: box width %d, want %d", width, i, got, m.contentWidth())
			}
		}
		if open {
			t.Fatal("last section never closed")
		}
	}
}

func TestDeepConditionsKeepUsableWidthAndHangingIndent(t *testing.T) {
	leaf := polisee.Expression{Type: "SIMPLE", Condition: &polisee.Condition{
		Field: "demo.value", Label: "A long synthetic label that needs to wrap at narrow terminal widths",
		Filters: []polisee.Filter{{Attributes: []polisee.Attribute{{Name: "TYPE", Value: "equals"}, {Name: "FILTER_ID", Value: "synthetic-filter"}}, Operands: []polisee.Element{{Name: "VALUE", Text: strings.Repeat("endpoint", 40)}}}},
	}}
	for depth := 0; depth < 20; depth++ {
		leaf = polisee.Expression{Type: "AND", Children: []polisee.Expression{leaf}}
	}
	doc, err := policy.Describe(&polisee.Catalog{Folders: []polisee.Folder{{ID: "f", Name: "Root"}}, Policies: []polisee.Policy{{ID: "p", FolderID: "f", Rules: []polisee.Rule{{ID: "p", Name: "Deep nesting", Enabled: true, Expression: &leaf}}}}}, "Synthetic deep nesting")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{40, 80, 120, 180} {
		m := New(doc)
		m.Update(tea.WindowSizeMsg{Width: width, Height: 50})
		filterIndent := -1
		for _, line := range m.detail {
			if line.Fields == nil {
				continue
			}
			if line.Fields.Name == "EXPRESSION AND" {
				inside := strings.TrimPrefix(ansi.Strip(line.Text), "│ ")
				spaces := len(inside) - len(strings.TrimLeft(inside, " "))
				if spaces > 10 {
					t.Fatalf("%d columns: expression indented %d columns", width, spaces)
				}
			}
			if line.Fields.Name != "FILTER" {
				continue
			}
			body := strings.TrimSuffix(strings.TrimPrefix(ansi.Strip(line.Text), "│ "), " │")
			spaces := len(body) - len(strings.TrimLeft(body, " "))
			if filterIndent < 0 {
				at := strings.Index(body, "endpoint")
				if at < 0 {
					t.Fatal("filter value pushed off its first row")
				}
				filterIndent = at
			} else if strings.Contains(body, "endpoint") && spaces != filterIndent {
				t.Fatalf("%d columns: wrapped value starts at %d instead of %d", width, spaces, filterIndent)
			}
			if ansi.StringWidth(line.Text) != m.contentWidth() {
				t.Fatal("long word overflowed its box")
			}
		}
	}
}

func TestInlineSummariesKeepMeaningfulTextAndExactInspection(t *testing.T) {
	xml := "<POLICY_FOLDER ID=\"f\" NAME=\"Root\"><RULE ID=\"p\" NAME=\"Text values\"><EXPRESSION EXPR_TYPE=\"OR\"><CONDITION FIELD_NAME=\"demo.text\"><FILTER TYPE=\"equals\" FILTER_ID=\"text-filter\">\n  <VALUE>\n    first    value\n    continuation\n  </VALUE>\n</FILTER></CONDITION></EXPRESSION></RULE></POLICY_FOLDER>"
	doc, err := policy.Load(context.Background(), strings.NewReader(xml), "Synthetic multiline text")
	if err != nil {
		t.Fatal(err)
	}
	m := New(doc)
	var context *fields
	var rows []string
	for _, line := range m.detail {
		if line.Fields != nil && line.Fields.Name == "FILTER" {
			context = line.Fields
			rows = append(rows, ansi.Strip(line.Text))
		}
	}
	if context == nil || !strings.Contains(strings.Join(rows, "\n"), "first    value continuation") {
		t.Fatal("summary removed meaningful text or literal spaces")
	}
	if got := context.Children[0].Text; got != "\n    first    value\n    continuation\n  " {
		t.Fatalf("raw operand changed: %q", got)
	}
}

func TestLongFilterValuesUseTheReadingWidthWithDiagnosticsOff(t *testing.T) {
	m := demo(t)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	condition := m.doc.Nodes[m.selected].Rule.Expression.Children[0].Children[0].Condition
	condition.Filters = []polisee.Filter{{
		Attributes: []polisee.Attribute{{Name: "TYPE", Value: "equals"}, {Name: "CASE_SENSITIVE", Value: "false"}},
		Operands:   []polisee.Element{{Name: "VALUE", Text: strings.Repeat("x", 350)}},
	}}
	m.rebuildDetails()
	var rows []string
	for _, line := range m.detail {
		if line.Fields != nil && line.Fields.Name == "FILTER" {
			rows = append(rows, ansi.Strip(line.Text))
		}
	}
	if len(rows) < 2 {
		t.Fatal("long filter did not wrap")
	}
	first := strings.TrimSuffix(strings.TrimPrefix(rows[0], "│ "), " │")
	if strings.HasSuffix(first, " ") {
		t.Fatal("hidden diagnostics still reserve blank columns beside the value")
	}
	count := 0
	for _, row := range rows {
		count += strings.Count(row, "x")
	}
	// "equals" and the other demo filters contain no x.
	if count != 350 {
		t.Fatalf("long value was truncated: retained %d of 350 characters", count)
	}
	before := len(m.detail)
	m.diagnostics = true
	m.rebuildDetails()
	if len(m.detail) != before {
		t.Fatal("diagnostics changed wrapping")
	}
}
