// Package policy adapts polisee-lib for terminal presentation. It does not
// parse XML or evaluate policy criteria.
package policy

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/andareed/polisee-lib"
	"github.com/andareed/polisee-lib/fieldnames"
)

const MaxPolicyBytes = 64 << 20

// DemoXML is the reviewed synthetic fixture shared with the GUI and library.
//
//go:embed demo.xml
var DemoXML string

type Node struct {
	Key, Parent, ID, Name, Kind string
	Rule                        *polisee.Rule
	Folder                      *polisee.Folder
}

type Option struct{ Name, ID, Value string }

type Document struct {
	Name        string
	Engine      *polisee.Engine
	Nodes       map[string]*Node
	Children    map[string][]string
	RuleOrder   []string
	InitialID   string
	Options     [5][]Option
	Segments    map[string][]Segment
	Diagnostics []polisee.Diagnostic
	Policies    int
	Rules       int
	Folders     int
}

func LoadDemo(ctx context.Context) (*Document, error) {
	return Load(ctx, strings.NewReader(DemoXML), "Demonstration policies")
}

func LoadFile(ctx context.Context, path string) (*Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("choose a regular policy XML file")
	}
	if info.Size() > MaxPolicyBytes {
		return nil, fmt.Errorf("policy export exceeds 64 MiB")
	}
	return Load(ctx, file, filepath.Base(path))
}

func Load(ctx context.Context, reader io.Reader, name string) (*Document, error) {
	// Bound reads too: a regular file may grow after Stat.
	data, err := io.ReadAll(io.LimitReader(reader, MaxPolicyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPolicyBytes {
		return nil, fmt.Errorf("policy export exceeds 64 MiB")
	}
	catalog, err := polisee.Parse(ctx, strings.NewReader(string(data)), name)
	if err != nil {
		return nil, err
	}
	return Describe(catalog, name)
}

func Describe(catalog *polisee.Catalog, name string) (*Document, error) {
	engine, err := polisee.NewEngine(catalog)
	if err != nil {
		return nil, err
	}
	snapshot := engine.Snapshot()
	doc := &Document{
		Name: name, Engine: engine, Nodes: map[string]*Node{},
		Children: map[string][]string{}, Segments: map[string][]Segment{},
		Policies: len(snapshot.Policies), Folders: len(snapshot.Folders),
		Diagnostics: snapshot.Diagnostics,
	}
	sets := [5]map[string]Option{}
	for i := range sets {
		sets[i] = map[string]Option{}
	}
	add := func(field int, name, id string) {
		if strings.TrimSpace(name) == "" {
			return
		}
		value := name
		if field == 2 && id != "" {
			value = id
		}
		sets[field][name+"\x00"+id] = Option{Name: name, ID: id, Value: value}
	}
	var elements func([]polisee.Element)
	elements = func(nodes []polisee.Element) {
		for _, node := range nodes {
			if strings.EqualFold(node.Name, "GROUP") {
				add(2, Attr(node.Attributes, "NAME"), Attr(node.Attributes, "ID"))
			}
			for _, a := range node.Attributes {
				if a.Namespace == "" && (a.Name == "FIELD_NAME" || a.Name == "FILED_NAME" || a.Name == "COMP_FIELD_NAME") {
					add(1, a.Value, "")
					add(1, fieldnames.Label(a.Value, ""), "")
				}
			}
			elements(node.Children)
		}
	}
	for i := range snapshot.Folders {
		folder := &snapshot.Folders[i]
		parent := ""
		if folder.ParentID != "" {
			parent = "folder:" + folder.ParentID
		}
		doc.add(&Node{Key: "folder:" + folder.ID, Parent: parent, ID: folder.ID,
			Name: fallback(folder.Name, "Unnamed folder"), Kind: "folder", Folder: folder})
	}
	for pi := range snapshot.Policies {
		p := &snapshot.Policies[pi]
		for ri := range p.Rules {
			rule := &p.Rules[ri]
			parent, kind := "rule:"+rule.ParentID, "subpolicy"
			if rule.ParentID == "" {
				parent, kind = "folder:"+p.FolderID, "policy"
			}
			doc.add(&Node{Key: "rule:" + rule.ID, Parent: parent, ID: rule.ID,
				Name: fallback(rule.Name, "Unnamed policy"), Kind: kind, Rule: rule})
			doc.Rules++
			if doc.InitialID == "" {
				doc.InitialID = rule.ID
			}
			// The engine keeps the complete source catalog; only the presentation
			// snapshot removes bulky range tables from normal detail rendering.
			rule.Scope, doc.Segments[rule.ID] = SplitSegments(rule.Scope)
			for _, c := range rule.Expression.Conditions() {
				add(1, c.Field, "")
				add(1, fieldnames.Label(c.Field, c.Label), "")
				for _, f := range c.Filters {
					elements(f.Operands)
				}
			}
			elements(rule.Scope)
			for _, a := range rule.Actions {
				add(3, a.Name, "")
				for _, p := range a.Parameters {
					add(4, p.Value, "")
					if strings.Contains(strings.ToLower(p.Name), "group") {
						name, id := Reference(p.Value)
						if id != "" {
							add(2, name, id)
						} else if strings.EqualFold(strings.ReplaceAll(p.Name, "_", "-"), "group-name") {
							add(2, p.Value, "")
						}
					}
					elements(p.Children)
					var values func([]polisee.Element)
					values = func(nodes []polisee.Element) {
						for _, n := range nodes {
							for _, a := range n.Attributes {
								if a.Name == "NAME" || a.Name == "VALUE" || a.Name == "VALUE2" {
									add(4, a.Value, "")
								}
							}
							if strings.TrimSpace(n.Text) != "" {
								add(4, n.Text, "")
							}
							values(n.Children)
						}
					}
					values(p.Children)
				}
			}
		}
	}
	for parent, siblings := range doc.Children {
		if parent != "" && doc.Nodes[parent].Kind != "folder" {
			continue
		}
		sort.SliceStable(siblings, func(i, j int) bool {
			a, b := doc.Nodes[siblings[i]], doc.Nodes[siblings[j]]
			if (a.Kind == "folder") != (b.Kind == "folder") {
				return a.Kind == "folder"
			}
			return NaturalLess(a.Name, b.Name)
		})
	}
	var visit func(string)
	visit = func(parent string) {
		for _, key := range doc.Children[parent] {
			if doc.Nodes[key].Rule != nil {
				doc.RuleOrder = append(doc.RuleOrder, doc.Nodes[key].ID)
			}
			visit(key)
		}
	}
	visit("")
	for i, set := range sets {
		for _, option := range set {
			doc.Options[i] = append(doc.Options[i], option)
		}
		sort.Slice(doc.Options[i], func(a, b int) bool {
			x, y := doc.Options[i][a], doc.Options[i][b]
			if strings.EqualFold(x.Name, y.Name) {
				return x.ID < y.ID
			}
			return NaturalLess(x.Name, y.Name)
		})
	}
	return doc, nil
}

func (d *Document) add(n *Node) {
	d.Nodes[n.Key] = n
	d.Children[n.Parent] = append(d.Children[n.Parent], n.Key)
}

func (d *Document) Visible(ids []string) map[string]bool {
	visible := map[string]bool{}
	for _, id := range ids {
		for key := "rule:" + id; key != ""; {
			node := d.Nodes[key]
			if node == nil || visible[key] {
				break
			}
			visible[key] = true
			key = node.Parent
		}
	}
	return visible
}

func Attr(attrs []polisee.Attribute, name string) string {
	for _, a := range attrs {
		if a.Namespace == "" && strings.EqualFold(a.Name, name) {
			return a.Value
		}
	}
	return ""
}

func HasAttr(attrs []polisee.Attribute, name string) bool {
	for _, a := range attrs {
		if a.Namespace == "" && strings.EqualFold(a.Name, name) {
			return true
		}
	}
	return false
}

func Identifier(name string) bool {
	key := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	for _, suffix := range []string{"id", "guid", "uuid"} {
		if key == suffix || strings.HasSuffix(key, "_"+suffix) {
			return true
		}
	}
	return false
}

func Label(value string) string {
	labels := map[string]string{
		"GROUP_IN_FILTER": "Groups", "SUBNET": "Subnet", "RANGE": "Address range",
		"INACTIVITY_TTL": "Inactivity timeout", "ADMISSION_RESOLVE_DELAY": "Admission resolve delay",
		"MATCH_TIMING": "When matched", "UNMATCH_TIMING": "When unmatched", "USE_DEFAULT": "Use defaults",
		"START_CLASS": "Start", "OCCURENCE": "Occurrence", "ONSTART": "On start",
	}
	if label := labels[strings.ToUpper(value)]; label != "" {
		return label
	}
	words := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' || unicode.IsSpace(r) })
	for i, w := range words {
		words[i] = strings.ToLower(w)
		if strings.EqualFold(w, "ID") || strings.EqualFold(w, "GUID") || strings.EqualFold(w, "UUID") {
			words[i] = strings.ToUpper(w)
		}
	}
	runes := []rune(strings.Join(words, " "))
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

func Reference(value string) (string, string) {
	parts := strings.Split(value, ";")
	if len(parts) != 2 {
		return value, ""
	}
	var name, id string
	for _, p := range parts {
		key, text, ok := strings.Cut(p, ":")
		if !ok {
			return value, ""
		}
		switch strings.ToLower(key) {
		case "id":
			id = text
		case "name":
			name = text
		default:
			return value, ""
		}
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(id) == "" {
		return value, ""
	}
	return name, id
}

// NaturalLess compares digit runs without machine-integer limits. Stable ties
// preserve source order; subrules never pass through this browsing comparator.
func NaturalLess(left, right string) bool {
	a, b := []rune(strings.ToLower(left)), []rune(strings.ToLower(right))
	for i, j := 0, 0; ; {
		if i == len(a) || j == len(b) {
			return i == len(a) && j < len(b)
		}
		if a[i] >= '0' && a[i] <= '9' && b[j] >= '0' && b[j] <= '9' {
			x, y := i, j
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				i++
			}
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				j++
			}
			na, nb := strings.TrimLeft(string(a[x:i]), "0"), strings.TrimLeft(string(b[y:j]), "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
}

func fallback(value, otherwise string) string {
	if value == "" {
		return otherwise
	}
	return value
}
