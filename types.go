package brigodier

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Builtin argument types.
var (
	Bool ArgumentType // Bool argument type.

	Int     ArgumentType // Int argument type.
	Int32   = Int        // Int32 is an alias of Int.
	Float64 ArgumentType // Float64 argument type.

	// String argument type is quoted or unquoted.
	String ArgumentType
	// StringWord argument type is a single word.
	StringWord ArgumentType
	// StringPhrase argument type is phrase.
	StringPhrase ArgumentType
)

// ArgumentType is a parsable argument type.
type ArgumentType interface {
	// Parse parses the argument from the given reader input.
	Parse(rd *StringReader) (interface{}, error)
	String() string // String returns the name of the type.
}

// ArgumentTypeFuncs is a convenient struct implementing ArgumentType.
type ArgumentTypeFuncs struct {
	Name    string                                      // The name of the argument type returned by ArgumentType.String.
	ParseFn func(rd *StringReader) (interface{}, error) // ArgumentType.Parse
	// Optional suggestions for use with Dispatcher.CompletionSuggestions (ProvideSuggestions).
	SuggestionsFn func(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions
}

func (t *ArgumentTypeFuncs) Parse(rd *StringReader) (interface{}, error) { return t.ParseFn(rd) }
func (t *ArgumentTypeFuncs) String() string                              { return t.Name }

// Initialize builtin argument types.
func init() {
	Bool = &ArgumentTypeFuncs{
		Name:    "bool",
		ParseFn: func(rd *StringReader) (interface{}, error) { return rd.ReadBool() },
		SuggestionsFn: func(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions {
			if strings.HasPrefix("true", builder.RemainingLowerCase) {
				builder.Suggest("true")
			} else if strings.HasPrefix("false", builder.RemainingLowerCase) {
				builder.Suggest("false")
			}
			return builder.Build()
		},
	}
	Int = &IntegerArgumentType{
		Min: math.MinInt32,
		Max: math.MaxInt32,
	}
	Float64 = &Float64ArgumentType{
		Min: -math.MaxFloat64,
		Max: math.MaxFloat64,
	}
	String = QuotablePhase
	StringWord = SingleWord
	StringPhrase = GreedyPhrase
}

// Int returns the parsed int argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) Int(argumentName string) int {
	if c.Arguments == nil {
		return 0
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return 0
	}
	v, _ := r.Result.(int)
	return v
}

// Bool returns the parsed bool argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) Bool(argumentName string) bool {
	if c.Arguments == nil {
		return false
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return false
	}
	v, _ := r.Result.(bool)
	return v
}

// Float64 returns the parsed float64 argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) Float64(argumentName string) float64 {
	if c.Arguments == nil {
		return 0
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return 0
	}
	v, _ := r.Result.(float64)
	return v
}

// String returns the parsed string argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) String(argumentName string) string {
	if c.Arguments == nil {
		return ""
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return ""
	}
	v, _ := r.Result.(string)
	return v
}

// StringType is a string ArgumentType.
type StringType uint8

// Builtin string argument types.
const (
	SingleWord    StringType = iota // A single-word string.
	QuotablePhase                   // A quotable phrase string.
	GreedyPhrase                    // A "greedy" string phrase.
)

func (t StringType) String() string { return "string" }
func (t StringType) Parse(rd *StringReader) (interface{}, error) {
	switch t {
	case GreedyPhrase:
		text := rd.Remaining()
		rd.Cursor = len(rd.String)
		return text, nil
	case SingleWord:
		return rd.ReadUnquotedString(), nil
	default:
		return rd.ReadString()
	}
}

type IntegerArgumentType struct{ Min, Max int }
type Float64ArgumentType struct{ Min, Max float64 }

var (
	// ErrArgumentIntegerTooHigh occurs when the found integer is higher than the specified maximum.
	ErrArgumentIntegerTooHigh = errors.New("integer too high")
	// ErrArgumentIntegerTooLow occurs when the found integer is lower than the specified minimum.
	ErrArgumentIntegerTooLow = errors.New("integer too low")

	// ErrArgumentFloatTooHigh occurs when the found float is higher than the specified maximum.
	ErrArgumentFloatTooHigh = errors.New("float too high")
	// ErrArgumentFloatTooLow occurs when the found float is lower than the specified minimum.
	ErrArgumentFloatTooLow = errors.New("float too low")
)

func (t *IntegerArgumentType) String() string { return "int" }
func (t *IntegerArgumentType) Parse(rd *StringReader) (interface{}, error) {
	start := rd.Cursor
	result, err := rd.ReadInt()
	if err != nil {
		return nil, err
	}
	if result < t.Min {
		rd.Cursor = start
		return nil, &CommandSyntaxError{Err: fmt.Errorf("%w (%d < %d)",
			ErrArgumentIntegerTooLow, result, t.Min)}
	}
	if result > t.Max {
		rd.Cursor = start
		return nil, &CommandSyntaxError{Err: fmt.Errorf("%w (%d > %d)",
			ErrArgumentIntegerTooHigh, result, t.Max)}
	}
	return result, nil
}

func (t *Float64ArgumentType) String() string { return "float64" }
func (t *Float64ArgumentType) Parse(rd *StringReader) (interface{}, error) {
	start := rd.Cursor
	result, err := rd.ReadFloat64()
	if err != nil {
		return nil, err
	}
	if result < t.Min {
		rd.Cursor = start
		return nil, &CommandSyntaxError{Err: fmt.Errorf("%w (%f < %f)",
			ErrArgumentFloatTooLow, result, t.Min)}
	}
	if result > t.Max {
		rd.Cursor = start
		return nil, &CommandSyntaxError{Err: fmt.Errorf("%w (%f > %f)",
			ErrArgumentFloatTooHigh, result, t.Max)}
	}
	return result, nil
}
