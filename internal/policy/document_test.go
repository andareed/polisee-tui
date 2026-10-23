package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andareed/polisee-lib"
)

func TestDemoUsesSharedCatalog(t *testing.T) {
	doc, err := LoadDemo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if doc.Policies != 1 || doc.Rules != 3 || doc.Folders != 1 {
		t.Fatalf("unexpected counts: %+v", doc)
	}
	child := doc.Nodes["rule:grandchild"].Rule
	if !child.Enabled || child.EffectiveEnabled {
		t.Fatal("declared/effective enabled states lost")
	}
	if got := doc.Engine.Search(polisee.Query{Property: "Operating system", Group: "staff", Action: "send", ActionValue: "ops@example.test"}); len(got) != 1 || got[0].RuleID != "office" {
		t.Fatalf("combined library query: %+v", got)
	}
	if got := doc.Nodes["rule:child"].Rule.ExternalExpressions; len(got) != 1 {
		t.Fatalf("external notice lost: %v", got)
	}
}
func TestBrowsingSortsFoldersAndPoliciesButKeepsSubrules(t *testing.T) {
	rule := func(id, name, parent string) polisee.Rule {
		return polisee.Rule{ID: id, Name: name, ParentID: parent, Enabled: true}
	}
	catalog := &polisee.Catalog{
		Folders: []polisee.Folder{{ID: "root", Name: "Root"}, {ID: "z", Name: "Folder 10", ParentID: "root"}, {ID: "a", Name: "Folder 2", ParentID: "root"}},
		Policies: []polisee.Policy{
			{ID: "ten", FolderID: "root", Rules: []polisee.Rule{rule("ten", "1.1.10", "")}},
			{ID: "two", FolderID: "root", Rules: []polisee.Rule{rule("two", "1.1.2", ""), rule("last", "Z first", "two"), rule("first", "A second", "two")}},
			{ID: "one", FolderID: "root", Rules: []polisee.Rule{rule("one", "1.1.01", "")}},
		},
	}
	doc, err := Describe(catalog, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"folder:a", "folder:z", "rule:one", "rule:two", "rule:ten"}; !slices.Equal(doc.Children["folder:root"], want) {
		t.Fatalf("folder contents: %v", doc.Children["folder:root"])
	}
	if want := []string{"rule:last", "rule:first"}; !slices.Equal(doc.Children["rule:two"], want) {
		t.Fatalf("subrule order: %v", doc.Children["rule:two"])
	}
	if want := []string{"one", "two", "last", "first", "ten"}; !slices.Equal(doc.RuleOrder, want) {
		t.Fatalf("result order: %v", doc.RuleOrder)
	}
	visible := doc.Visible([]string{"first"})
	for _, key := range []string{"folder:root", "rule:two", "rule:first"} {
		if !visible[key] {
			t.Errorf("ancestor %s not visible", key)
		}
	}
	if visible["rule:last"] {
		t.Fatal("unmatched sibling visible")
	}
	if !NaturalLess("Policy 999999999999999999999999", "Policy 1000000000000000000000000") {
		t.Fatal("numeric comparator overflow")
	}
}
func TestGroupSuggestionsDeduplicateNamesAndKeepDistinctIDs(t *testing.T) {
	xml := `<POLICY_FOLDER ID="f" NAME="Root"><RULE ID="p" NAME="Policy"><GROUP_IN_FILTER>
<GROUP NAME="Shared name" ID="a"/><GROUP NAME="Shared name" ID="a"/><GROUP NAME="Shared name" ID="b"/>
</GROUP_IN_FILTER><ACTION NAME="SET_GROUP"><PARAM NAME="group" VALUE="id:c;name:Other"/></ACTION></RULE></POLICY_FOLDER>`
	doc, err := Load(context.Background(), strings.NewReader(xml), "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	options := doc.Options[2]
	if len(options) != 3 {
		t.Fatalf("options: %+v", options)
	}
	for _, o := range options {
		if o.Value != o.ID {
			t.Fatalf("selection must disambiguate by ID: %+v", o)
		}
	}
	if name, id := Reference("id:x;name:Human name"); name != "Human name" || id != "x" {
		t.Fatal(name, id)
	}
	for _, raw := range []string{"id:x;name:", "id:x;unexpected:value", "opaque;value;more"} {
		if name, id := Reference(raw); name != raw || id != "" {
			t.Fatalf("unknown format changed: %q %q", name, id)
		}
	}
}
func TestLoadRejectsInvalidAndOversizedInput(t *testing.T) {
	if _, err := Load(context.Background(), strings.NewReader("<broken>"), "synthetic"); err == nil {
		t.Fatal("invalid XML accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadDemo(ctx); err == nil {
		t.Fatal("cancellation ignored")
	}
	dir := t.TempDir()
	if _, err := LoadFile(context.Background(), dir); err == nil {
		t.Fatal("directory accepted")
	}
	path := filepath.Join(dir, "large.xml")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxPolicyBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := LoadFile(context.Background(), path); err == nil {
		t.Fatal("oversized file accepted")
	}
}

func TestLargeSegmentsStayPagedAndComplete(t *testing.T) {
	var xml strings.Builder
	xml.WriteString(`<POLICY_FOLDER ID="f" NAME="Synthetic"><RULE ID="p" NAME="Range test">`)
	for s := 0; s < 24; s++ {
		fmt.Fprintf(&xml, `<SEGMENT NAME="Segment %d" ID="seg-%d" EXTRA="kept"><RANGES>`, s, s)
		for r := 0; r < 1200; r++ {
			fmt.Fprintf(&xml, `<RANGE FROM="192.0.2.%d" TO="198.51.100.%d" ID="range-%02d-%04d" EXTRA="literal-%d"/>`, r, r, s, r, r)
		}
		xml.WriteString("</RANGES></SEGMENT>")
	}
	xml.WriteString("</RULE></POLICY_FOLDER>")
	doc, err := Load(context.Background(), strings.NewReader(xml.String()), "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	segments := doc.Segments["p"]
	if len(segments) != 24 || len(segments[0].Ranges) != 1200 {
		t.Fatal("ranges lost")
	}
	if len(doc.Nodes["rule:p"].Rule.Scope) != 0 {
		t.Fatal("range tables included in normal detail rendering")
	}
	if strings.Contains(ElementText(segments[0].Metadata), "192.0.2.") {
		t.Fatal("range tables included in segment metadata")
	}
	if Attr(segments[0].Metadata.Attributes, "EXTRA") != "kept" {
		t.Fatal("segment metadata lost")
	}
	matches := segments[0].MatchingRanges("range-00-0017")
	if len(matches) != 1 || matches[0] != 17 {
		t.Fatalf("literal ID filtering: %v", matches)
	}
	if len(segments[0].MatchingRanges("192.0.2.0/24")) != 0 {
		t.Fatal("unexpected address containment inference")
	}
	page, start, end := PageBounds(900, 1200, RangePageSize)
	if page != 47 || start != 1175 || end != 1200 {
		t.Fatalf("bad page bounds: %d %d %d", page, start, end)
	}
	if len(doc.Engine.Search(polisee.Query{Text: "range-23-1199"})) != 1 {
		t.Fatal("complete catalog missing from library search")
	}
}

func TestPageBoundsEmptyAndNegative(t *testing.T) {
	for _, page := range []int{-5, 0, 9} {
		if p, a, b := PageBounds(page, 0, 25); p != 0 || a != 0 || b != 0 {
			t.Fatal(p, a, b)
		}
	}
}
