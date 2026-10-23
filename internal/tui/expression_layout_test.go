package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-lib"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

func expressionModel(t *testing.T, width int) *Model {
	t.Helper()
	leaf := func(label string) polisee.Expression {
		value := label
		if label == "C" {
			value = strings.Repeat("scope", 50)
		}
		return polisee.Expression{Type: "SIMPLE", Condition: &polisee.Condition{Field: "demo." + label, Label: "Condition " + label,
			Filters: []polisee.Filter{{Attributes: []polisee.Attribute{{Name: "TYPE", Value: "equals"}, {Name: "FILTER_ID", Value: label + "-filter"}},
				Operands: []polisee.Element{{Name: "VALUE", Attributes: []polisee.Attribute{{Name: "VALUE", Value: value}}}}}}}}
	}
	group := func(operator string, children ...polisee.Expression) polisee.Expression {
		return polisee.Expression{Type: operator, Attributes: []polisee.Attribute{{Name: "EXPR_TYPE", Value: operator}}, Children: children}
	}
	expression := group("OR", group("PARENTHESIS", group("OR", leaf("A"), leaf("B"), group("PARENTHESIS", group("AND", leaf("C"), group("NOT", leaf("D")))))),
		leaf("E"), group("PARENTHESIS", group("AND", leaf("F"), group("NOT", leaf("G")))), leaf("H"))
	catalog := &polisee.Catalog{Folders: []polisee.Folder{{ID: "f", Name: "Synthetic"}}, Policies: []polisee.Policy{{ID: "p", FolderID: "f",
		Rules: []polisee.Rule{{ID: "p", Name: "Synthetic grouping", Enabled: true, Expression: &expression}}}}}
	doc, err := policy.Describe(catalog, "Synthetic grouping")
	if err != nil {
		t.Fatal(err)
	}
	m := New(doc)
	m.Update(tea.WindowSizeMsg{Width: width, Height: 60})
	return m
}

func TestSharedExpressionLayoutKeepsWrappedBodiesInsideTheirScope(t *testing.T) {
	for _, width := range []int{40, 80, 120, 180, 240} {
		m := expressionModel(t, width)
		before, _ := m.doc.Engine.Rule("p")
		m.rebuildDetails()
		contexts := map[*fields]bool{}
		var scopedRows int
		for _, line := range m.detail {
			if line.Fields == nil {
				continue
			}
			if line.Fields.Name == "CONDITION" {
				contexts[line.Fields] = true
			}
			if ansi.StringWidth(line.Text) != m.contentWidth() {
				t.Fatalf("%d columns: scope or condition overflows its section", width)
			}
			if line.Fields.Name == "FILTER" && policy.Attr(line.Fields.Attributes, "FILTER_ID") == "C-filter" {
				inside := strings.TrimPrefix(ansi.Strip(line.Text), "│ ")
				if !strings.HasPrefix(inside, "    │ ") {
					t.Fatalf("%d columns: wrapped condition lost enclosing bracket: %q", width, inside)
				}
				scopedRows++
			}
		}
		if len(contexts) != 8 || scopedRows < 2 {
			t.Fatalf("%d columns: got %d conditions and %d wrapped scoped rows", width, len(contexts), scopedRows)
		}
		after, _ := m.doc.Engine.Rule("p")
		if !reflect.DeepEqual(before, after) {
			t.Fatal("rendering changed the authoritative expression")
		}
	}
}

func TestTextConditionsRemainAvailableAndUseSharedInfix(t *testing.T) {
	m := expressionModel(t, 180)
	press(m, 'v')
	if !m.textMode {
		t.Fatal("text option is unavailable")
	}
	var lines []string
	for _, line := range m.detail {
		lines = append(lines, ansi.Strip(line.Text))
	}
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, `Condition A = "A"`) || !strings.Contains(text, `AND NOT Condition D = "D"`) || strings.Contains(text, "PARENTHESIS") {
		t.Fatal(text)
	}
	press(m, 'v')
	if m.textMode {
		t.Fatal("visual option is unavailable after switching to text")
	}
}
