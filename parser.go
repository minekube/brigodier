package brigodier

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Parse parses a given command.
//
// The result of this method can be cached, and it is advised to do so where appropriate. Parsing is often the
// most expensive step, and this allows you to essentially "precompile" a command if it will be ran often.
//
// If the command passes through a node that is CommandNode.IsFork then the resulting context will be marked as 'forked'.
// Forked contexts may contain child contexts, which may be modified by the RedirectModifier attached to the fork.
//
// Parsing a command can never fail, you will always be provided with a new ParseResults.
// However, that does not mean that it will always parse into a valid command. You should inspect the returned results
// to check for validity. If its ParseResults.Reader StringReader.CanRead then it did not finish parsing successfully.
// You can use that position as an indicator to the user where the command stopped being valid.
// You may inspect ParseResults.Errs if you know the parse failed, as it will explain why it could
// not find any valid commands. It may contain multiple errors, one for each "potential node" that it could have visited,
// explaining why it did not go down that node.
//
// When you eventually call Execute(ParseResults) with the result of this method, the above error checking
// will occur. You only need to inspect it yourself if you wish to handle that yourself.
func (d *Dispatcher) Parse(ctx context.Context, command string) *ParseResults {
	return d.ParseReader(ctx, &StringReader{String: command})
}

// ParseReader parses a given command within a reader and optional StringReader.Cursor offset.
//
// See Parse for more details.
func (d *Dispatcher) ParseReader(ctx context.Context, command *StringReader) *ParseResults {
	return d.parseNodes(command, &d.Root, &CommandContext{
		Context:  ctx,
		RootNode: &d.Root,
		Range:    StringRange{Start: command.Cursor, End: command.Cursor},
		cursor:   command.Cursor,
	})
}

// ParseResults stores the parse results returned by Dispatcher.Parse.
type ParseResults struct {
	Context *CommandContext
	Reader  *StringReader
	Errs    map[CommandNode]error
}

// CommandContext is the context for executing a command.
type CommandContext struct {
	context.Context
	Arguments map[string]*ParsedArgument
	RootNode  CommandNode
	Child     *CommandContext
	Command   Command
	Nodes     []*ParsedCommandNode
	Range     StringRange
	Modifier  RedirectModifier
	Forks     bool
	Input     string

	cursor int
}

func (c *CommandContext) build(input string) *CommandContext {
	var child *CommandContext
	if c.Child != nil {
		child = c.Child.build(input)
	}
	return &CommandContext{
		Context:   c.Context,
		Input:     input,
		Arguments: c.Arguments,
		Command:   c.Command,
		RootNode:  c.RootNode,
		Nodes:     c.Nodes,
		Range:     c.Range,
		Child:     child,
		Modifier:  c.Modifier,
		Forks:     c.Forks,
	}
}

// HasNodes indicates whether the command context has at least one ParsedCommandNode.
func (c *CommandContext) HasNodes() bool { return len(c.Nodes) != 0 }

// Copy copies the CommandContext.
func (c *CommandContext) Copy() *CommandContext {
	return &CommandContext{
		Context: c.Context,
		Arguments: func() map[string]*ParsedArgument {
			m := make(map[string]*ParsedArgument, len(c.Arguments))
			for k, v := range c.Arguments {
				m[k] = v
			}
			return m
		}(),
		RootNode: c.RootNode,
		Child:    c.Child,
		Command:  c.Command,
		Nodes:    append(make([]*ParsedCommandNode, 0, len(c.Nodes)), c.Nodes...),
		Range:    c.Range.Copy(),
		Modifier: c.Modifier,
		Forks:    c.Forks,
		Input:    c.Input,
		cursor:   c.cursor,
	}
}

// CopyFor copies the CommandContext if ctx is not equal to
// CommandContext.Context and sets CommandContext.Context to ctx.
func (c *CommandContext) CopyFor(ctx context.Context) *CommandContext {
	if c.Context == ctx || ctx == nil {
		return c
	}
	clone := c.Copy()
	clone.Context = ctx
	return clone
}

// ParsedCommandNode is a parsed command node.
type ParsedCommandNode struct {
	Node  CommandNode
	Range *StringRange
}

// ArgumentSeparator separates individual arguments in a command input string.
const ArgumentSeparator rune = ' '

// ErrDispatcherExpectedArgumentSeparator occurs when the dispatcher expected an ArgumentSeparator.
var ErrDispatcherExpectedArgumentSeparator = errors.New("dispatcher: expected argument separator")

// CommandSyntaxError is a syntax error returned on parse error.
type CommandSyntaxError struct{ Err error }

// Unwrap implements errors.Unwrap.
func (e *CommandSyntaxError) Unwrap() error { return e.Err }
func (e *CommandSyntaxError) Error() string {
	return e.Err.Error()
}

func (d *Dispatcher) parseNodes(originalReader *StringReader, node CommandNode, ctxSoFar *CommandContext) *ParseResults {
	errs := map[CommandNode]error{}
	var potentials []*ParseResults
	cursor := originalReader.Cursor

	var (
		err error
		ctx *CommandContext
		rd  *StringReader
	)
	for _, child := range node.RelevantNodes(originalReader) {
		if !child.CanUse(ctxSoFar) {
			continue
		}
		ctx = ctxSoFar.Copy()
		rd = &StringReader{
			Cursor: originalReader.Cursor,
			String: originalReader.String,
		}

		err = child.Parse(ctx, rd)
		if err == nil && rd.CanRead() && rd.Peek() != ArgumentSeparator {
			err = &CommandSyntaxError{Err: &ReaderError{
				Err:    ErrDispatcherExpectedArgumentSeparator,
				Reader: rd,
			}}
		}
		if err != nil {
			errs[child] = err
			rd.Cursor = cursor
			continue
		}

		ctx.Command = child.Command()
		redirect := child.Redirect()
		canRead := 1
		if redirect == nil {
			canRead = 2
		}
		if rd.CanReadLen(canRead) {
			rd.Skip()
			if redirect != nil {
				childCtx := &CommandContext{
					Context:  ctx,
					RootNode: redirect,
					cursor:   rd.Cursor,
					Range: StringRange{
						Start: rd.Cursor,
						End:   rd.Cursor,
					},
				}
				parse := d.parseNodes(rd, redirect, childCtx)
				ctx.Child = parse.Context
				return &ParseResults{
					Context: ctx,
					Reader:  parse.Reader,
					Errs:    parse.Errs,
				}
			}
			potentials = append(potentials, d.parseNodes(rd, child, ctx))
		} else {
			potentials = append(potentials, &ParseResults{
				Context: ctx,
				Reader:  rd,
			})
		}
	}

	if len(potentials) != 0 {
		if len(potentials) > 1 {
			sort.Slice(potentials, func(i, j int) bool {
				a := potentials[i]
				b := potentials[j]
				if !a.Reader.CanRead() && b.Reader.CanRead() {
					return true
				}
				if a.Reader.CanRead() && !b.Reader.CanRead() {
					return false
				}
				if len(a.Errs) == 0 && len(b.Errs) != 0 {
					return false
				}
				if len(a.Errs) != 0 && len(b.Errs) == 0 {
					return true
				}
				return false
			})
		}
		return potentials[0]
	}

	return &ParseResults{
		Context: ctxSoFar,
		Reader:  originalReader,
		Errs:    errs,
	}
}

func (r *ParseResults) firstErr() error {
	for _, err := range r.Errs {
		return err
	}
	return nil
}

func (n *Node) RelevantNodes(input *StringReader) []CommandNode {
	if len(n.literals) != 0 {
		cursor := input.Cursor
		for input.CanRead() && input.Peek() != ArgumentSeparator {
			input.Skip()
		}
		text := input.String[cursor:input.Cursor]
		input.Cursor = cursor
		literal, ok := n.literals[text]
		if ok {
			return []CommandNode{literal}
		}
	}
	nodes := make([]CommandNode, 0, len(n.arguments))
	for _, a := range n.arguments {
		nodes = append(nodes, a)
	}
	return nodes
}

// IncorrectLiteralError is used to indicate an incorrect literal parse error.
type IncorrectLiteralError struct {
	Literal string // The incorrect literal value.
}

func (e *IncorrectLiteralError) Error() string { return fmt.Sprintf("incorrect literal %q", e.Literal) }

// Parse parses the literal from an input reader.
func (n *LiteralCommandNode) Parse(ctx *CommandContext, rd *StringReader) error {
	start := rd.Cursor
	end := n.parse(rd)
	if end <= -1 {
		return &CommandSyntaxError{Err: &ReaderError{
			Err:    &IncorrectLiteralError{Literal: n.Literal},
			Reader: rd,
		}}
	}
	ctx.withNode(n, &StringRange{Start: start, End: end})
	return nil
}

func (n *LiteralCommandNode) parse(rd *StringReader) int {
	start := rd.Cursor
	if rd.CanReadLen(len(n.Literal)) {
		end := start + len(n.Literal)
		if rd.String[start:end] == n.Literal {
			rd.Cursor = end
			if !rd.CanRead() || rd.Peek() == ArgumentSeparator {
				return end
			}
			rd.Cursor = start
		}
	}
	return -1
}

// ParsedArgument is a parsed argument.
type ParsedArgument struct {
	Range  *StringRange // The string range of the argument in the command input.
	Result interface{}  // The parsed result value.
}

// Parse parses the argument from an input reader.
func (a *ArgumentCommandNode) Parse(ctx *CommandContext, rd *StringReader) error {
	start := rd.Cursor
	result, err := a.argType.Parse(rd)
	if err != nil {
		return fmt.Errorf("error parsing argument: %w", err)
	}
	parsed := &ParsedArgument{
		Range:  &StringRange{Start: start, End: rd.Cursor},
		Result: result,
	}
	ctx.withArgument(a.name, parsed)
	ctx.withNode(a, parsed.Range)
	return nil
}

func (c *CommandContext) withNode(node CommandNode, r *StringRange) {
	c.Nodes = append(c.Nodes, &ParsedCommandNode{
		Node:  node,
		Range: r,
	})
	c.Range = *EncompassingRange(&c.Range, r)
	c.Modifier = node.RedirectModifier()
	c.Forks = node.IsFork()
}

func (c *CommandContext) withArgument(name string, parsed *ParsedArgument) {
	if c.Arguments == nil {
		c.Arguments = map[string]*ParsedArgument{}
	}
	c.Arguments[name] = parsed
}
