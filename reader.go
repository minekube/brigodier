package brigodier

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type StringReader struct {
	Cursor int
	String string
}

type ReaderError struct {
	Err    error
	Reader *StringReader
}

type ReaderInvalidValueError struct {
	Type  ArgumentType // The expected value type
	Value string

	Err error // Optional underlying error
}

func (e *ReaderInvalidValueError) Unwrap() error { return e.Err }
func (e *ReaderInvalidValueError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("read invalid value %q for type %q", e.Value, e.Type)
}

var (
	ErrReaderExpectedBool = errors.New("reader expected bool")
)

func (e *ReaderError) Unwrap() error { return e.Err }
func (e *ReaderError) Error() string { return e.Err.Error() }

func (r *StringReader) CanRead() bool              { return r.CanReadLen(1) }
func (r *StringReader) CanReadLen(length int) bool { return r.Cursor+length <= len(r.String) }
func (r *StringReader) Peek() rune                 { return rune(r.String[r.Cursor]) }
func (r *StringReader) Skip()                      { r.Cursor++ }
func (r *StringReader) ReadBool() (bool, error) {
	start := r.Cursor
	value, err := r.ReadString()
	if err != nil {
		return false, err
	}
	if len(value) == 0 {
		return false, &CommandSyntaxError{Err: &ReaderError{
			Err:    ErrReaderExpectedBool,
			Reader: r,
		}}
	}
	if strings.EqualFold(value, "true") {
		return true, nil
	} else if strings.EqualFold(value, "false") {
		return false, nil
	}
	r.Cursor = start
	return false, &CommandSyntaxError{Err: &ReaderError{
		Err: &ReaderInvalidValueError{
			Type:  Bool,
			Value: value,
		},
		Reader: r,
	}}
}

func (r *StringReader) Read() rune {
	c := r.String[r.Cursor]
	r.Cursor++
	return rune(c)
}
func (r *StringReader) ReadString() (string, error) {
	if !r.CanRead() {
		return "", nil
	}
	next := r.Peek()
	if IsQuotedStringStart(next) {
		r.Skip()
		return r.ReadStringUntil(next)
	}
	return r.ReadUnquotedString(), nil
}

var (
	ErrReaderInvalidEscape        = errors.New("read invalid escape character")
	ErrReaderExpectedEndOfQuote   = errors.New("reader expected end of quote")
	ErrReaderExpectedStartOfQuote = errors.New("reader expected start of quote")
)

func (r *StringReader) ReadStringUntil(terminator rune) (string, error) {
	var (
		result  strings.Builder
		escaped = false
	)
	for r.CanRead() {
		c := r.Read()
		if escaped {
			if c == terminator || c == SyntaxEscape {
				result.WriteRune(c)
				escaped = false
			} else {
				r.Cursor = r.Cursor - 1
				return "", &CommandSyntaxError{Err: &ReaderError{
					Err: &ReaderInvalidValueError{
						Value: string(c),
						Err:   ErrReaderInvalidEscape,
					},
					Reader: r,
				}}
			}
		} else if c == SyntaxEscape {
			escaped = true
		} else if c == terminator {
			return result.String(), nil
		} else {
			result.WriteRune(c)
		}
	}

	return "", &CommandSyntaxError{Err: &ReaderError{
		Err:    ErrReaderExpectedEndOfQuote,
		Reader: r,
	}}
}

func (r *StringReader) ReadUnquotedString() string {
	start := r.Cursor
	for r.CanRead() && IsAllowedInUnquotedString(r.Peek()) {
		r.Skip()
	}
	return r.String[start:r.Cursor]
}

func (r *StringReader) ReadQuotedString() (string, error) {
	if !r.CanRead() {
		return "", nil
	}
	next := r.Peek()
	if !IsQuotedStringStart(next) {
		return "", &CommandSyntaxError{Err: &ReaderError{
			Err:    ErrReaderExpectedStartOfQuote,
			Reader: r,
		}}
	}
	r.Skip()
	return r.ReadStringUntil(next)
}

var (
	ErrReaderExpectedFloat = errors.New("reader expected float")
	ErrReaderExpectedInt   = errors.New("reader expected int")
	ErrReaderInvalidInt    = errors.New("read invalid int")
	ErrReaderInvalidFloat  = errors.New("read invalid float")
)

func (r *StringReader) ReadInt() (int, error) {
	start := r.Cursor
	for r.CanRead() && IsAllowedNumber(r.Peek()) {
		r.Skip()
	}
	number := r.String[start:r.Cursor]
	if number == "" {
		return 0, &CommandSyntaxError{Err: &ReaderError{
			Err:    ErrReaderExpectedInt,
			Reader: r,
		}}
	}
	i, err := strconv.ParseInt(number, 0, 32)
	if err != nil {
		r.Cursor = start
		return 0, &CommandSyntaxError{Err: &ReaderError{
			Err: &ReaderInvalidValueError{
				Value: number,
				Err:   fmt.Errorf("%w (%q): %v", ErrReaderInvalidInt, number, err),
			},
			Reader: r,
		}}
	}
	return int(i), nil
}
func (r *StringReader) ReadFloat64() (float64, error) {
	start := r.Cursor
	for r.CanRead() && IsAllowedNumber(r.Peek()) {
		r.Skip()
	}
	number := r.String[start:r.Cursor]
	if number == "" {
		return 0, &CommandSyntaxError{Err: &ReaderError{
			Err:    ErrReaderExpectedFloat,
			Reader: r,
		}}
	}
	f, err := strconv.ParseFloat(number, 64)
	if err != nil {
		r.Cursor = start
		return 0, &CommandSyntaxError{Err: &ReaderError{
			Err: &ReaderInvalidValueError{
				Value: number,
				Err:   fmt.Errorf("%w (%q): %v", ErrReaderInvalidFloat, number, err),
			},
			Reader: r,
		}}
	}
	return f, nil
}

func (r *StringReader) Remaining() string { return r.String[r.Cursor:] }
func (r *StringReader) RemainingLen() int { return len(r.String) - r.Cursor }

const (
	SyntaxDoubleQuote rune = '"'
	SyntaxSingleQuote rune = '\''
	SyntaxEscape      rune = '\\'
)

func IsAllowedNumber(c rune) bool { return c >= '0' && c <= '9' || c == '.' || c == '-' }

func IsQuotedStringStart(c rune) bool {
	return c == SyntaxDoubleQuote || c == SyntaxSingleQuote
}
func IsAllowedInUnquotedString(c rune) bool {
	return c >= '0' && c <= '9' ||
		c >= 'A' && c <= 'Z' ||
		c >= 'a' && c <= 'z' ||
		c == '_' || c == '-' ||
		c == '.' || c == '+'
}

type StringRange struct{ Start, End int }

func (r *StringRange) IsEmpty() bool {
	return r.Start == r.End
}

func (r StringRange) Copy() StringRange { return r }

func (r *StringRange) Get(s string) string {
	return s[r.Start:r.End]
}

func EncompassingRange(r1, r2 *StringRange) *StringRange {
	return &StringRange{
		Start: min(r1.Start, r2.Start),
		End:   max(r1.End, r2.End),
	}
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
