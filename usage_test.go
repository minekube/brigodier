package brigodier

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func setupUsage(d *Dispatcher) {
	cmd := CommandFunc(func(c *CommandContext) error { return nil })
	requireFalse := func(ctx context.Context) bool { return false }
	d.Register(Literal("a").Then(
		Literal("1").Then(
			Literal("i").Executes(cmd),
			Literal("ii").Executes(cmd),
		),
		Literal("2").Then(
			Literal("i").Executes(cmd),
			Literal("ii").Executes(cmd),
		),
	))
	d.Register(Literal("b").Then(Literal("1").Executes(cmd)))
	d.Register(Literal("c").Executes(cmd))
	d.Register(Literal("d").Requires(requireFalse).Executes(cmd))
	d.Register(Literal("e").Executes(cmd).Then(
		Literal("1").Executes(cmd).Then(
			Literal("i").Executes(cmd),
			Literal("ii").Executes(cmd),
		),
	))
	d.Register(Literal("f").Then(
		Literal("1").Then(
			Literal("i").Executes(cmd),
			Literal("ii").Executes(cmd).Requires(requireFalse),
		),
		Literal("2").Then(
			Literal("i").Executes(cmd).Requires(requireFalse),
			Literal("ii").Executes(cmd),
		),
	))
	d.Register(Literal("g").Executes(cmd).Then(
		Literal("1").Then(Literal("i").Executes(cmd)),
	))
	d.Register(Literal("h").Executes(cmd).Then(
		Literal("1").Then(Literal("i").Executes(cmd)),
		Literal("2").Then(Literal("i").Then(Literal("ii").Executes(cmd))),
		Literal("3").Executes(cmd),
	))
	d.Register(Literal("i").Executes(cmd).Then(
		Literal("1").Executes(cmd),
		Literal("2").Executes(cmd),
	))
	d.Register(Literal("j").Redirect(&d.Root))
	d.Register(Literal("k").Redirect(get(d, "h")))
}

func get(d *Dispatcher, command string) CommandNode {
	for _, n := range d.Parse(context.TODO(), command).Context.Nodes {
		return n.Node
	}
	return nil
}
func getR(d *Dispatcher, command *StringReader) CommandNode {
	for _, n := range d.ParseReader(context.TODO(), command).Context.Nodes {
		return n.Node
	}
	return nil
}

func TestDispatcher_AllUsage_NoCommands(t *testing.T) {
	var d Dispatcher
	results := d.AllUsage(context.TODO(), &d.Root, true)
	require.Empty(t, results)
}

func TestDispatcher_SmartUsage_NoCommands(t *testing.T) {
	var d Dispatcher
	results := d.SmartUsage(context.TODO(), &d.Root)
	require.True(t, results.Empty())
}

func TestDispatcher_AllUsage_Root(t *testing.T) {
	var d Dispatcher
	setupUsage(&d)
	results := d.AllUsage(context.TODO(), &d.Root, true)
	usages := []string{
		"a 1 i",
		"a 1 ii",
		"a 2 i",
		"a 2 ii",
		"b 1",
		"c",
		"e",
		"e 1",
		"e 1 i",
		"e 1 ii",
		"f 1 i",
		"f 2 ii",
		"g",
		"g 1 i",
		"h",
		"h 1 i",
		"h 2 i ii",
		"h 3",
		"i",
		"i 1",
		"i 2",
		"j ...",
		"k -> h",
	}
	for _, s := range usages {
		require.Contains(t, results, s)
	}
	require.Equal(t, len(usages), len(results))
}

type expectedSmartUsage struct {
	node  CommandNode
	usage string
}

func testSmartUsage(t testing.TB, results CommandNodeStringMap, expected ...expectedSmartUsage) {
	t.Helper()

	require.Equal(t, len(expected), results.Size())

	var i int
	results.Range(func(key CommandNode, value string) bool {
		require.Equal(t, expected[i].usage, value, "%s is not %s", expected[i].usage, value)
		require.NotNil(t, expected[i].node)
		i++
		return true
	})
}

func TestDispatcher_SmartUsage_Root(t *testing.T) {
	d := new(Dispatcher)
	setupUsage(d)
	results := d.SmartUsage(context.TODO(), &d.Root)

	testSmartUsage(t, results, []expectedSmartUsage{
		{get(d, "a"), "a (1|2)"},
		{get(d, "b"), "b 1"},
		{get(d, "c"), "c"},
		{get(d, "e"), "e [1]"},
		{get(d, "f"), "f (1|2)"},
		{get(d, "g"), "g [1]"},
		{get(d, "h"), "h [1|2|3]"},
		{get(d, "i"), "i [1|2]"},
		{get(d, "j"), "j ..."},
		{get(d, "k"), "k -> h"},
	}...)
}

func TestDispatcher_SmartUsage_H(t *testing.T) {
	d := new(Dispatcher)
	setupUsage(d)
	results := d.SmartUsage(context.TODO(), get(d, "h"))

	testSmartUsage(t, results, []expectedSmartUsage{
		{get(d, "h 1"), "[1] i"},
		{get(d, "h 2"), "[2] i ii"},
		{get(d, "h 3"), "[3]"},
	}...)
}

func TestDispatcher_SmartUsage_OffsetH(t *testing.T) {
	d := new(Dispatcher)
	setupUsage(d)
	results := d.SmartUsage(context.TODO(), getR(d, inputWithOffset("/|/|/h", 5)))

	testSmartUsage(t, results, []expectedSmartUsage{
		{get(d, "h 1"), "[1] i"},
		{get(d, "h 2"), "[2] i ii"},
		{get(d, "h 3"), "[3]"},
	}...)
}
