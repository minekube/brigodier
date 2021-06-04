package brigodier

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func testSuggestions(t testing.TB,
	d *Dispatcher,
	contents string,
	cursor int,
	strRange StringRange,
	suggestions ...string,
) {
	t.Helper()

	result, err := d.CompletionSuggestionsCursor(d.Parse(context.TODO(), contents), cursor)
	require.NoError(t, err)
	require.Equal(t, strRange, result.Range)

	for _, s := range result.Suggestions {
		require.Contains(t, suggestions, s.Text)
	}
	require.Len(t, result.Suggestions, len(suggestions))
}

func inputWithOffset(input string, offset int) *StringReader {
	return &StringReader{String: input, Cursor: offset}
}

func TestDispatcher_CompletionSuggestions_RootCommands(t *testing.T) {
	var d Dispatcher
	literals := []string{"foo", "bar", "baz"}
	for _, l := range literals {
		d.Register(Literal(l))
	}

	result, err := d.CompletionSuggestions(d.Parse(context.TODO(), ""))
	require.NoError(t, err)

	require.Len(t, result.Suggestions, len(literals))
	zeroRange := StringRange{}
	require.Equal(t, zeroRange, result.Range)
	for i, l := range literals {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, zeroRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_RootCommands_WithInputOffset(t *testing.T) {
	var d Dispatcher
	literals := []string{"foo", "bar", "baz"}
	for _, l := range literals {
		d.Register(Literal(l))
	}

	result, err := d.CompletionSuggestions(d.ParseReader(context.TODO(), inputWithOffset("000", 3)))
	require.NoError(t, err)

	require.Len(t, result.Suggestions, len(literals))
	expectedRange := StringRange{Start: 3, End: 3}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range literals {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_RootCommands_Partial(t *testing.T) {
	var d Dispatcher
	literals := []string{"foo", "bar", "baz"}
	for _, l := range literals {
		d.Register(Literal(l))
	}

	result, err := d.CompletionSuggestions(d.Parse(context.TODO(), "b"))
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 2)
	expectedRange := StringRange{Start: 0, End: 1}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"bar", "baz"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_RootCommands_Partial_WithInputOffset(t *testing.T) {
	var d Dispatcher
	literals := []string{"foo", "bar", "baz"}
	for _, l := range literals {
		d.Register(Literal(l))
	}

	result, err := d.CompletionSuggestions(d.ParseReader(context.TODO(), inputWithOffset("Zb", 1)))
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 2)
	expectedRange := StringRange{Start: 1, End: 2}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"bar", "baz"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_SubCommands(t *testing.T) {
	var d Dispatcher
	d.Register(Literal("parent").Then(
		Literal("foo"),
		Literal("bar"),
		Literal("baz"),
	))

	result, err := d.CompletionSuggestions(d.Parse(context.TODO(), "parent "))
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 3)
	expectedRange := StringRange{Start: 7, End: 7}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"foo", "bar", "baz"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_MovingCursor_SubCommands(t *testing.T) {
	d := new(Dispatcher)
	d.Register(Literal("parent_one").Then(
		Literal("faz"),
		Literal("fbz"),
		Literal("gaz"),
	))

	d.Register(Literal("parent_two"))

	testSuggestions(t, d, "parent_one faz ", 0, StringRange{}, "parent_one", "parent_two")
	testSuggestions(t, d, "parent_one faz ", 1, StringRange{0, 1}, "parent_one", "parent_two")
	testSuggestions(t, d, "parent_one faz ", 7, StringRange{0, 7}, "parent_one", "parent_two")
	testSuggestions(t, d, "parent_one faz ", 8, StringRange{0, 8}, "parent_one")
	testSuggestions(t, d, "parent_one faz ", 10, StringRange{})
	testSuggestions(t, d, "parent_one faz ", 11, StringRange{11, 11}, "faz", "fbz", "gaz")
	testSuggestions(t, d, "parent_one faz ", 12, StringRange{11, 12}, "faz", "fbz")
	testSuggestions(t, d, "parent_one faz ", 13, StringRange{11, 13}, "faz")
	testSuggestions(t, d, "parent_one faz ", 14, StringRange{})
	testSuggestions(t, d, "parent_one faz ", 15, StringRange{})
}

func TestDispatcher_CompletionSuggestions_SubCommands_Partial(t *testing.T) {
	var d Dispatcher
	parent := Literal("parent")
	literals := []string{"foo", "bar", "baz"}
	for _, l := range literals {
		parent.Then(Literal(l))
	}
	d.Register(parent)

	result, err := d.CompletionSuggestions(d.Parse(context.TODO(), "parent b"))
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 2)
	expectedRange := StringRange{Start: 7, End: 8}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"bar", "baz"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_SubCommands_Partial_WithInputOffset(t *testing.T) {
	var d Dispatcher
	parent := Literal("parent")
	literals := []string{"foo", "bar", "baz"}
	for _, l := range literals {
		parent.Then(Literal(l))
	}
	d.Register(parent)

	parse := d.ParseReader(context.TODO(), inputWithOffset("junk parent b", 5))
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 2)
	expectedRange := StringRange{Start: 12, End: 13}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"bar", "baz"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_Redirect(t *testing.T) {
	var d Dispatcher
	actual := d.Register(Literal("actual").Then(Literal("sub")))
	d.Register(Literal("redirect").Redirect(actual))

	parse := d.Parse(context.TODO(), "redirect ")
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 1)
	expectedRange := StringRange{Start: 9, End: 9}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"sub"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}
func TestDispatcher_CompletionSuggestions_Redirect_Partial(t *testing.T) {
	var d Dispatcher
	actual := d.Register(Literal("actual").Then(Literal("sub")))
	d.Register(Literal("redirect").Redirect(actual))

	parse := d.Parse(context.TODO(), "redirect s")
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 1)
	expectedRange := StringRange{Start: 9, End: 10}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"sub"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_MovingCursor_Redirect(t *testing.T) {
	d := new(Dispatcher)
	actualOne := d.Register(Literal("actual_one").Then(
		Literal("faz"),
		Literal("fbz"),
		Literal("gaz"),
	))
	d.Register(Literal("actual_two"))

	d.Register(Literal("redirect_one").Redirect(actualOne))
	d.Register(Literal("redirect_two").Redirect(actualOne))

	testSuggestions(t, d, "redirect_one faz ", 0, StringRange{}, "actual_one", "actual_two", "redirect_one", "redirect_two")
	testSuggestions(t, d, "redirect_one faz ", 9, StringRange{0, 9}, "redirect_one", "redirect_two")
	testSuggestions(t, d, "redirect_one faz ", 10, StringRange{0, 10}, "redirect_one")
	testSuggestions(t, d, "redirect_one faz ", 12, StringRange{})
	testSuggestions(t, d, "redirect_one faz ", 13, StringRange{13, 13}, "faz", "fbz", "gaz")
	testSuggestions(t, d, "redirect_one faz ", 14, StringRange{13, 14}, "faz", "fbz")
	testSuggestions(t, d, "redirect_one faz ", 15, StringRange{13, 15}, "faz")
	testSuggestions(t, d, "redirect_one faz ", 16, StringRange{})
	testSuggestions(t, d, "redirect_one faz ", 17, StringRange{})
}

func TestDispatcher_CompletionSuggestions_Redirect_Partial_WithInputOffset(t *testing.T) {
	var d Dispatcher
	actual := d.Register(Literal("actual").Then(Literal("sub")))
	d.Register(Literal("redirect").Redirect(actual))

	parse := d.ParseReader(context.TODO(), inputWithOffset("/redirect s", 1))
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 1)
	expectedRange := StringRange{Start: 10, End: 11}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"sub"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_Redirect_Lots(t *testing.T) {
	var d Dispatcher
	loop := d.Register(Literal("redirect"))
	d.Register(Literal("redirect").Then(Literal("loop").Then(Argument("loop", Int).Redirect(loop))))

	parse := d.Parse(context.TODO(), "redirect loop 1 loop 02 loop 003 ")
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 1)
	expectedRange := StringRange{Start: 33, End: 33}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"loop"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}

func TestDispatcher_CompletionSuggestions_Execute_Simulation(t *testing.T) {
	var d Dispatcher
	execute := d.Register(Literal("execute"))
	d.Register(Literal("execute").Then(
		Literal("as").Then(Argument("name", StringWord).Redirect(execute)),
		Literal("store").Then(Argument("name", StringWord).Redirect(execute)),
		Literal("run").Executes(CommandFunc(func(c *CommandContext) error { return nil })),
	))

	parse := d.Parse(context.TODO(), "execute as Dinnerbone as")
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)
	require.Empty(t, result.Suggestions)
}

func TestDispatcher_CompletionSuggestions_Execute_Simulation_Partial(t *testing.T) {
	var d Dispatcher
	execute := d.Register(Literal("execute"))
	d.Register(Literal("execute").Then(
		Literal("as").Then(
			Literal("bar").Redirect(execute),
			Literal("baz").Redirect(execute),
		),
		Literal("store").Then(Argument("name", StringWord).Redirect(execute)),
		Literal("run").Executes(CommandFunc(func(c *CommandContext) error { return nil })),
	))

	parse := d.Parse(context.TODO(), "execute as bar as ")
	result, err := d.CompletionSuggestions(parse)
	require.NoError(t, err)

	require.Len(t, result.Suggestions, 2)
	expectedRange := StringRange{Start: 18, End: 18}
	require.Equal(t, expectedRange, result.Range)
	for i, l := range []string{"bar", "baz"} {
		require.Equal(t, l, result.Suggestions[i].Text)
		require.Equal(t, expectedRange, result.Suggestions[i].Range)
	}
}
