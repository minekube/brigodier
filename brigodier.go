package brigodier

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

const (
	// ArgumentSeparator is the default string required
	// to separate individual arguments in an input string.
	ArgumentSeparator rune = ' '
)

type Dispatcher struct {
	Root RootCommandNode
}

func (d *Dispatcher) AllUsage(ctx context.Context, node CommandNode, restricted bool) []string {
	return d.allUsage(ctx, node, nil, "", restricted)
}
func (d *Dispatcher) allUsage(ctx context.Context, node CommandNode, result []string, prefix string, restricted bool) []string {
	if restricted && !node.CanUse(ctx) {
		return result
	}
	if node.Command() != nil {
		result = append(result, prefix)
	}
	if node.Redirect() != nil {
		var add, redirect string
		if node.Redirect() == &d.Root {
			redirect = "..."
		} else {
			redirect = "-> " + node.Redirect().UsageText()
		}
		if prefix == "" {
			add = fmt.Sprintf("%s%c%s", node.UsageText(), ArgumentSeparator, redirect)
		} else {
			add = fmt.Sprintf("%s%c%s", prefix, ArgumentSeparator, redirect)
		}
		result = append(result, add)
	} else { // if len(node.Children()) != 0
		for _, child := range node.Children() {
			var p string
			if prefix == "" {
				p = child.UsageText()
			} else {
				p = fmt.Sprintf("%s%c%s", prefix, ArgumentSeparator, child.UsageText())
			}
			result = d.allUsage(ctx, child, result, p, restricted)
		}
	}
	return result
}

func (d *Dispatcher) Register(command *LiteralArgumentBuilder) *LiteralCommandNode {
	b := command.Build()
	d.Root.AddChild(b)
	return b
}

type RequireFn func(context.Context) bool

var (
	ErrDispatcherUnknownCommand  = errors.New("dispatcher: unknown command")
	ErrDispatcherUnknownArgument = errors.New("dispatcher: unknown argument")
)

func (d *Dispatcher) Execute(parse *ParseResults) error {
	if parse.Reader.CanRead() {
		if len(parse.Errs) == 1 {
			return parse.firstErr()
		} else if parse.Context.Range.IsEmpty() {
			return &CommandSyntaxError{Err: &ReaderError{
				Err:    ErrDispatcherUnknownCommand,
				Reader: parse.Reader,
			}}
		} else {
			return &CommandSyntaxError{Err: &ReaderError{
				Err:    ErrDispatcherUnknownArgument,
				Reader: parse.Reader,
			}}
		}
	}

	forked := false
	foundCommand := false
	original := parse.Context.build(parse.Reader.String)
	contexts := []*CommandContext{original}
	var next []*CommandContext

	var err error
	for contexts != nil {
		size := len(contexts)
		for i := 0; i < size; i++ {
			theContext := contexts[i]
			child := theContext.Child
			if child != nil {
				forked = forked || theContext.Forks
				if child.HasNodes() {
					foundCommand = true
					modifier := theContext.Modifier
					if modifier == nil {
						next = append(next, child.CopyFor(theContext))
					} else {
						result, err := modifier.Apply(theContext)
						if err != nil {
							if !forked {
								return err
							}
						} else {
							next = append(next, child.CopyFor(result))
						}
					}
				}
			} else if theContext.Command != nil {
				foundCommand = true
				if err = theContext.Command.Run(theContext); err != nil && !forked {
					return err
				}
			}
		}

		contexts = next
		next = nil
	}

	if !foundCommand {
		return &CommandSyntaxError{Err: &ReaderError{
			Err:    ErrDispatcherUnknownCommand,
			Reader: parse.Reader,
		}}
	}
	return nil
}

type ParseResults struct {
	Context *CommandContext
	Reader  *StringReader
	Errs    map[CommandNode]error
}

func (r *ParseResults) firstErr() error {
	for _, err := range r.Errs {
		return err
	}
	return nil
}

type ParsedCommandNode struct {
	Node  CommandNode
	Range *StringRange
}

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

func (c *CommandContext) HasNodes() bool { return len(c.Nodes) != 0 }

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

func (c *CommandContext) Int(argumentName string) int {
	if c.Arguments == nil {
		return 0
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return 0
	}
	v, _ := r.Result.(int)
	return v
}
func (c *CommandContext) Bool(argumentName string) bool {
	if c.Arguments == nil {
		return false
	}
	r, ok := c.Arguments[argumentName]
	if !ok {
		return false
	}
	v, _ := r.Result.(bool)
	return v
}

func (c *CommandContext) CopyFor(ctx context.Context) *CommandContext {
	if c.Context == ctx {
		return c
	}
	clone := c.Copy()
	clone.Context = ctx
	return clone
}

type RedirectModifier interface {
	Apply(ctx *CommandContext) (context.Context, error)
}

type ModifierFunc func(c *CommandContext) (context.Context, error)

func (m ModifierFunc) Apply(c *CommandContext) (context.Context, error) { return m(c) }

type CommandSyntaxError struct {
	Err error
}

func (e *CommandSyntaxError) Unwrap() error { return e.Err }
func (e *CommandSyntaxError) Error() string {
	return e.Err.Error()
}

func (d *Dispatcher) ParseExecute(ctx context.Context, command string) error {
	return d.Execute(d.Parse(ctx, command))
}

func (d *Dispatcher) Parse(ctx context.Context, command string) *ParseResults {
	return d.parseNodes(&StringReader{String: command}, &d.Root, &CommandContext{
		Context:  ctx,
		RootNode: &d.Root,
	})
}

var ErrDispatcherExpectedArgumentSeparator = errors.New("dispatcher expected argument separator")

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

func (d *Dispatcher) Path(target CommandNode) []string {
	var nodes [][]CommandNode
	d.addPaths(&d.Root, &nodes, &[]CommandNode{})
	for _, list := range nodes {
		if list[len(list)-1] == target {
			var result []string
			for _, node := range list {
				if node != &d.Root {
					result = append(result, node.Name())
				}
			}
			return result
		}
	}
	return nil
}

func (d *Dispatcher) addPaths(node CommandNode, result *[][]CommandNode, parents *[]CommandNode) {
	current := append([]CommandNode{}, *parents...) // copy
	current = append(current, node)
	*result = append(*result, current)
	for _, child := range node.Children() {
		d.addPaths(child, result, &current)
	}
}

func (d *Dispatcher) FindNode(path ...string) CommandNode {
	var node CommandNode = &d.Root
	for _, name := range path {
		node = node.Children()[name]
		if node == nil {
			return nil
		}
	}
	return node
}

func (n *Node) AddChild(nodes ...CommandNode) {
	for _, node := range nodes {
		if _, ok := node.(*RootCommandNode); ok {
			continue
		}

		child := n.Children()[node.Name()]
		if child != nil {
			// We've found something to merge onto
			if node.Command() != nil {
				child.setCommand(node.Command())
			}
			for _, grandchild := range node.Children() {
				child.AddChild(grandchild)
			}
		} else {
			n.children[node.Name()] = node
			switch t := node.(type) {
			case *LiteralCommandNode:
				n.Literals()[node.Name()] = t
			case *ArgumentCommandNode:
				n.Arguments()[node.Name()] = t
			}
		}
	}
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

type Command interface {
	Run(c *CommandContext) error
}

type CommandFunc func(c *CommandContext) error

func (cf CommandFunc) Run(c *CommandContext) error { return cf(c) }

type CommandNode interface {
	Arguments() map[string]*ArgumentCommandNode
	Literals() map[string]*LiteralCommandNode
	CanUse(ctx context.Context) bool
	RelevantNodes(input *StringReader) []CommandNode
	Parse(ctx *CommandContext, rd *StringReader) error
	Redirect() CommandNode
	Command() Command
	RedirectModifier() RedirectModifier
	IsFork() bool
	Name() string
	Children() map[string]CommandNode
	setCommand(Command)
	AddChild(nodes ...CommandNode)
	UsageText() string
}

type Node struct {
	children    map[string]CommandNode
	literals    map[string]*LiteralCommandNode
	arguments   map[string]*ArgumentCommandNode
	requirement RequireFn
	redirect    CommandNode
	command     Command
	modifier    RedirectModifier
	forks       bool
}

func (n *Node) RedirectModifier() RedirectModifier { return n.modifier }

func (n *Node) Children() map[string]CommandNode {
	if n.children == nil {
		n.children = map[string]CommandNode{}
	}
	return n.children
}
func (n *Node) IsFork() bool { return n.forks }

func (n *Node) Literals() map[string]*LiteralCommandNode {
	if n.literals == nil {
		n.literals = map[string]*LiteralCommandNode{}
	}
	return n.literals
}
func (n *Node) Arguments() map[string]*ArgumentCommandNode {
	if n.arguments == nil {
		n.arguments = map[string]*ArgumentCommandNode{}
	}
	return n.arguments
}

func (n *Node) CanUse(ctx context.Context) bool {
	if n.requirement == nil {
		return true
	}
	return n.requirement(ctx)
}

func (n *Node) Redirect() CommandNode { return n.redirect }
func (n *Node) Command() Command      { return n.command }
func (n *Node) setCommand(c Command)  { n.command = c }

type RootCommandNode struct {
	Node
}

func (r *RootCommandNode) String() string                             { return "<root>" }
func (r *RootCommandNode) Name() string                               { return "" }
func (r *RootCommandNode) UsageText() string                          { return "" }
func (r *RootCommandNode) Parse(*CommandContext, *StringReader) error { return nil }

type LiteralCommandNode struct {
	Node
	Literal string
}

type IncorrectLiteralError struct {
	Literal string
}

func (e *IncorrectLiteralError) Error() string { return fmt.Sprintf("incorrect literal %q", e.Literal) }

func (n *LiteralCommandNode) String() string    { return fmt.Sprintf("<literal %s>", n.Literal) }
func (n *LiteralCommandNode) Name() string      { return n.Literal }
func (n *LiteralCommandNode) UsageText() string { return n.Literal }
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

type ArgumentCommandNode struct {
	Node
	name    string
	argType ArgumentType
}

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

func (a *ArgumentCommandNode) String() string {
	return fmt.Sprintf("<argument %s:%s>", a.name, a.argType)
}
func (a *ArgumentCommandNode) Name() string       { return a.name }
func (a *ArgumentCommandNode) Type() ArgumentType { return a.argType }

const (
	UsageArgumentOpen  rune = '['
	UsageArgumentClose rune = ']'
)

func (a *ArgumentCommandNode) UsageText() string {
	return fmt.Sprintf("%c%s%c", UsageArgumentOpen, a.name, UsageArgumentClose)
}

type ParsedArgument struct {
	Range  *StringRange
	Result interface{}
}

type LiteralArgument struct {
	Node
}
