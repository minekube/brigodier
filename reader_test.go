package brigodier

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStringReader_CanRead(t *testing.T) {
	r := StringReader{String: "abc"}
	require.True(t, r.CanRead())
	r.Skip() // a
	require.True(t, r.CanRead())
	r.Skip() // b
	require.True(t, r.CanRead())
	r.Skip() // c
	require.False(t, r.CanRead())
}
func TestStringReader_RemainingLen(t *testing.T) {
	r := StringReader{String: "abc"}
	require.Equal(t, 3, r.RemainingLen())
	r.Skip()
	require.Equal(t, 2, r.RemainingLen())
	r.Skip()
	require.Equal(t, 1, r.RemainingLen())
	r.Skip()
	require.Equal(t, 0, r.RemainingLen())
}
func TestStringReader_CanReadLen(t *testing.T) {
	r := StringReader{String: "abc"}
	require.Equal(t, 'a', r.Peek())
	require.Equal(t, 0, r.Cursor)
	r.Cursor = 2
	require.Equal(t, 'c', r.Peek())
	require.Equal(t, 2, r.Cursor)
}
func TestStringReader_Read(t *testing.T) {
	r := StringReader{String: "abc"}
	require.Equal(t, 'a', r.Read())
	require.Equal(t, 'b', r.Read())
	require.Equal(t, 'c', r.Read())
	require.Equal(t, 3, r.Cursor)
}
func TestStringReader_Skip(t *testing.T) {
	r := StringReader{String: "abc"}
	r.Skip()
	require.Equal(t, 1, r.Cursor)
}
func TestStringReader_Remaining(t *testing.T) {
	r := StringReader{String: "Hello!"}
	require.Equal(t, "Hello!", r.Remaining())
	r.Cursor = 3
	require.Equal(t, "lo!", r.Remaining())
	r.Cursor = 6
	require.Equal(t, "", r.Remaining())
}
func TestStringReader_ReadUnquotedString(t *testing.T) {
	r := StringReader{String: "hello world"}
	require.Equal(t, "hello", r.ReadUnquotedString())
	require.Equal(t, " world", r.Remaining())
}
func TestStringReader_ReadUnquotedString_Empty_WithRemaining(t *testing.T) {
	r := StringReader{String: " hello world"}
	require.Equal(t, "", r.ReadUnquotedString())
	require.Equal(t, " hello world", r.Remaining())
}
func TestStringReader_ReadQuotedString(t *testing.T) {
	r := StringReader{String: `"hello world"`}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_Mixed_DoubleInsideSingle(t *testing.T) {
	r := StringReader{String: `'hello "world"'`}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, `hello "world"`, s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_SingleInsideDouble(t *testing.T) {
	r := StringReader{String: `"hello 'world'"`}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, `hello 'world'`, s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_Empty(t *testing.T) {
	r := StringReader{}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Empty(t, s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_EmptyQuoted(t *testing.T) {
	r := StringReader{String: `""`}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Empty(t, s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_EmptyQuoted_WithRemaining(t *testing.T) {
	r := StringReader{String: `"" hello world`}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Empty(t, s)
	require.Equal(t, " hello world", r.Remaining())
}
func TestStringReader_ReadQuotedString_WithEscapeQuote(t *testing.T) {
	r := StringReader{String: "\"hello \\\"world\\\"\""}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, "hello \"world\"", s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_WithEscapedEscapes(t *testing.T) {
	r := StringReader{String: "\"\\\\o/\""}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, "\\o/", s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadQuotedString_WithRemaining(t *testing.T) {
	r := StringReader{String: "\"hello world\" foo bar"}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
	require.Equal(t, " foo bar", r.Remaining())
}
func TestStringReader_ReadQuotedString_WithImmediateRemaining(t *testing.T) {
	r := StringReader{String: "\"hello world\"foo bar"}
	s, err := r.ReadQuotedString()
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
	require.Equal(t, "foo bar", r.Remaining())
}
func TestStringReader_ReadQuotedString_NoOpen(t *testing.T) {
	r := StringReader{String: "hello world\""}
	_, err := r.ReadQuotedString()
	require.ErrorIs(t, err, ErrReaderExpectedStartOfQuote)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 0, rErr.Reader.Cursor)
}
func TestStringReader_ReadQuotedString_NoClose(t *testing.T) {
	r := StringReader{String: "\"hello world"}
	_, err := r.ReadQuotedString()
	require.ErrorIs(t, err, ErrReaderExpectedEndOfQuote)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 12, rErr.Reader.Cursor)
}

func TestStringReader_ReadQuotedString_InvalidEscape(t *testing.T) {
	r := StringReader{String: "\"hello\\nworld\""}
	_, err := r.ReadQuotedString()
	require.ErrorIs(t, err, ErrReaderInvalidEscape)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 7, rErr.Reader.Cursor)
}

func TestStringReader_ReadQuotedString_InvalidQuoteEscape(t *testing.T) {
	r := StringReader{String: "'hello\\\"'world"}
	_, err := r.ReadQuotedString()
	require.ErrorIs(t, err, ErrReaderInvalidEscape)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 7, rErr.Reader.Cursor)
}

func TestStringReader_ReadString_NoQuotes(t *testing.T) {
	r := StringReader{String: "hello world"}
	s, err := r.ReadString()
	require.NoError(t, err)
	require.Equal(t, "hello", s)
	require.Equal(t, " world", r.Remaining())
}

func TestStringReader_ReadString_SingleQuotes(t *testing.T) {
	r := StringReader{String: "'hello world'"}
	s, err := r.ReadString()
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadString_DoubleQuotes(t *testing.T) {
	r := StringReader{String: `"hello world"`}
	s, err := r.ReadString()
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
	require.Empty(t, r.Remaining())
}

func TestStringReader_ReadInt(t *testing.T) {
	r := StringReader{String: "1234567890"}
	i, err := r.ReadInt()
	require.NoError(t, err)
	require.Equal(t, 1234567890, i)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadInt_Negative(t *testing.T) {
	r := StringReader{String: "-1234567890"}
	i, err := r.ReadInt()
	require.NoError(t, err)
	require.Equal(t, -1234567890, i)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadInt_Invalid(t *testing.T) {
	r := StringReader{String: "12.34"}
	_, err := r.ReadInt()
	require.ErrorIs(t, err, ErrReaderInvalidInt)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 0, rErr.Reader.Cursor)
}
func TestStringReader_ReadInt_None(t *testing.T) {
	r := StringReader{}
	_, err := r.ReadInt()
	require.ErrorIs(t, err, ErrReaderExpectedInt)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 0, rErr.Reader.Cursor)
}
func TestStringReader_ReadInt_WithRemainingImmediate(t *testing.T) {
	r := StringReader{String: "1234567890foo bar"}
	i, err := r.ReadInt()
	require.NoError(t, err)
	require.Equal(t, 1234567890, i)
	require.Equal(t, "foo bar", r.Remaining())
}

func TestStringReader_ReadBool(t *testing.T) {
	r := StringReader{String: "true"}
	b, err := r.ReadBool()
	require.NoError(t, err)
	require.True(t, b)
	require.Empty(t, r.Remaining())
}
func TestStringReader_ReadBool_Incorrect(t *testing.T) {
	r := StringReader{String: "tuesday"}
	_, err := r.ReadBool()
	require.Error(t, err)
	var (
		rErr *ReaderError
		vErr *ReaderInvalidValueError
	)
	require.True(t, errors.As(err, &rErr))
	require.True(t, errors.As(err, &vErr))
	require.Equal(t, "tuesday", vErr.Value)
	require.Equal(t, 0, rErr.Reader.Cursor)
}
func TestStringReader_ReadBool_None(t *testing.T) {
	r := StringReader{}
	_, err := r.ReadBool()
	require.ErrorIs(t, err, ErrReaderExpectedBool)
	var rErr *ReaderError
	require.True(t, errors.As(err, &rErr))
	require.Equal(t, 0, rErr.Reader.Cursor)
}
