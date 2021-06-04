package brigodier

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// SuggestionProvider provides Suggestions and can optionally be implemented
// by a CommandNode to add suggestions support to an command argument type.
type SuggestionProvider interface {
	Suggestions(*CommandContext, *SuggestionsBuilder) *Suggestions
}

// ProvideSuggestions returns the Suggestions if i implements
// SuggestionProvider or returns empty Suggestions if it doesn't.
func ProvideSuggestions(i interface{}, ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if i == nil {
		return emptySuggestions
	}
	if i, ok := i.(SuggestionProvider); ok {
		return i.Suggestions(ctx, builder)
	}
	return emptySuggestions
}

// CanProvideSuggestions tests whether i implements SuggestionProvider.
func CanProvideSuggestions(i interface{}) bool {
	if i == nil {
		return false
	}
	_, ok := i.(SuggestionProvider)
	return ok
}

type (
	// Suggestions are command suggestions within a string range.
	Suggestions struct {
		Range       StringRange
		Suggestions []*Suggestion
	}
	// Suggestion is a command suggestion.
	Suggestion struct {
		Range   StringRange
		Text    string
		Tooltip fmt.Stringer
	}
	// SuggestionContext is a command suggestion context.
	SuggestionContext struct {
		Parent CommandNode
		Start  int
	}
	// SuggestionsBuilder is a convenient struct for building Suggestions.
	SuggestionsBuilder struct {
		Input              string
		InputLowerCase     string
		Start              int
		Remaining          string
		RemainingLowerCase string
		Result             []*Suggestion
	}
)

// Suggest adds a suggestion to the builder.
func (b *SuggestionsBuilder) Suggest(text string) *SuggestionsBuilder {
	if text != b.Remaining {
		b.Result = append(b.Result, &Suggestion{
			Range: StringRange{Start: b.Start, End: len(b.Input)},
			Text:  text,
		})
	}
	return b
}

// Build returns a Suggestions build from the builder.
func (b *SuggestionsBuilder) Build() *Suggestions { return CreateSuggestion(b.Input, b.Result) }

// CompletionSuggestions gets suggestions for a parsed input string on what comes next.
//
// As it is ultimately up to custom argument types to provide suggestions.
//
// The suggestions provided will be in the context of the end of the parsed input string, but may suggest
// new or replacement strings for earlier in the input string. For example, if the end of the string was
// foobar but an argument preferred it to be minecraft:foobar, it will suggest a replacement for that
// whole segment of the input.
func (d *Dispatcher) CompletionSuggestions(parse *ParseResults) (*Suggestions, error) {
	return d.CompletionSuggestionsCursor(parse, len(parse.Reader.String))
}

// CompletionSuggestionsCursor gets suggestions for a parsed input
// string on what comes next with a cursor to begin suggesting at.
// See CompletionSuggestions for details.
func (d *Dispatcher) CompletionSuggestionsCursor(parse *ParseResults, cursor int) (*Suggestions, error) {
	ctx := parse.Context

	nodeBeforeCursor, err := ctx.FindSuggestionContext(cursor)
	if err != nil {
		return nil, err
	}
	parent := nodeBeforeCursor.Parent
	start := min(nodeBeforeCursor.Start, cursor)

	fullInput := parse.Reader.String
	truncatedInput := fullInput[:cursor]
	truncatedInputLowerCase := strings.ToLower(truncatedInput)
	suggestions := make([]*Suggestions, 0, len(parent.Children()))
	parent.ChildrenOrdered().Range(func(_ string, node CommandNode) bool {
		if !CanProvideSuggestions(node) {
			return true
		}
		suggestions = append(suggestions, ProvideSuggestions(node, ctx.build(truncatedInput), &SuggestionsBuilder{
			Input:              truncatedInput,
			InputLowerCase:     truncatedInputLowerCase,
			Start:              start,
			Remaining:          truncatedInput[start:],
			RemainingLowerCase: truncatedInputLowerCase[start:],
		}))
		return true
	})

	return MergeSuggestions(fullInput, suggestions), nil
}

// MergeSuggestions merges multiple Suggestions into one.
func MergeSuggestions(command string, input []*Suggestions) *Suggestions {
	if len(input) == 0 {
		return emptySuggestions
	}
	if len(input) == 1 {
		return input[0]
	}

	texts := make(map[string]struct{}, len(input)) // deduplicate
	a := make([]*Suggestion, 0, len(input))
	for _, suggestions := range input {
		for _, suggestion := range suggestions.Suggestions {
			if _, ok := texts[suggestion.Text]; !ok {
				texts[suggestion.Text] = struct{}{}
				a = append(a, suggestion)
			}
		}
	}
	return CreateSuggestion(command, a)
}

// CreateSuggestion creates a Suggestions from multiple Suggestion.
func CreateSuggestion(command string, suggestions []*Suggestion) *Suggestions {
	if len(suggestions) == 0 {
		return emptySuggestions
	}
	start := math.MaxInt32
	end := math.MinInt32
	for _, suggestion := range suggestions {
		start = min(suggestion.Range.Start, start)
		end = max(suggestion.Range.End, end)
	}
	strRange := &StringRange{Start: start, End: end}
	texts := make(map[string]struct{}, len(suggestions)) // deduplicate
	a := make([]*Suggestion, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if _, ok := texts[suggestion.Text]; !ok {
			texts[suggestion.Text] = struct{}{}
			a = append(a, suggestion.Expand(command, strRange))
		}
	}
	sort.Slice(a, func(i, j int) bool { return a[i].compareToIgnoreCase(a[j]) }) // TODO test
	return &Suggestions{Range: *strRange, Suggestions: a}
}

func (s *Suggestion) compareToIgnoreCase(other *Suggestion) bool {
	return strings.EqualFold(s.Text, other.Text)
}

// Expand expands a command suggestion if appropriate.
func (s *Suggestion) Expand(command string, strRange *StringRange) *Suggestion {
	if *strRange == s.Range {
		return s
	}
	result := new(strings.Builder)
	if strRange.Start < s.Range.Start {
		result.WriteString(command[strRange.Start:s.Range.Start])
	}
	result.WriteString(s.Text)
	if strRange.End > s.Range.End {
		result.WriteString(command[s.Range.End:strRange.End])
	}
	return &Suggestion{Range: s.Range, Text: result.String(), Tooltip: s.Tooltip}
}

var emptySuggestions = &Suggestions{}

// ErrNoNodeBeforeCursor indicates that CommandContext.FindSuggestionContext
// could not find a matching node before the specified cursor.
var ErrNoNodeBeforeCursor = errors.New("can't find node before cursor")

// FindSuggestionContext tries to calculate the SuggestionContext starting at cursor
// or returns ErrNoNodeBeforeCursor if no node could be found before the specified cursor.
func (c *CommandContext) FindSuggestionContext(cursor int) (*SuggestionContext, error) {
	if c.Range.Start <= cursor {
		if c.Range.End < cursor {
			if c.Child != nil {
				return c.Child.FindSuggestionContext(cursor)
			} else if len(c.Nodes) != 0 {
				last := c.Nodes[len(c.Nodes)-1]
				return &SuggestionContext{
					Parent: last.Node,
					Start:  last.Range.End + 1,
				}, nil
			} else {
				return &SuggestionContext{
					Parent: c.RootNode,
					Start:  c.Range.Start,
				}, nil
			}
		} else {
			prev := c.RootNode
			for _, node := range c.Nodes {
				nodeRange := node.Range
				if nodeRange.Start <= cursor && cursor <= nodeRange.End {
					return &SuggestionContext{
						Parent: prev,
						Start:  nodeRange.Start,
					}, nil
				}
				prev = node.Node
			}
			if prev == nil {
				return nil, ErrNoNodeBeforeCursor
			}
			return &SuggestionContext{
				Parent: prev,
				Start:  c.Range.Start,
			}, nil
		}
	}
	return nil, ErrNoNodeBeforeCursor
}

// Suggestions implements SuggestionProvider.
func (a *ArgumentCommandNode) Suggestions(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if a.customSuggestions == nil {
		return ProvideSuggestions(a.argType, ctx, builder)
	}
	return a.customSuggestions.Suggestions(ctx, builder)
}

// Suggestions implements SuggestionProvider.
func (n *LiteralCommandNode) Suggestions(_ *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if n.cachedLiteralLowerCase == "" {
		n.cachedLiteralLowerCase = strings.ToLower(n.Literal)
	}
	if strings.HasPrefix(n.cachedLiteralLowerCase, builder.RemainingLowerCase) {
		return builder.Suggest(n.Literal).Build()
	}
	return emptySuggestions
}

// Suggestions implements SuggestionProvider.
func (t *ArgumentTypeFuncs) Suggestions(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if t.SuggestionsFn == nil {
		return emptySuggestions
	}
	return t.SuggestionsFn(ctx, builder)
}
