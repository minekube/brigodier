package brigodier

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

var (
	Bool ArgumentType

	Int     ArgumentType
	Int32   = Int
	Float64 ArgumentType

	String             ArgumentType
	StringSingleWord   ArgumentType
	StringGreedyPhrase ArgumentType
)

type ArgumentType interface {
	Parse(rd *StringReader) (interface{}, error)
	String() string // The name of the type.
}

type ArgumentTypeFuncs struct {
	Name          string
	ParseFn       func(rd *StringReader) (interface{}, error)
	SuggestionsFn func(ctx *CommandContext, builder *SuggestionsBuilder) *Suggestions
}

func (t *ArgumentTypeFuncs) Parse(rd *StringReader) (interface{}, error) { return t.ParseFn(rd) }
func (t *ArgumentTypeFuncs) String() string                              { return t.Name }

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
	StringSingleWord = SingleWord
	StringGreedyPhrase = GreedyPhrase
}

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

type StringType uint8

const (
	SingleWord StringType = iota
	QuotablePhase
	GreedyPhrase
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
	ErrArgumentIntegerTooLow  = errors.New("integer too low")
	ErrArgumentIntegerTooHigh = errors.New("integer too high")

	ErrArgumentFloatTooLow  = errors.New("float too low")
	ErrArgumentFloatTooHigh = errors.New("float too high")
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
