package brigodier

import (
	"bytes"
	"context"
)

// AllUsage gets all possible executable commands following the given node.
//
// You may use Dispatcher.Root as a target to get all usage data for the entire command tree.
//
// The returned syntax will be in "simple" form: <param> and literal.
// "Optional" nodes will be listed as multiple entries: the parent node, and the child nodes.
// For example, a required literal "foo" followed by an optional param "int" will be two nodes:
//  foo
//  foo <int>
//
// The path to the specified node will NOT be prepended to the output, as there can theoretically be many
// ways to reach a given node. It will only give you paths relative to the specified node, not absolute from root.
func (d *Dispatcher) AllUsage(ctx context.Context, node CommandNode, restricted bool) []string {
	return d.allUsage(ctx, node, nil, "", restricted)
}
func (d *Dispatcher) allUsage(ctx context.Context, node CommandNode, result []string, prefix string, restricted bool) []string {
	if restricted && !node.CanUse(ctx) {
		return result
	}
	if node.Command() != nil {
		result = append(result, prefix)
	}
	b := new(bytes.Buffer)
	if node.Redirect() != nil {
		if prefix == "" {
			b.WriteString(node.UsageText())
		} else {
			b.WriteString(prefix)
		}
		b.WriteRune(ArgumentSeparator)
		if node.Redirect() == &d.Root {
			b.WriteString("...")
		} else {
			b.WriteString("-> ")
			b.WriteString(node.Redirect().UsageText())
		}
		result = append(result, b.String())
	} else { // if len(node.Children()) != 0
		node.ChildrenOrdered().Range(func(_ string, child CommandNode) bool {
			b.Reset()
			if prefix != "" {
				b.WriteString(prefix)
				b.WriteRune(ArgumentSeparator)
			}
			b.WriteString(child.UsageText())
			result = d.allUsage(ctx, child, result, b.String(), restricted)
			return true
		})
	}
	return result
}

const (
	// UsageOptionalOpen is the open rune for an optional argument.
	UsageOptionalOpen rune = '['
	// UsageOptionalClose is the close rune for an optional argument.
	UsageOptionalClose rune = ']'
	// UsageRequiredOpen is the open rune for a required argument.
	UsageRequiredOpen rune = '('
	// UsageRequiredClose is the close rune for a required argument.
	UsageRequiredClose rune = ')'
	// UsageOr is the or rune splitting multiple argument options.
	UsageOr rune = '|'
)

// SmartUsage gets the possible executable commands from a specified node.
//
// You may use Dispatcher.Root as a target to get usage data for the entire command tree.
//
// The returned syntax will be in "smart" form: <param>, literal, [optional] and (either|or).
// These forms may be mixed and matched to provide as much information about the child nodes as it can, without being too verbose.
// For example, a required literal "foo" followed by an optional param "int" can be compressed into one string:
//  foo [<int>]
//
// The path to the specified node will NOT be prepended to the output, as there can theoretically be many
// ways to reach a given node. It will only give you paths relative to the specified node, not absolute from root.
//
// The returned usage will be restricted to only commands that the provided context.Context can use.
func (d *Dispatcher) SmartUsage(ctx context.Context, node CommandNode) CommandNodeStringMap {
	result := NewCommandNodeStringMap()
	optional := node.Command() != nil
	node.ChildrenOrdered().Range(func(_ string, child CommandNode) bool {
		usage := d.smartUsage(ctx, child, optional, false)
		if usage != "" {
			result.Put(child, usage)
		}
		return true
	})
	return result
}
func (d *Dispatcher) smartUsage(ctx context.Context, node CommandNode, optional bool, deep bool) string {
	if !node.CanUse(ctx) {
		return ""
	}

	b := new(bytes.Buffer) // self
	if optional {
		b.WriteRune(UsageOptionalOpen)
		b.WriteString(node.UsageText())
		b.WriteRune(UsageOptionalClose)
	} else {
		b.WriteString(node.UsageText())
	}
	if deep {
		return b.String()
	}

	var openChar, closeChar rune
	childOptional := node.Command() != nil
	if childOptional {
		openChar = UsageOptionalOpen
		closeChar = UsageOptionalClose
	} else {
		openChar = UsageRequiredOpen
		closeChar = UsageRequiredClose
	}

	if node.Redirect() != nil {
		b.WriteRune(ArgumentSeparator)
		if node.Redirect() == &d.Root {
			b.WriteString("...")
		} else {
			b.WriteString("-> ")
			b.WriteString(node.Redirect().UsageText())
		}
		return b.String()
	}

	var children []CommandNode
	node.ChildrenOrdered().Range(func(_ string, child CommandNode) bool {
		if child.CanUse(ctx) {
			children = append(children, child)
		}
		return true
	})
	if len(children) == 1 {
		usage := d.smartUsage(ctx, children[0], childOptional, childOptional)
		if usage != "" {
			b.WriteRune(ArgumentSeparator)
			b.WriteString(usage)
			return b.String()
		}
	} else if len(children) > 1 {
		var (
			childUsage  []string
			deduplicate = map[string]struct{}{}
		)
		for _, child := range children {
			usage := d.smartUsage(ctx, child, optional, true)
			if usage != "" {
				if _, ok := deduplicate[usage]; !ok {
					childUsage = append(childUsage, usage)
					deduplicate[usage] = struct{}{}
				}
			}
		}
		if len(childUsage) == 1 {
			b.WriteRune(ArgumentSeparator)
			if childOptional {
				b.WriteRune(UsageOptionalOpen)
				b.WriteString(childUsage[0])
				b.WriteRune(UsageOptionalClose)
			} else {
				b.WriteString(childUsage[0])
			}
			return b.String()
		} else if len(children) > 1 {
			s := new(bytes.Buffer)
			s.WriteRune(openChar)
			for i, child := range children {
				if i != 0 {
					s.WriteRune(UsageOr)
				}
				s.WriteString(child.UsageText())
			}
			if len(children) != 0 {
				b.WriteRune(ArgumentSeparator)
				_, _ = s.WriteTo(b)
				b.WriteRune(closeChar)
			}
		}
	}

	return b.String()
}
