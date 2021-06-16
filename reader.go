package brigodier

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// StringReader is a string reader used for input parsing.
type StringReader struct {
	Cursor int
	String string
}

// ReaderError indicates a StringReader error.
type ReaderError struct {
	Err    error
	Reader *StringReader
}

// ReaderInvalidValueError indicates an invalid value error.
type ReaderInvalidValueError struct {
	Type  ArgumentType // The expected value type
	Value string

	Err error // Optional underlying error
}

// Unwrap implements errors.Unwrap.
func (e *ReaderInvalidValueError) Unwrap() error { return e.Err }

// Error implements error.
func (e *ReaderInvalidValueError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("read invalid value %q for type %q", e.Value, e.Type)
}

// Unwrap implements errors.Unwrap.
func (e *ReaderError) Unwrap() error { return e.Err }
func (e *ReaderError) Error() string { return e.Err.Error() }

// CanRead indicates whether a next rune can be read to a call to Read.
func (r *StringReader) CanRead() bool { return r.CanReadLen(1) }

// CanReadLen indicates whether the next length runes can be read.
func (r *StringReader) CanReadLen(length int) bool { return r.Cursor+length <= len(r.String) }

// Peek returns the next rune without incrementing the Cursor.
func (r *StringReader) Peek() rune { return rune(r.String[r.Cursor]) }

// Skip increments the Cursor.
func (r *StringReader) Skip() { r.Cursor++ }

// ReadBool tries to read a bool.
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

// Read returns the next rune.
func (r *StringReader) Read() rune {
	c := r.String[r.Cursor]
	r.Cursor++
	return rune(c)
}

// ReadString returns the next quoted or unquoted string.
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
	// ErrReaderInvalidEscape indicates an invalid escape error.
	ErrReaderInvalidEscape = errors.New("read invalid escape character")
	// ErrReaderExpectedStartOfQuote occurs when a start quote is missing.
	ErrReaderExpectedStartOfQuote = errors.New("reader expected start of quote")
	// ErrReaderExpectedEndOfQuote occurs when an end quote is missing.
	ErrReaderExpectedEndOfQuote = errors.New("reader expected end of quote")
)

// ReadStringUntil reads a string until the terminator rune.
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

// ReadUnquotedString reads an unquoted string.
func (r *StringReader) ReadUnquotedString() string {
	start := r.Cursor
	for r.CanRead() && IsAllowedInUnquotedString(r.Peek()) {
		r.Skip()
	}
	return r.String[start:r.Cursor]
}

// ReadQuotedString reads a quoted string.
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
	// ErrReaderExpectedBool occurs when the reader expected a bool.
	ErrReaderExpectedBool = errors.New("reader expected bool")
	// ErrReaderExpectedFloat occurs when the reader expected a float.
	ErrReaderExpectedFloat = errors.New("reader expected float")
	// ErrReaderExpectedInt occurs when the reader expected a int.
	ErrReaderExpectedInt = errors.New("reader expected int")

	// ErrReaderInvalidInt occurs when the reader read an invalid int value.
	ErrReaderInvalidInt = errors.New("read invalid int")
	// ErrReaderInvalidFloat occurs when the reader read an invalid int float.
	ErrReaderInvalidFloat = errors.New("read invalid float")
)

// ReadInt tries to read an int32.
func (r *StringReader) ReadInt() (int, error) {
	i, err := r.ReadInt32()
	return int(i), err
}

// ReadInt32 tries to read an int32.
func (r *StringReader) ReadInt32() (int32, error) {
	i, err := r.readInt(32)
	return int32(i), err
}

// ReadInt64 tries to read an int64.
func (r *StringReader) ReadInt64() (int64, error) { return r.readInt(64) }

func (r *StringReader) readInt(bitSize int) (int64, error) {
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
	i, err := strconv.ParseInt(number, 0, bitSize)
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
	return i, nil
}

// ReadFloat32 tries to read a float32.
func (r *StringReader) ReadFloat32() (float32, error) {
	f, err := r.readFloat(32)
	return float32(f), err
}

// ReadFloat64 tries to read a float64.
func (r *StringReader) ReadFloat64() (float64, error) {
	return r.readFloat(64)
}

func (r *StringReader) readFloat(bitSize int) (float64, error) {
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
	f, err := strconv.ParseFloat(number, bitSize)
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

// Remaining returns the remaining string beginning at the current Cursor
func (r *StringReader) Remaining() string { return r.String[r.Cursor:] }

// RemainingLen returns the remaining string length beginning at the current Cursor
func (r *StringReader) RemainingLen() int { return len(r.String) - r.Cursor }

const (
	// SyntaxDoubleQuote is a double quote.
	SyntaxDoubleQuote rune = '"'
	// SyntaxSingleQuote is a single quote.
	SyntaxSingleQuote rune = '\''
	// SyntaxEscape is an escape.
	SyntaxEscape rune = '\\'
)

// IsAllowedNumber indicated whether c is an allowed number rune.
func IsAllowedNumber(c rune) bool { return c >= '0' && c <= '9' || c == '.' || c == '-' }

// IsQuotedStringStart indicated whether c is the start of a quoted string.
func IsQuotedStringStart(c rune) bool {
	return c == SyntaxDoubleQuote || c == SyntaxSingleQuote
}

// IsAllowedInUnquotedString indicated whether c is an allowed rune in an unquoted string.
func IsAllowedInUnquotedString(c rune) bool {
	return c >= '0' && c <= '9' ||
		c >= 'A' && c <= 'Z' ||
		c >= 'a' && c <= 'z' ||
		c == '_' || c == '-' ||
		c == '.' || c == '+'
}

// StringRange stores a range indicating the start and end of a string
type StringRange struct{ Start, End int }

// IsEmpty indicated whether Start and End is equal.
func (r *StringRange) IsEmpty() bool {
	return r.Start == r.End
}

// Copy copies the StringRange.
func (r StringRange) Copy() StringRange { return r }

// Get returns the substring of s from Start to End.
func (r *StringRange) Get(s string) string { return s[r.Start:r.End] }

// EncompassingRange returns the min and max StringRange of two ranges.
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
