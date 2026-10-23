package policy

import (
	"strconv"
	"strings"

	"github.com/andareed/polisee-lib"
)

const SegmentPageSize = 8
const RangePageSize = 25

type Segment struct {
	Key, Name, ID string
	Metadata      polisee.Element
	Ranges        []polisee.Element
	search        []string
}

// SplitSegments builds a literal inspection index, retaining every range field.
// It makes no address containment or policy evaluation decisions.
func SplitSegments(scope []polisee.Element) ([]polisee.Element, []Segment) {
	var segments []Segment
	var strip func([]polisee.Element) []polisee.Element
	strip = func(nodes []polisee.Element) []polisee.Element {
		var kept []polisee.Element
		for _, node := range nodes {
			if strings.EqualFold(node.Name, "SEGMENT") {
				entry := Segment{Key: strconv.Itoa(len(segments)), Name: Attr(node.Attributes, "NAME"), ID: Attr(node.Attributes, "ID")}
				var collect func(polisee.Element) polisee.Element
				collect = func(n polisee.Element) polisee.Element {
					copy := n
					copy.Children = nil
					for _, child := range n.Children {
						if strings.EqualFold(child.Name, "RANGE") {
							entry.Ranges = append(entry.Ranges, child)
							entry.search = append(entry.search, strings.ToLower(ElementText(child)))
						} else {
							copy.Children = append(copy.Children, collect(child))
						}
					}
					return copy
				}
				entry.Metadata = collect(node)
				segments = append(segments, entry)
			} else {
				node.Children = strip(node.Children)
				kept = append(kept, node)
			}
		}
		return kept
	}
	kept := strip(scope)
	return kept, segments
}

func (s Segment) MatchingRanges(text string) []int {
	text = strings.ToLower(strings.TrimSpace(text))
	var result []int
	for i, value := range s.search {
		if text == "" || strings.Contains(value, text) {
			result = append(result, i)
		}
	}
	return result
}

func MatchingSegments(segments []Segment, text string) []int {
	text = strings.ToLower(strings.TrimSpace(text))
	var result []int
	for i, s := range segments {
		if text == "" || strings.Contains(strings.ToLower(s.Name+" "+s.ID), text) {
			result = append(result, i)
		}
	}
	return result
}

func PageBounds(page, count, size int) (int, int, int) {
	if size < 1 {
		size = 1
	}
	last := max(0, (count-1)/size)
	page = max(0, min(page, last))
	start := page * size
	return page, start, min(start+size, count)
}

func ElementText(n polisee.Element) string {
	var out strings.Builder
	var visit func(polisee.Element)
	visit = func(node polisee.Element) {
		out.WriteString(node.Name)
		out.WriteByte(' ')
		for _, a := range node.Attributes {
			out.WriteString(a.Name)
			out.WriteByte('=')
			out.WriteString(a.Value)
			out.WriteByte(' ')
		}
		out.WriteString(node.Text)
		out.WriteByte(' ')
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(n)
	return out.String()
}
