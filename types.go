package brigodier

import (
	"errors"
	"fmt"
	"math"
)

var (
	Bool ArgumentType
	Int  ArgumentType
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
}

type IntegerArgumentType struct{ Min, Max int }

var (
	ErrArgumentIntegerTooLow  = errors.New("integer too low")
	ErrArgumentIntegerTooHigh = errors.New("integer too high")
)

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

func (t *IntegerArgumentType) String() string { return "int" }
