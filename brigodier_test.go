package brigodier

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDispatcher_ParseExecute(t *testing.T) {
	const cmd = `base`
	var (
		d     Dispatcher
		input string
	)
	d.Register(Literal(cmd).Executes(CommandFunc(func(c *CommandContext) error {
		input = c.Input
		return nil
	})))

	require.NoError(t, d.ParseExecute(context.TODO(), cmd))
	require.Equal(t, cmd, input)
}

func TestDispatcher_MergeCommands(t *testing.T) {
	var (
		d     Dispatcher
		times int
	)
	cmdFn := CommandFunc(func(c *CommandContext) error { times++; return nil })
	d.Register(Literal("base").Then(Literal("foo").Executes(cmdFn)))
	d.Register(Literal("base").Then(Literal("bar").Executes(cmdFn)))

	require.NoError(t, d.ParseExecute(context.TODO(), "base foo"))
	require.NoError(t, d.ParseExecute(context.TODO(), "base bar"))
	require.Equal(t, 2, times)
}

func TestDispatcher_Execute_UnknownCommand(t *testing.T) {
	var d Dispatcher
	d.Register(Literal("bar"))
	d.Register(Literal("baz"))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownCommand)
	require.Equal(t, 0, err.Reader.Cursor)
}

func TestDispatcher_Execute_UnknownSubCommand(t *testing.T) {
	var (
		d     Dispatcher
		times int
	)
	cmdFn := CommandFunc(func(c *CommandContext) error { times++; return nil })
	d.Register(Literal("foo").Executes(cmdFn))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo bar"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownArgument)
	require.Equal(t, 0, times)
	require.Equal(t, 4, err.Reader.Cursor)
}

func TestDispatcher_Execute_ImpermissibleCommand(t *testing.T) {
	var d Dispatcher
	d.Register(Literal("foo").Requires(func(context.Context) bool { return false }))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownCommand)
	require.Equal(t, 0, err.Reader.Cursor)
}

func TestDispatcher_Execute_EmptyCommand(t *testing.T) {
	var d Dispatcher
	d.Register(Literal(""))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), ""), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownCommand)
	require.Equal(t, 0, err.Reader.Cursor)
}

func TestDispatcher_Execute_IncorrectLiteral(t *testing.T) {
	var (
		d     Dispatcher
		times int
	)
	cmdFn := CommandFunc(func(c *CommandContext) error { times++; return nil })
	d.Register(Literal("foo").Executes(cmdFn).Then(Literal("bar")))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo baz"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownArgument)
	require.Equal(t, 0, times)
	require.Equal(t, 4, err.Reader.Cursor)
}

func TestDispatcher_Execute_AmbiguousIncorrectArgument(t *testing.T) {
	var d Dispatcher
	cmdFn := CommandFunc(func(c *CommandContext) error { return nil })

	d.Register(Literal("foo").Executes(cmdFn).
		Then(Literal("bar")).
		Then(Literal("baz")),
	)

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo unknown"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownArgument)
	require.Equal(t, 4, err.Reader.Cursor)
}

func TestDispatcher_Execute_Subcommand(t *testing.T) {
	var d Dispatcher
	var input string
	cmdFn := CommandFunc(func(c *CommandContext) error { input = c.Input; return nil })
	d.Register(Literal("foo").Then(
		Literal("a"),
	).Then(
		Literal("=").Executes(cmdFn),
	).Then(
		Literal("c"),
	).Executes(cmdFn))

	require.NoError(t, d.ParseExecute(context.TODO(), "foo ="))
	require.Equal(t, "foo =", input)
}

func TestDispatcher_ParseIncompleteLiteral(t *testing.T) {
	var d Dispatcher
	d.Register(Literal("foo").Then(Literal("bar")))

	parse := d.Parse(context.TODO(), "foo ")
	require.Equal(t, " ", parse.Reader.remaining())
	require.Len(t, parse.Context.Nodes, 1)
}

func TestDispatcher_ParseIncompleteArgument(t *testing.T) {
	var d Dispatcher
	d.Register(Literal("foo").Then(Argument("bar", Int)))

	parse := d.Parse(context.TODO(), "foo ")
	require.Equal(t, " ", parse.Reader.remaining())
	require.Len(t, parse.Context.Nodes, 1)
}

func TestDispatcher_Execute_AmbiguousParentSubcommandViaRedirect(t *testing.T) {
	var d Dispatcher
	var c1, c2 bool
	cmdFn := CommandFunc(func(c *CommandContext) error { c1 = true; return nil })
	subCmdFn := CommandFunc(func(c *CommandContext) error {
		c2 = true
		require.Equal(t, 1, c.Int("right"))
		require.Equal(t, 2, c.Int("sub"))
		return nil
	})

	r := d.Register(Literal("test").
		Then(
			Argument("incorrect", Int).Executes(cmdFn)).
		Then(
			Argument("right", Int).Then(
				Argument("sub", Int).Executes(subCmdFn),
			)),
	)

	d.Register(Literal("redirect").Redirect(r))

	require.NoError(t, d.ParseExecute(context.TODO(), "redirect 1 2"))
	require.False(t, c1)
	require.True(t, c2)
}

func TestDispatcher_Execute_RedirectMultipleTimes(t *testing.T) {
	var d Dispatcher
	var cmdInput string
	cmd := CommandFunc(func(c *CommandContext) error { cmdInput += c.Input; return nil })

	concreteNode := d.Register(Literal("actual").Executes(cmd))
	redirectNode := d.Register(Literal("redirected").Redirect(&d.Root))

	const input = "redirected redirected actual"

	parse := d.Parse(context.TODO(), input)
	require.Equal(t, parse.Context.Range.get(input), "redirected")
	require.Len(t, parse.Context.Nodes, 1)
	require.Equal(t, parse.Context.RootNode, &d.Root)
	require.Equal(t, parse.Context.Nodes[0].Node, redirectNode)

	child1 := parse.Context.Child
	require.NotNil(t, child1)
	require.Equal(t, child1.Range.get(input), "redirected")
	require.Len(t, child1.Nodes, 1)
	require.Equal(t, &d.Root, child1.RootNode)
	require.Equal(t, child1.Range, *child1.Nodes[0].Range)
	require.Equal(t, redirectNode, child1.Nodes[0].Node)

	child2 := child1.Child
	require.NotNil(t, child2)
	require.Equal(t, "actual", child2.Range.get(input))
	require.Len(t, child2.Nodes, 1)
	require.Equal(t, &d.Root, child2.RootNode)
	require.Equal(t, *child2.Nodes[0].Range, child2.Range)
	require.Equal(t, child2.Nodes[0].Node, concreteNode)

	require.NoError(t, d.Execute(parse))
	require.Equal(t, input, cmdInput)
}

func TestDispatcher_Execute_Redirected(t *testing.T) {
	var d Dispatcher
	var cmdInput string
	cmd := CommandFunc(func(c *CommandContext) error { cmdInput += c.Input; return nil })
	mod := ModifierFunc(func(c *CommandContext) (context.Context, error) {
		// another context
		return context.Background(), nil
	})

	concreteNode := d.Register(Literal("actual").Executes(cmd))
	redirectNode := d.Register(Literal("redirected").Fork(&d.Root, mod))

	const input = "redirected actual"
	parse := d.Parse(context.TODO(), input)
	require.Equal(t, "redirected", parse.Context.Range.get(input))
	require.Len(t, parse.Context.Nodes, 1)
	require.Equal(t, &d.Root, parse.Context.RootNode)
	require.Equal(t, parse.Context.Range, *parse.Context.Nodes[0].Range)
	require.Equal(t, redirectNode, parse.Context.Nodes[0].Node)

	parent := parse.Context.Child
	require.NotNil(t, parent)
	require.Equal(t, "actual", parent.Range.get(input))
	require.Len(t, parse.Context.Nodes, 1)
	require.Equal(t, &d.Root, parse.Context.RootNode)
	require.Equal(t, parent.Range, *parent.Nodes[0].Range)
	require.Equal(t, concreteNode, parent.Nodes[0].Node)

	require.NoError(t, d.Execute(parse))
	require.Equal(t, input, cmdInput)
}

func TestDispatcher_Execute_OrphanedSubcommand(t *testing.T) {
	var d Dispatcher
	cmd := CommandFunc(func(c *CommandContext) error { return nil })
	d.Register(Literal("foo").Then(Argument("bar", Int)).Executes(cmd))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo 5"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownCommand)
	require.Equal(t, 5, err.Reader.Cursor)
}

func TestDispatcher_Execute_invalidOther(t *testing.T) {
	var d Dispatcher
	var i int
	cmd := CommandFunc(func(c *CommandContext) error { i += 1; return nil })
	wrongCmd := CommandFunc(func(c *CommandContext) error { i -= 100; return nil })
	d.Register(Literal("w").Executes(wrongCmd))
	d.Register(Literal("world").Executes(cmd))

	require.NoError(t, d.ParseExecute(context.TODO(), "world"))
	require.Equal(t, 1, i)
}

func TestDispatcher_Execute_noSpaceSeparator(t *testing.T) {
	var d Dispatcher
	cmd := CommandFunc(func(c *CommandContext) error { return nil })
	d.Register(Literal("foo").Then(Argument("bar", Int)).Executes(cmd))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo$"), &err))
	require.ErrorIs(t, err, ErrDispatcherUnknownCommand)
	require.Equal(t, 0, err.Reader.Cursor)
}

func TestDispatcher_Execute_InvalidSubcommand(t *testing.T) {
	var d Dispatcher
	cmd := CommandFunc(func(c *CommandContext) error { return nil })
	d.Register(Literal("foo").Then(Argument("bar", Int)).Executes(cmd))

	var err *ReaderError
	require.True(t, errors.As(d.ParseExecute(context.TODO(), "foo bar"), &err))
	require.ErrorIs(t, err, ErrReaderExpectedInt)
	require.Equal(t, 4, err.Reader.Cursor)
}

func TestDispatcher_Path(t *testing.T) {
	var d Dispatcher
	bar := Literal("bar").Build()
	d.Register(Literal("foo").Then(bar))

	require.Equal(t, []string{"foo", "bar"}, d.Path(bar))
}

func TestDispatcher_FindNode(t *testing.T) {
	var d Dispatcher
	bar := Literal("bar").Build()
	d.Register(Literal("foo").Then(bar))

	require.Equal(t, bar, d.FindNode("foo", "bar"))
}

func TestDispatcher_FindNode_DoesntExist(t *testing.T) {
	var d Dispatcher
	require.Nil(t, d.FindNode("foo", "bar"))
}
