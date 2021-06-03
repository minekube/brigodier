package brigodier

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

type (
	Suggestions struct {
		Range       StringRange
		Suggestions []*Suggestion
	}
	Suggestion struct {
		Range   StringRange
		Text    string
		Tooltip fmt.Stringer
	}
	SuggestionContext struct {
		Parent CommandNode
		Start  int
	}
	SuggestionsBuilder struct {
		Input              string
		InputLowerCase     string
		Start              int
		Remaining          string
		RemainingLowerCase string
		Result             []*Suggestion
	}
)

func (b *SuggestionsBuilder) Suggest(text string) *SuggestionsBuilder {
	if text != b.Remaining {
		b.Result = append(b.Result, &Suggestion{
			Range: StringRange{Start: b.Start, End: len(b.Input)},
			Text:  text,
		})
	}
	return b
}

func (b *SuggestionsBuilder) Build() *Suggestions { return CreateSuggestion(b.Input, b.Result) }

func (d *Dispatcher) CompletionSuggestions(parse *ParseResults) (*Suggestions, error) {
	return d.CompletionSuggestionsCursor(parse, len(parse.Reader.String))
}
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
	for _, node := range parent.Children() {
		if !CanProvideSuggestions(node) {
			continue
		}
		suggestions = append(suggestions, ProvideSuggestions(node, ctx.build(truncatedInput), &SuggestionsBuilder{
			Input:              truncatedInput,
			InputLowerCase:     truncatedInputLowerCase,
			Start:              start,
			Remaining:          truncatedInput[start:],
			RemainingLowerCase: truncatedInputLowerCase[start:],
		}))
	}

	return MergeSuggestions(fullInput, suggestions), nil
}

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

var ErrNoNodeBeforeCursor = errors.New("can't find node before cursor")

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

type SuggestionProvider interface {
	Suggestions(*CommandContext, *SuggestionsBuilder) *Suggestions
}

func CanProvideSuggestions(i interface{}) bool {
	if i == nil {
		return false
	}
	_, ok := i.(SuggestionProvider)
	return ok
}

func ProvideSuggestions(i interface{}, ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if i == nil {
		return emptySuggestions
	}
	if i, ok := i.(SuggestionProvider); ok {
		return i.Suggestions(ctx, builder)
	}
	return emptySuggestions
}

func (a *ArgumentCommandNode) Suggestions(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if a.customSuggestions == nil {
		return ProvideSuggestions(a.argType, ctx, builder)
	}
	return a.customSuggestions.Suggestions(ctx, builder)
}
func (n *LiteralCommandNode) Suggestions(_ *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if n.literalLowerCase == "" {
		n.literalLowerCase = strings.ToLower(n.Literal)
	}
	if strings.HasPrefix(n.literalLowerCase, builder.RemainingLowerCase) {
		return builder.Suggest(n.Literal).Build()
	}
	return emptySuggestions
}

func (t *ArgumentTypeFuncs) Suggestions(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if t.SuggestionsFn == nil {
		return emptySuggestions
	}
	return t.SuggestionsFn(ctx, builder)
}
