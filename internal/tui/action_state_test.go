package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

func TestActionStatusDoesNotInheritRuleStatus(t *testing.T) {
	const actions = `<ACTION NAME="DEFAULT_ACTION"/><ACTION NAME="ENABLED_ACTION" DISABLED="false"/><ACTION NAME="DISABLED_ACTION" DISABLED="true"/>`
	xml := `<POLICY_FOLDER ID="f"><RULE ID="off" NAME="Disabled rule" ENABLED="false">` + actions +
		`<INNER_RULE ID="inherited" NAME="Disabled by parent" ENABLED="true">` + actions + `</INNER_RULE></RULE>` +
		`<RULE ID="on" NAME="Enabled rule" ENABLED="true">` + actions + `</RULE></POLICY_FOLDER>`
	doc, err := policy.Load(context.Background(), strings.NewReader(xml), "Synthetic action states")
	if err != nil {
		t.Fatal(err)
	}
	m := New(doc)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 60})
	for _, id := range []string{"off", "inherited", "on"} {
		for _, diagnostics := range []bool{false, true} {
			m.selectKey("rule:" + id)
			m.diagnostics = diagnostics
			m.rebuildDetails()
			rows := map[string]string{}
			for _, line := range m.detail {
				if line.Fields != nil && line.Fields.Name == "ACTION" {
					name := policy.Attr(line.Fields.Attributes, "NAME")
					rows[name] += ansi.Strip(line.Text)
				}
			}
			for name, status := range map[string]string{"DEFAULT_ACTION": "[Enabled]", "ENABLED_ACTION": "[Enabled]", "DISABLED_ACTION": "[Disabled]"} {
				if !strings.Contains(rows[name], status) || strings.Contains(rows[name], "Inactive") {
					t.Fatalf("rule %s, diagnostics %t: action %s has wrong own status: %q", id, diagnostics, name, rows[name])
				}
			}
		}
	}
}
