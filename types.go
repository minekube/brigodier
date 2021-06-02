package brigodier

import (
	"errors"
	"fmt"
	"math"
)

var (
	Bool    ArgumentType
	Int     ArgumentType
	Int32   = Int
	Float64 ArgumentType
)

type ArgumentType interface {
	Parse(rd *StringReader) (interface{}, error)
	String() string // The name of the type.
}

type ArgumentTypeFuncs struct {
	Name    string
	ParseFn func(rd *StringReader) (interface{}, error)
}

func (t *ArgumentTypeFuncs) Parse(rd *StringReader) (interface{}, error) { return t.ParseFn(rd) }
func (t *ArgumentTypeFuncs) String() string                              { return t.Name }

func init() {
	Bool = &ArgumentTypeFuncs{
		Name:    "bool",
		ParseFn: func(rd *StringReader) (interface{}, error) { return rd.ReadBool() },
	}
	Int = &IntegerArgumentType{
		Min: math.MinInt32,
		Max: math.MaxInt32,
	}
	Float64 = &Float64ArgumentType{
		Min: -math.MaxFloat64,
		Max: math.MaxFloat64,
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
