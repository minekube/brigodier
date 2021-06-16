package brigodier

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Builtin argument types.
var (
	// String argument type is quoted or unquoted.
	String ArgumentType = QuotablePhase
	// StringWord argument type is a single word.
	StringWord ArgumentType = SingleWord
	// StringPhrase argument type is phrase.
	StringPhrase ArgumentType = GreedyPhrase
	// Bool argument type.
	Bool ArgumentType = &BoolArgumentType{}

	// Int32 argument type.
	Int32 ArgumentType = &Int32ArgumentType{
		Min: MinInt32,
		Max: MaxInt32,
	}
	// Int64 argument type.
	Int64 ArgumentType = &Int64ArgumentType{
		Min: MinInt64,
		Max: MaxInt64,
	}
	// Int is an alias of Int32.
	Int = Int32

	// Float32 argument type.
	Float32 ArgumentType = &Float32ArgumentType{
		Min: MinFloat32,
		Max: MaxFloat32,
	}
	// Float64 argument type.
	Float64 ArgumentType = &Float64ArgumentType{
		Min: MinFloat64,
		Max: MaxFloat64,
	}
)

// Default minimums and maximums of builtin numeric ArgumentType values.
const (
	MinInt32   = math.MinInt32
	MaxInt32   = math.MaxInt32
	MinInt64   = math.MinInt32
	MaxInt64   = math.MaxInt64
	MinFloat32 = -math.MaxFloat32
	MaxFloat32 = math.MaxFloat32
	MinFloat64 = -math.MaxFloat64
	MaxFloat64 = math.MaxFloat64
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

// Int is the same as CommandContext.Int32.
func (c *CommandContext) Int(argumentName string) int {
	return int(c.Int32(argumentName))
}

// Int32 returns the parsed int32 argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) Int32(argumentName string) int32 {
	if c.Arguments == nil {
		return 0
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return 0
	}
	v, _ := r.Result.(int32)
	return v
}

// Int64 returns the parsed int64 argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) Int64(argumentName string) int64 {
	if c.Arguments == nil {
		return 0
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return 0
	}
	v, _ := r.Result.(int64)
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

// Float32 returns the parsed float32 argument from the command context.
// It returns the zero-value if not found.
func (c *CommandContext) Float32(argumentName string) float32 {
	if c.Arguments == nil {
		return 0
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return 0
	}
	v, _ := r.Result.(float32)
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

type BoolArgumentType struct{}
type Int32ArgumentType struct{ Min, Max int32 }
type Int64ArgumentType struct{ Min, Max int64 }
type Float32ArgumentType struct{ Min, Max float32 }
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

func (t *BoolArgumentType) String() string                              { return "bool" }
func (t *BoolArgumentType) Parse(rd *StringReader) (interface{}, error) { return rd.ReadBool() }
func (t *BoolArgumentType) Suggestions(_ *CommandContext, builder *SuggestionsBuilder) *Suggestions {
	if strings.HasPrefix("true", builder.RemainingLowerCase) {
		builder.Suggest("true")
	} else if strings.HasPrefix("false", builder.RemainingLowerCase) {
		builder.Suggest("false")
	}
	return builder.Build()
}
func (t *Int32ArgumentType) String() string { return "int32" }
func (t *Int32ArgumentType) Parse(rd *StringReader) (interface{}, error) {
	i, err := parseInt(rd, 32, int64(t.Min), int64(t.Max))
	return int32(i), err
}
func (t *Int64ArgumentType) String() string { return "int64" }
func (t *Int64ArgumentType) Parse(rd *StringReader) (interface{}, error) {
	return parseInt(rd, 64, t.Min, t.Max)
}
func parseInt(rd *StringReader, bitSize int, min, max int64) (int64, error) {
	start := rd.Cursor
	result, err := rd.readInt(bitSize)
	if err != nil {
		return 0, err
	}
	if result < min {
		rd.Cursor = start
		return 0, &CommandSyntaxError{Err: fmt.Errorf("%w (%d < %d)",
			ErrArgumentIntegerTooLow, result, min)}
	}
	if result > max {
		rd.Cursor = start
		return 0, &CommandSyntaxError{Err: fmt.Errorf("%w (%d > %d)",
			ErrArgumentIntegerTooHigh, result, max)}
	}
	return result, nil
}

func (t *Float32ArgumentType) String() string { return "float32" }
func (t *Float32ArgumentType) Parse(rd *StringReader) (interface{}, error) {
	f, err := parseFloat(rd, 32, float64(t.Min), float64(t.Max))
	return float32(f), err
}
func (t *Float64ArgumentType) String() string { return "float64" }
func (t *Float64ArgumentType) Parse(rd *StringReader) (interface{}, error) {
	return parseFloat(rd, 64, t.Min, t.Max)
}
func parseFloat(rd *StringReader, bitSize int, min, max float64) (float64, error) {
	start := rd.Cursor
	result, err := rd.readFloat(bitSize)
	if err != nil {
		return 0, err
	}
	if result < min {
		rd.Cursor = start
		return 0, &CommandSyntaxError{Err: fmt.Errorf("%w (%f < %f)",
			ErrArgumentFloatTooLow, result, min)}
	}
	if result > max {
		rd.Cursor = start
		return 0, &CommandSyntaxError{Err: fmt.Errorf("%w (%f > %f)",
			ErrArgumentFloatTooHigh, result, max)}
	}
	return result, nil
}
