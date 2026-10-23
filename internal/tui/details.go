package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/andareed/polisee-lib"
	"github.com/andareed/polisee-lib/fieldnames"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/charmbracelet/x/ansi"
)

type fields struct {
	Name       string
	Attributes []polisee.Attribute
	Text       string
	Children   []polisee.Element
	External   []string
	Notes      []string
}

type detailLine struct {
	TextStart, TextEnd int
	Text               string
	Fields             *fields
	Section            string
	RuleKey            string
	Browse             bool
}

func elementFields(n polisee.Element) *fields {
	return &fields{Name: n.Name, Attributes: n.Attributes, Text: n.Text, Children: n.Children, External: n.ExternalExpressions}
}
func ruleFields(n *policy.Node) *fields {
	if n == nil {
		return nil
	}
	if n.Folder != nil {
		return &fields{Name: "POLICY_FOLDER", Attributes: n.Folder.Attributes, Children: n.Folder.Extensions, Notes: []string{"Folder ID: " + n.ID}}
	}
	r := n.Rule
	return &fields{Name: "RULE", Attributes: r.Attributes, Children: r.Extensions, External: r.ExternalExpressions,
		Notes: []string{fmt.Sprintf("Declared enabled: %t (exported: %t)", r.Enabled, r.EnabledSpecified), fmt.Sprintf("Effective enabled: %t", r.EffectiveEnabled), "Parent rule ID: " + r.ParentID}}
}

func (m *Model) nodeFields(n *policy.Node) *fields {
	context := ruleFields(n)
	if context == nil {
		return nil
	}
	for _, note := range m.doc.Diagnostics {
		if note.RuleID == "" || (n.Rule != nil && note.RuleID == n.ID) {
			context.Notes = append(context.Notes, note.Code+": "+note.Message)
		}
	}
	return context
}

func (b *detailBuilder) link(text string, context *fields, key string, browse bool) {
	first := len(b.lines)
	style := normal
	if browse {
		style = accent
	}
	b.add(text, style, context, "")
	for i := first; i < len(b.lines); i++ {
		b.lines[i].RuleKey = key
		b.lines[i].Browse = browse
	}
}

type detailBuilder struct {
	model        *Model
	lines        []detailLine
	framed       bool
	width        int
	prefix       string
	continuation string
}

func (b *detailBuilder) innerWidth() int {
	if b.framed {
		return max(1, b.width-4)
	}
	return max(1, b.width)
}

func (b *detailBuilder) add(text string, style lipgloss.Style, context *fields, technical string) {
	b.row(0, "", text, style, context, technical)
}

// row separates structural indentation from content. Every continuation keeps
// the same value column; deep XML nesting cannot consume the reading width.
func (b *detailBuilder) row(indent int, lead, text string, style lipgloss.Style, context *fields, technical string) {
	width := b.innerWidth()
	prefix := terminalExpressionPrefix(b.prefix, width)
	continuation := terminalExpressionPrefix(b.continuation, width)
	prefixWidth := max(ansi.StringWidth(prefix), ansi.StringWidth(continuation))
	indent = min(max(0, indent), min(10, width/6))
	lead = one(lead)
	leadWidth := ansi.StringWidth(lead)
	if leadWidth > max(4, width/3) {
		text = lead + text
		lead, leadWidth = "", 0
	}
	available := max(1, width-prefixWidth-indent-leadWidth)
	wrapped := strings.Split(ansi.Wrap(safe(text), available, " "), "\n")
	for i, fragment := range wrapped {
		gutter := prefix
		if i > 0 {
			gutter = continuation
		}
		marker := lead
		if i > 0 {
			marker = strings.Repeat(" ", leadWidth)
		}
		primary := gutter + strings.Repeat(" ", indent) + marker + fragment
		painted := style.Render(primary)
		// Technical previews use leftover space; they never reduce the width of
		// the value or change wrapping when diagnostics is toggled.
		spare := width - ansi.StringWidth(primary)
		if i == 0 && technical != "" && b.model.diagnostics && spare >= 6 {
			preview := "⋯"
			if spare >= 14 {
				preview = one(technical)
			}
			painted += dim.Render("  " + ansi.Truncate(preview, spare-2, "…"))
		}
		textStart, textEnd := 0, ansi.StringWidth(painted)
		painted = fit(painted, width)
		if b.framed {
			painted = dim.Render("│ ") + painted + dim.Render(" │")
			textStart += 2
			textEnd += 2
		}
		b.lines = append(b.lines, detailLine{Text: painted, Fields: context, TextStart: textStart, TextEnd: textEnd})
	}
	b.prefix = b.continuation
}

// At unusually deep logical nesting, mark omitted ancestor columns explicitly
// and retain the nearest scope. Exported fields still contain the complete tree.
func terminalExpressionPrefix(prefix string, width int) string {
	runes := []rune(prefix)
	limit := max(0, min(24, width/3))
	if len(runes) <= limit {
		return prefix
	}
	if limit < 3 {
		return strings.Repeat(" ", limit)
	}
	return "… " + string(runes[len(runes)-(limit-2):])
}

func (b *detailBuilder) section(key, title, summary string, initial bool, body func()) {
	m := b.model
	state := m.sectionOpen[m.selected]
	if state == nil {
		state = map[string]bool{}
		m.sectionOpen[m.selected] = state
	}
	open, exists := state[key]
	if !exists {
		open = initial
		state[key] = open
	}
	arrow := "▸"
	if open {
		arrow = "▾"
	}
	index := len(b.lines)
	header := arrow + " " + title + " · " + summary
	textStart, textEnd := 0, min(b.width, ansi.StringWidth(header))
	framed := b.width >= 4
	if framed {
		textStart, textEnd = 3, min(b.width-1, 3+ansi.StringWidth(header))
		label := ansi.Truncate("─ "+header+" ", b.width-2, "…")
		label += strings.Repeat("─", max(0, b.width-2-ansi.StringWidth(label)))
		header = dim.Render("╭") + sectionStyle.Render(label) + dim.Render("╮")
	} else {
		header = sectionStyle.Render(fit(header, b.width))
	}
	b.lines = append(b.lines, detailLine{Text: header, Section: key, TextStart: textStart, TextEnd: textEnd})
	m.headings = append(m.headings, index)
	if open {
		b.framed = framed
		body()
		b.framed = false
	}
	if framed {
		b.lines = append(b.lines, detailLine{Text: dim.Render("╰" + strings.Repeat("─", b.width-2) + "╯")})
	}
	b.lines = append(b.lines, detailLine{})
}

func (b *detailBuilder) attributes(attributes []polisee.Attribute, context *fields) {
	if len(attributes) == 0 {
		return
	}
	b.section("attributes", "Attributes", countSummary(len(attributes), "attribute"), true, func() {
		names := make([]string, len(attributes))
		nameWidth := 0
		for i, attribute := range attributes {
			name := attribute.Name
			if attribute.Namespace != "" {
				name = "{" + attribute.Namespace + "}" + name
			}
			names[i] = one(name)
			nameWidth = max(nameWidth, ansi.StringWidth(names[i]))
		}
		for i, attribute := range attributes {
			if nameWidth+2 <= max(4, b.innerWidth()/3) {
				lead := names[i] + strings.Repeat(" ", nameWidth-ansi.StringWidth(names[i])+2)
				b.row(0, lead, shown(attribute.Value), normal, context, "")
			} else {
				b.add(names[i], muted, context, "")
				b.row(2, "", shown(attribute.Value), normal, context, "")
			}
		}
	})
}

func (m *Model) rebuildDetails() {
	m.headings = nil
	b := detailBuilder{model: m, width: m.contentWidth()}
	n := m.doc.Nodes[m.selected]
	if n == nil {
		b.add("Select a policy to inspect its definition.", muted, nil, "")
		m.detail = b.lines
		m.clampDetail()
		return
	}
	context := m.nodeFields(n)
	b.add(n.Name, strong, context, brief(context.Attributes, "NAME", "DESCRIPTION", "ID", "ENABLED"))
	if n.Folder != nil {
		b.add("Folder · ID "+n.ID, muted, context, "")
		b.add("", normal, nil, "")
		b.section("contents", "Folder contents", strconv.Itoa(len(m.doc.Children[n.Key])), true, func() {
			for _, key := range m.doc.Children[n.Key] {
				child := m.doc.Nodes[key]
				b.link(child.Name+"  · "+child.Kind, m.nodeFields(child), key, false)
			}
		})
		if len(n.Folder.Extensions) > 0 {
			b.section("extensions", "Additional exported fields", strconv.Itoa(len(n.Folder.Extensions)), false, func() {
				for _, e := range n.Folder.Extensions {
					b.element(e, 0)
				}
			})
		}
		m.detail = b.lines
		m.clampDetail()
		return
	}
	r := n.Rule
	state, style := "● Enabled", green
	if !r.EffectiveEnabled {
		state = "○ Disabled"
		style = muted
		if r.Enabled {
			state = "○ Disabled by parent"
		}
	}
	b.add(state+"   · ID "+r.ID, style, context, "")
	if r.Description != "" {
		b.add(r.Description, normal, context, "")
	}
	b.add("", normal, nil, "")
	segments := m.doc.Segments[r.ID]
	scopeCount := len(r.Scope) + len(segments)
	b.section("scope", "Scope & Filters", countSummary(scopeCount, "item"), scopeCount > 0, func() {
		if scopeCount == 0 {
			b.add("None exported", muted, nil, "")
		}
		for _, e := range r.Scope {
			b.element(e, 0)
		}
		if len(segments) > 0 {
			ranges := 0
			for _, s := range segments {
				ranges += len(s.Ranges)
			}
			b.link(fmt.Sprintf("Segments  %d · %d ranges   [Enter] browse", len(segments), ranges), nil, "", true)
		}
	})
	conditions := len(r.Expression.Conditions())
	b.section("conditions", "Conditions", countSummary(conditions, "condition"), true, func() {
		for _, path := range r.ExternalExpressions {
			b.add("! External expression unavailable: "+path, warning, context, "")
		}
		if r.Expression == nil {
			b.add("No condition expression exported.", muted, context, "")
		} else {
			b.expression(r.Expression)
		}
	})
	b.section("actions", "Actions", countSummary(len(r.Actions), "action"), len(r.Actions) > 0, func() {
		if len(r.Actions) == 0 {
			b.add("None exported", muted, nil, "")
		}
		for _, action := range r.Actions {
			label := policy.Label(action.Name)
			if action.Disabled {
				label += "  [Disabled]"
			} else {
				label += "  [Enabled]"
			}
			ctx := &fields{Name: "ACTION", Attributes: action.Attributes, Children: append([]polisee.Element{}, action.Extensions...)}
			for _, parameter := range action.Parameters {
				ctx.Children = append(ctx.Children, polisee.Element{Name: "PARAM", Attributes: parameter.Attributes, Text: parameter.Text, Children: parameter.Children})
			}
			var values, secondary []string
			// Readable group values lead; identifiers and empty settings follow.
			for priority := 0; priority < 3; priority++ {
				for _, p := range action.Parameters {
					key := strings.ToLower(strings.ReplaceAll(p.Name, "-", "_"))
					level := 1
					if key == "group" || key == "group_name" {
						level = 0
					}
					if policy.Identifier(p.Name) || key == "item_key" || (p.Value == "" && len(p.Children) == 0) {
						level = 2
					}
					if level != priority {
						continue
					}
					name, id := p.Value, ""
					if strings.Contains(key, "group") {
						name, id = policy.Reference(p.Value)
					}
					value := inlineSummary(name)
					if id != "" {
						value += " · ID " + id
					}
					for _, child := range p.Children {
						value = join(value, readable(child))
					}
					if value == "" {
						value = "Not set"
					}
					value = join(value, identifiers(p.Attributes))
					value = policy.Label(p.Name) + " " + value
					if level == 2 {
						secondary = append(secondary, value)
					} else {
						values = append(values, value)
					}
				}
			}
			if id := identifiers(action.Attributes); id != "" {
				secondary = append(secondary, id)
			}
			content := label
			if len(values) > 0 {
				content += "  · " + strings.Join(values, " · ")
			}
			b.add(content, normal, ctx, brief(action.Attributes, "NAME", "DISABLED"))
			if len(secondary) > 0 {
				b.add("  "+strings.Join(secondary, " · "), muted, ctx, "")
			}
			for _, schedule := range action.Schedules {
				b.add("  Schedule  "+readable(schedule), muted, elementFields(schedule), "")
			}
		}
	})
	configured := false
	for _, timer := range r.Timers {
		v := strings.ToLower(strings.TrimSpace(policy.Attr(timer.Attributes, "USE_DEFAULT")))
		if v != "true" && v != "1" && v != "yes" {
			configured = true
		}
	}
	b.section("timers", "Timers & Admissions", countSummary(len(r.Timers), "setting"), configured, func() {
		if len(r.Timers) == 0 {
			b.add("None exported", muted, nil, "")
		}
		for _, e := range r.Timers {
			b.element(e, 0)
		}
	})
	children := m.doc.Children[n.Key]
	if len(children) > 0 {
		b.section("subrules", "Sub Rules", countSummary(len(children), "rule")+" · source order", false, func() {
			for _, key := range children {
				child := m.doc.Nodes[key]
				state := "Enabled"
				if !child.Rule.EffectiveEnabled {
					state = "Disabled"
					if child.Rule.Enabled {
						state = "Disabled by parent"
					}
				}
				b.link(child.Name+"  · "+state+"  · ID "+child.ID, m.nodeFields(child), key, false)
			}
		})
	}
	if len(r.Extensions) > 0 {
		b.section("extensions", "Additional exported fields", countSummary(len(r.Extensions), "element"), false, func() {
			for _, e := range r.Extensions {
				b.element(e, 0)
			}
		})
	}
	b.attributes(r.Attributes, context)
	m.detail = b.lines
	m.clampDetail()
}

func (b *detailBuilder) element(n polisee.Element, depth int) {
	label := policy.Label(n.Name)
	b.row(depth*2, "", label+"  "+readable(n), normal, elementFields(n), brief(n.Attributes, "NAME", "VALUE", "VALUE2", "ID", "FROM", "TO", "ADDRESS", "PREFIX", "UNIT"))
	for _, path := range n.ExternalExpressions {
		b.row(depth*2, "", "! External expression unavailable: "+one(path), warning, elementFields(n), "")
	}
	// Summary formatting is separate from the owning element's exact fields.
}

func (b *detailBuilder) expression(e *polisee.Expression) {
	if b.model.textMode {
		b.add(e.Text(), normal, &fields{Name: "EXPRESSION " + e.Type, Attributes: e.Attributes,
			Children: []polisee.Element{expressionElement(e)}, External: e.ExternalExpressions}, "")
		return
	}
	for _, line := range e.Layout() {
		source := expressionAt(e, line.SourcePath)
		ctx := &fields{Name: "EXPRESSION " + source.Type, Attributes: source.Attributes, Children: append([]polisee.Element{}, source.Extensions...), External: source.ExternalExpressions}
		if source.Condition != nil {
			c := source.Condition
			ctx = &fields{Name: "CONDITION", Attributes: append(append([]polisee.Attribute{}, source.Attributes...), c.Attributes...), Children: append(append([]polisee.Element{}, source.Extensions...), c.Extensions...), External: source.ExternalExpressions}
		}
		var ids string
		for _, path := range line.ContextPaths {
			context := expressionAt(e, path)
			ctx.Children = append(ctx.Children, polisee.Element{Name: "EXPRESSION", Attributes: context.Attributes, Children: context.Extensions, ExternalExpressions: context.ExternalExpressions})
			ids += spaceIDs(context.Attributes)
		}
		b.prefix, b.continuation = line.Prefix, line.Continuation
		if line.Kind == "condition" {
			b.conditionExpression(source, ctx, ids)
		} else {
			b.row(0, "", line.Text+spaceIDs(source.Attributes)+ids, accent, ctx, brief(source.Attributes, "EXPR_TYPE", "ID"))
			if line.Kind == "expression" {
				for _, path := range source.ExternalExpressions {
					b.row(0, "", "! External expression unavailable: "+one(path), warning, ctx, "")
				}
				for _, extension := range source.Extensions {
					b.element(extension, 0)
				}
			}
		}
		b.prefix, b.continuation = "", ""
	}
}

func expressionAt(e *polisee.Expression, path []int) *polisee.Expression {
	for _, index := range path {
		e = &e.Children[index]
	}
	return e
}

func expressionElement(e *polisee.Expression) polisee.Element {
	result := polisee.Element{Name: "EXPRESSION", Attributes: e.Attributes, Children: append([]polisee.Element{}, e.Extensions...), ExternalExpressions: e.ExternalExpressions}
	if e.Condition != nil {
		c := e.Condition
		condition := polisee.Element{Name: "CONDITION", Attributes: c.Attributes, Children: append([]polisee.Element{}, c.Extensions...)}
		for _, filter := range c.Filters {
			condition.Children = append(condition.Children, polisee.Element{Name: "FILTER", Attributes: filter.Attributes, Text: filter.Text, Children: filter.Operands})
		}
		result.Children = append(result.Children, condition)
	}
	for i := range e.Children {
		result.Children = append(result.Children, expressionElement(&e.Children[i]))
	}
	return result
}

func (b *detailBuilder) conditionExpression(e *polisee.Expression, ctx *fields, contextIDs string) {
	indent := 0
	c := e.Condition
	label := inlineSummary(fieldnames.Display(c.Field, c.Label))
	if label == "" {
		label = "Condition"
	}
	b.row(indent, "", label+spaceIDs(c.Attributes)+spaceIDs(e.Attributes)+contextIDs, normal, ctx, brief(c.Attributes, "LABEL", "FIELD_NAME", "ID"))
	switch c.UnknownBehavior() {
	case polisee.UnknownTrue:
		b.row(indent+2, "", "Irresolvable criteria evaluate as true", muted, ctx, "")
	case polisee.UnknownFalse:
		b.row(indent+2, "", "Irresolvable criteria evaluate as false", muted, ctx, "")
	case polisee.UnknownNoOverride:
		b.row(indent+2, "", "Irresolvable criteria: no override", muted, ctx, "")
	case polisee.UnknownUnsupported:
		b.row(indent+2, "", "Irresolvable criteria: "+inlineSummary(c.UnknownResult), warning, ctx, "")
	}
	for _, f := range c.Filters {
		operator := inlineSummary(policy.Attr(f.Attributes, "TYPE"))
		if operator == "" {
			operator = "filter"
		}
		// Direct FILTER character data includes indentation before and after
		// every operand. It is presentation whitespace, not an operand.
		value := inlineSummary(f.Text)
		for _, operand := range f.Operands {
			value = join(value, readable(operand))
		}
		for _, a := range f.Attributes {
			if a.Namespace == "" && (a.Name == "VALUE" || a.Name == "VALUE2") {
				value = join(value, summaryValue(a.Value))
			}
		}
		fctx := &fields{Name: "FILTER", Attributes: f.Attributes, Text: f.Text, Children: f.Operands}
		b.row(indent, operator+"  ", value, normal, fctx, brief(f.Attributes, "TYPE", "VALUE", "VALUE2"))
	}
	for _, path := range e.ExternalExpressions {
		b.row(indent, "", "! External expression unavailable: "+one(path), warning, ctx, "")
	}
	for _, extension := range e.Extensions {
		b.element(extension, 1)
	}
	if e.Condition != nil {
		for _, extension := range e.Condition.Extensions {
			b.element(extension, 1)
		}
	}
}

// Inline summaries discard XML line indentation, without collapsing literal
// spaces inside a value's line. Exact text and attributes remain in fields.
func inlineSummary(value string) string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " ")
}

func summaryValue(value string) string {
	if value == "" {
		return "\"\""
	}
	if compact := inlineSummary(value); compact != "" {
		return compact
	}
	return strconv.Quote(value)
}

func readable(n polisee.Element) string {
	var values []string
	used := map[string]bool{}
	if policy.HasAttr(n.Attributes, "NAME") {
		name := summaryValue(policy.Attr(n.Attributes, "NAME"))
		if strings.EqualFold(n.Name, "EXCEPTION") {
			name = fieldnames.Display(policy.Attr(n.Attributes, "NAME"), "")
		}
		values = append(values, name)
		used["NAME"] = true
	}
	kind := strings.ToUpper(n.Name)
	if kind == "SUBNET" && policy.HasAttr(n.Attributes, "ADDRESS") {
		value := summaryValue(policy.Attr(n.Attributes, "ADDRESS"))
		if policy.HasAttr(n.Attributes, "PREFIX") {
			value += "/" + policy.Attr(n.Attributes, "PREFIX")
			used["PREFIX"] = true
		}
		values = append(values, value)
		used["ADDRESS"] = true
	}
	if kind == "RANGE" && policy.HasAttr(n.Attributes, "FROM") && policy.HasAttr(n.Attributes, "TO") {
		values = append(values, summaryValue(policy.Attr(n.Attributes, "FROM"))+" – "+summaryValue(policy.Attr(n.Attributes, "TO")))
		used["FROM"] = true
		used["TO"] = true
	}
	for _, a := range n.Attributes {
		key := strings.ToUpper(a.Name)
		if used[key] || a.Namespace != "" || policy.Identifier(a.Name) {
			continue
		}
		value := summaryValue(a.Value)
		if key == "FIELD_NAME" || key == "FILED_NAME" || key == "COMP_FIELD_NAME" {
			value = fieldnames.Display(a.Value, "")
		}
		if key != "VALUE" && key != "VALUE2" {
			value = policy.Label(a.Name) + " " + value
		}
		values = append(values, value)
	}
	if strings.TrimSpace(n.Text) != "" {
		values = append(values, inlineSummary(n.Text))
	}
	for _, child := range n.Children {
		value := readable(child)
		if !(kind == "GROUP_IN_FILTER" && child.Name == "GROUP") {
			value = policy.Label(child.Name) + " " + value
		}
		values = append(values, value)
	}
	if ids := identifiers(n.Attributes); ids != "" {
		values = append(values, ids)
	}
	if len(values) == 0 {
		return "No value exported"
	}
	return strings.Join(values, " · ")
}

func identifiers(attrs []polisee.Attribute) string {
	var parts []string
	for _, a := range attrs {
		if policy.Identifier(a.Name) && !strings.EqualFold(a.Name, "FILTER_ID") {
			parts = append(parts, policy.Label(a.Name)+" "+summaryValue(a.Value))
		}
	}
	return strings.Join(parts, " · ")
}
func spaceIDs(attrs []polisee.Attribute) string {
	value := identifiers(attrs)
	if value != "" {
		return "  · " + value
	}
	return ""
}
func diagnosticAttributes(attrs []polisee.Attribute) []polisee.Attribute {
	var plugin, other []polisee.Attribute
	for _, attribute := range attrs {
		if attribute.Namespace == "" && strings.EqualFold(attribute.Name, "PLUGIN_NAME") {
			plugin = append(plugin, attribute)
		} else {
			other = append(other, attribute)
		}
	}
	return append(plugin, other...)
}

func brief(attrs []polisee.Attribute, omit ...string) string {
	var parts []string
	for _, a := range diagnosticAttributes(attrs) {
		skip := false
		for _, name := range omit {
			if strings.EqualFold(a.Name, name) {
				skip = true
			}
		}
		if !skip {
			parts = append(parts, a.Name+"="+strconv.Quote(a.Value))
		}
	}
	return strings.Join(parts, "  ")
}
func join(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " · " + b
}
func countSummary(n int, name string) string {
	if n != 1 {
		name += "s"
	}
	if n == 0 {
		return "None exported"
	}
	return fmt.Sprintf("%d %s", n, name)
}
