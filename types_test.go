package brigodier

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStringType_Parse(t *testing.T) {
	r := &StringReader{String: `"hello world"`}
	s, err := String.Parse(r)
	require.NoError(t, err)
	require.Equal(t, "hello world", s)
	require.Equal(t, "", r.Remaining())

	r = &StringReader{String: `hello world`}
	s, err = String.Parse(r)
	require.NoError(t, err)
	require.Equal(t, "hello", s)
	require.Equal(t, " world", r.Remaining())
}
func TestStringType_Parse_Word(t *testing.T) {
	r := &StringReader{String: "hello world"}
	s, err := StringWord.Parse(r)
	require.NoError(t, err)
	require.Equal(t, "hello", s)
	require.Equal(t, " world", r.Remaining())
}
func TestStringType_Parse_Phrase(t *testing.T) {
	r := &StringReader{String: "Hello world! This is a test."}
	s, err := StringPhrase.Parse(r)
	require.NoError(t, err)
	require.Equal(t, "Hello world! This is a test.", s)
}

func TestBoolType_Parse(t *testing.T) {
	parse, err := Bool.Parse(&StringReader{String: "true"})
	require.NoError(t, err)
	require.Equal(t, true, parse)

	parse, err = Bool.Parse(&StringReader{String: "false"})
	require.NoError(t, err)
	require.Equal(t, false, parse)
}
