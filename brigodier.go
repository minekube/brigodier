package brigodier

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
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
		var redirect string
		if node.Redirect() == &d.Root {
			redirect = "..."
		} else {
			redirect = "-> " + node.Redirect().UsageText()
			var add string
			if prefix == "" {
				add = fmt.Sprintf("%s%c%s", node.UsageText(), ArgumentSeparator, redirect)
			} else {
				add = fmt.Sprintf("%s%c%s", prefix, ArgumentSeparator, redirect)
			}
			result = append(result, add)
		}
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

func (d *Dispatcher) Register(commands ...*LiteralArgumentBuilder) {
	for _, c := range commands {
		d.Root.AddChild(c.build())
	}
}

func Literal(literal string) *LiteralArgumentBuilder {
	return &LiteralArgumentBuilder{Literal: literal}
}

type LiteralArgumentBuilder struct {
	Literal string
	ArgumentBuilder
}

func (b *LiteralArgumentBuilder) Then(argument interface{ build() CommandNode }) *LiteralArgumentBuilder {
	b.Arguments.AddChild(argument.build())
	return b
}
func (b *LiteralArgumentBuilder) build() CommandNode {
	return &LiteralCommandNode{
		Node:    *b.ArgumentBuilder.build(),
		Literal: b.Literal,
	}
}

type RequiredArgumentBuilder struct {
	ArgumentBuilder

	Name string
	Type ArgumentType
}

func (b *RequiredArgumentBuilder) build() CommandNode {
	return &ArgumentCommandNode{
		Node:    *b.ArgumentBuilder.build(),
		name:    b.Name,
		argType: b.Type,
	}
}

func Argument(name string, argType ArgumentType) *RequiredArgumentBuilder {
	return &RequiredArgumentBuilder{Name: name, Type: argType}
}
func (b *LiteralArgumentBuilder) Executes(command Command) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Executes(command)
	return b
}
func (b *RequiredArgumentBuilder) Executes(command Command) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Executes(command)
	return b
}
func (b *ArgumentBuilder) Executes(command Command) *ArgumentBuilder {
	b.Command = command
	return b
}
func (b *LiteralArgumentBuilder) Requires(fn RequireFn) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Requires(fn)
	return b
}
func (b *RequiredArgumentBuilder) Requires(fn RequireFn) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Requires(fn)
	return b
}
func (b *ArgumentBuilder) Requires(fn RequireFn) *ArgumentBuilder {
	b.Requirement = fn
	return b
}
func (b *LiteralArgumentBuilder) Redirect(target CommandNode) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Redirect(target)
	return b
}
func (b *RequiredArgumentBuilder) Redirect(target CommandNode) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Redirect(target)
	return b
}
func (b *ArgumentBuilder) Redirect(target CommandNode) *ArgumentBuilder {
	return b.forward(target, nil, false)
}
func (b *LiteralArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *LiteralArgumentBuilder {
	b.RedirectWithModifier(target, modifier)
	return b
}
func (b *RequiredArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *RequiredArgumentBuilder {
	b.RedirectWithModifier(target, modifier)
	return b
}
func (b *ArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *ArgumentBuilder {
	return b.forward(target, modifier, false)
}
func (b *LiteralArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *LiteralArgumentBuilder {
	b.Fork(target, modifier)
	return b
}
func (b *RequiredArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *RequiredArgumentBuilder {
	b.Fork(target, modifier)
	return b
}
func (b *ArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *ArgumentBuilder {
	return b.forward(target, modifier, true)
}
func (b *ArgumentBuilder) forward(target CommandNode, modifier RedirectModifier, fork bool) *ArgumentBuilder {
	if len(b.Arguments.children) != 0 {
		return b // cannot forward a node with children
	}
	b.Target = target
	b.Modifier = modifier
	b.Forks = fork
	return b
}

func (b *ArgumentBuilder) build() *Node {
	n := &Node{
		requirement: b.Requirement,
		redirect:    b.Target,
		command:     b.Command,
		modifier:    b.Modifier,
		forks:       b.Forks,
	}
	for _, arg := range b.Arguments.children {
		n.AddChild(arg)
	}
	return n
}

type ArgumentBuilder struct {
	Arguments   RootCommandNode
	Command     Command
	Requirement RequireFn
	Target      CommandNode
	Modifier    RedirectModifier
	Forks       bool
}

type RequireFn func(context.Context) bool

var (
	ErrDispatcherUnknownCommand  = errors.New("dispatcher: unknown command")
	ErrDispatcherUnknownArgument = errors.New("dispatcher: unknown argument")
)

func (d *Dispatcher) Execute(parse *ParseResults) error {
	if parse.Reader.canRead() {
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
						if theContext == child.Context {
							next = append(next, child)
						} else {
							next = append(next, child.Copy())
						}
					} else {
						if err = modifier.Apply(theContext); err != nil && !forked {
							return err
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
	c.Range = *encompassingRange(&c.Range, r)
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
		Nodes: func() []*ParsedCommandNode {
			l := make([]*ParsedCommandNode, 0, len(c.Nodes))
			for _, n := range c.Nodes {
				l = append(l, n)
			}
			return l
		}(),
		Range:    c.Range.Copy(),
		Modifier: c.Modifier,
		Forks:    c.Forks,
		Input:    c.Input,
		cursor:   c.cursor,
	}
}

func (c *CommandContext) Int(argumentName string) int {
	return 0 // TODO
}

type RedirectModifier interface {
	Apply(ctx *CommandContext) error
}

type CommandSyntaxError struct {
	Err error
}

func (e *CommandSyntaxError) Unwrap() error { return e.Err }
func (e *CommandSyntaxError) Error() string {
	return e.Err.Error()
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
		if err == nil && rd.canRead() && rd.peek() != ArgumentSeparator {
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
		if rd.canReadLen(canRead) {
			rd.skip()
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
					Reader:  rd,
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

	if len(potentials) > 1 {
		sort.Slice(potentials, func(i, j int) bool {
			a := potentials[i]
			b := potentials[j]
			if !a.Reader.canRead() && b.Reader.canRead() {
				return true
			}
			if a.Reader.canRead() && !b.Reader.canRead() {
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
		return potentials[0]
	}

	return &ParseResults{
		Context: ctxSoFar,
		Reader:  originalReader,
		Errs:    errs,
	}
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
		for input.canRead() && input.peek() != ArgumentSeparator {
			input.skip()
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

func (n *LiteralCommandNode) String() string    { return n.Literal }
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
	if rd.canReadLen(len(n.Literal)) {
		end := start + len(n.Literal)
		if rd.String[start:end] == n.Literal {
			rd.Cursor = end
			if !rd.canRead() || rd.peek() == ArgumentSeparator {
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

func (a *ArgumentCommandNode) String() string     { return a.name }
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

type ArgumentType interface {
	Parse(rd *StringReader) (interface{}, error)
	String() string // The name of the type.
}

type ArgumentTypeFuncs struct {
	ParseFn func(rd *StringReader) (interface{}, error)
	Name    string
}

func (t *ArgumentTypeFuncs) Parse(rd *StringReader) (interface{}, error) { return t.ParseFn(rd) }
func (t *ArgumentTypeFuncs) String() string                              { return t.Name }

var (
	Bool             ArgumentType
	BoolArgumentType ArgumentType
)

func init() {
	BoolArgumentType = &ArgumentTypeFuncs{
		Name:    "Bool",
		ParseFn: func(rd *StringReader) (interface{}, error) { return rd.readBool() },
	}
	Bool = BoolArgumentType
}

type LiteralArgument struct {
	Node
}

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

func (r *StringReader) canRead() bool              { return r.canReadLen(1) }
func (r *StringReader) canReadLen(length int) bool { return r.Cursor+length <= len(r.String) }
func (r *StringReader) peek() rune                 { return rune(r.String[r.Cursor]) }
func (r *StringReader) skip()                      { r.Cursor++ }
func (r *StringReader) readBool() (bool, error) {
	start := r.Cursor
	value, err := r.readString()
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
			Type:  BoolArgumentType,
			Value: value,
		},
		Reader: r,
	}}
}

func (r *StringReader) read() rune {
	c := r.String[r.Cursor]
	r.Cursor++
	return rune(c)
}
func (r *StringReader) readString() (string, error) {
	if !r.canRead() {
		return "", nil
	}
	next := r.peek()
	if isQuotedStringStart(next) {
		r.skip()
		return r.readStringUntil(next)
	}
	return r.readUnquotedString()
}

var (
	ErrReaderInvalidEscape        = errors.New("read invalid escape character")
	ErrReaderExpectedEndOfQuote   = errors.New("reader expected end of quote")
	ErrReaderExpectedStartOfQuote = errors.New("reader expected start of quote")
)

func (r *StringReader) readStringUntil(terminator rune) (string, error) {
	var (
		result  strings.Builder
		escaped = false
	)
	for r.canRead() {
		c := r.read()
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

func (r *StringReader) readUnquotedString() (string, error) {
	if !r.canRead() {
		return "", nil
	}
	next := r.peek()
	if !isQuotedStringStart(next) {
		return "", &CommandSyntaxError{Err: &ReaderError{
			Err:    ErrReaderExpectedStartOfQuote,
			Reader: r,
		}}
	}
	r.skip()
	return r.readStringUntil(next)
}

const (
	SyntaxDoubleQuote = '"'
	SyntaxSingleQuote = '\''
	SyntaxEscape      = '\\'
)

func isQuotedStringStart(c rune) bool {
	return c == SyntaxDoubleQuote || c == SyntaxSingleQuote
}

type StringRange struct{ Start, End int }

func (r *StringRange) IsEmpty() bool {
	return r.Start == r.End
}

func (r StringRange) Copy() StringRange { return r }

func encompassingRange(r1, r2 *StringRange) *StringRange {
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
