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

// Dispatcher is the command dispatcher,
// for registering, parsing, and executing commands.
type Dispatcher struct {
	// The root of this command tree.
	// This is often useful as a target of an
	// ArgumentBuilder.Redirect, AllUsage or SmartUsage.
	Root RootCommandNode
}

// Register registers new commands.
// This is a shortcut for calling Dispatcher.Root.AddChild after building the provided command.
//
// As RootCommandNode can only hold literals, this method will only allow literal arguments.
func (d *Dispatcher) Register(command *LiteralArgumentBuilder) *LiteralCommandNode {
	b := command.Build()
	d.Root.AddChild(b)
	return b
}

// RequireFn is the function used for CommandNode.CanUse.
type RequireFn func(context.Context) bool

var (
	// ErrDispatcherUnknownCommand indicates that an input command was not found.
	ErrDispatcherUnknownCommand = errors.New("dispatcher: unknown command")
	// ErrDispatcherUnknownArgument indicates that the argument of an input command was not found.
	ErrDispatcherUnknownArgument = errors.New("dispatcher: unknown argument")
)

// Execute executes a given pre-parsed command.
//
// If this command returns a nil error, then it successfully executed something.
// If the execution was a failure, then an error.
// Most errors will be of type CommandSyntaxError wrapping another error struct for details
// or an unknown error returned by a Command execution.
// The meaning behind the returned result is arbitrary, and will depend
// entirely on what command was performed.
//
// If the command passes through a node that is CommandNode.IsFork then it will be 'forked'.
// A forked command will not return a CommandSyntaxError.
//
// TODO After each and any command is ran, a registered callback given to #setConsumer(ResultConsumer)}
// will be notified of the result and success of the command. You can use that method to gather more meaningful
// results than this method will return, especially when a command forks.
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

func (d *Dispatcher) Do(ctx context.Context, command string) error {
	return d.Execute(d.Parse(ctx, command))
}

var ErrDispatcherExpectedArgumentSeparator = errors.New("dispatcher expected argument separator")

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

// Path finds a valid path to a given node on the command tree.
//
// There may theoretically be multiple paths to a node on the tree, especially with the use of forking or redirecting.
// As such, this method makes no guarantees about which path it finds. It will not look at forks or redirects,
// and find the first instance of the target node on the tree.
//
// The only guarantee made is that for the same command tree and the same version of this library, the result of
// this method will ALWAYS be a valid input for FindNode, which should return the same node as provided to this method.
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

// FindNode finds a node by its path.
//
// Paths may be generated with Path(CommandNode), and are guaranteed (for the same tree, and the
// same version of this library) to always produce the same valid node by this method.
//
// If a node could not be found at the specified path, nil will be returned.
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

// AddChild adds a CommandNode to the Node's children.
// Most often times one should use Dispatcher.Register instead.
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
			n.putChild(node.Name(), node)
			switch t := node.(type) {
			case *LiteralCommandNode:
				n.Literals()[node.Name()] = t
			case *ArgumentCommandNode:
				n.Arguments()[node.Name()] = t
			}
		}
	}
}

func (n *Node) putChild(name string, node CommandNode) {
	n.Children()[name] = node
	n.ChildrenOrdered().Put(name, node)
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

// Command is the command run by Dispatcher.Execute for a matching input.
type Command interface {
	Run(c *CommandContext) error
}

// CommandFunc is a convenient function type implementing the Command interface.
type CommandFunc func(c *CommandContext) error

func (cf CommandFunc) Run(c *CommandContext) error { return cf(c) }

// CommandNode is a command node in a tree.
type CommandNode interface {
	// Arguments returns the nodes's arguments.
	Arguments() map[string]*ArgumentCommandNode
	// Literals returns the node's literals.
	Literals() map[string]*LiteralCommandNode
	// CanUse tests whether the node can be used with the given context.Context.
	// It can be used for restricting Dispatcher.Parse/Execute and Dispatcher.AllUsage
	// for certain subjects, e.g. whether the entity seeing/executing a command is allowed.
	CanUse(ctx context.Context) bool
	// RelevantNodes returns the relevant nodes of a Node for an input.
	RelevantNodes(input *StringReader) []CommandNode
	// Parse parses the given reader input.
	Parse(ctx *CommandContext, rd *StringReader) error
	// Redirect is the optional redirect node.
	// May return nil.
	Redirect() CommandNode
	// Command returns the command of the node.
	// May return nil since not all nodes must be executable.
	Command() Command
	setCommand(Command)
	// RedirectModifier is the optional redirect modifier.
	// May return nil.
	RedirectModifier() RedirectModifier
	// IsFork indicated whether the node is a fork.
	IsFork() bool
	// Name returns the name of the node.
	Name() string
	// Children returns the node's children.
	// Use ChildrenOrdered if register order matters.
	Children() map[string]CommandNode
	// ChildrenOrdered returns the node's children in the same order as registered.
	ChildrenOrdered() StringCommandNodeMap
	// AddChild adds node children to the node.
	AddChild(nodes ...CommandNode)
	// UsageText returns the usage text of the node.
	UsageText() string
}

// Node is a node with the common fields and wrapped by
// RootCommandNode, LiteralCommandNode and ArgumentCommandNode.
type Node struct {
	childrenOrdered StringCommandNodeMap
	children        map[string]CommandNode
	literals        map[string]*LiteralCommandNode
	arguments       map[string]*ArgumentCommandNode
	requirement     RequireFn
	redirect        CommandNode
	command         Command
	modifier        RedirectModifier
	forks           bool
}

func (n *Node) RedirectModifier() RedirectModifier { return n.modifier }

func (n *Node) ChildrenOrdered() StringCommandNodeMap {
	if n.childrenOrdered == nil {
		n.childrenOrdered = NewStringCommandNodeMap()
	}
	return n.childrenOrdered
}
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

// RootCommandNode is the root of a command node tree.
type RootCommandNode struct {
	Node // The starting root node.
}

func (r *RootCommandNode) String() string                             { return "<root>" }
func (r *RootCommandNode) Name() string                               { return "" }
func (r *RootCommandNode) UsageText() string                          { return "" }
func (r *RootCommandNode) Parse(*CommandContext, *StringReader) error { return nil }

// LiteralCommandNode is a command node storing a fixed literal.
//
// Use Literal(name) to build it.
type LiteralCommandNode struct {
	Node
	Literal string

	literalLowerCase string
}

// IncorrectLiteralError is used to indicate an incorrect literal parse error.
type IncorrectLiteralError struct {
	Literal string // The incorrect literal value.
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

// ArgumentCommandNode is an argument command node storing
// the argument type, name and optional custom suggestions.
//
// Use Argument(name, type) to build it.
type ArgumentCommandNode struct {
	Node
	name              string
	argType           ArgumentType
	customSuggestions SuggestionProvider // Optional

	cachedUsageText string
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

func (a *ArgumentCommandNode) String() string {
	return fmt.Sprintf("<argument %s:%s>", a.name, a.argType)
}
func (a *ArgumentCommandNode) Name() string                          { return a.name }
func (a *ArgumentCommandNode) Type() ArgumentType                    { return a.argType }
func (a *ArgumentCommandNode) CustomSuggestions() SuggestionProvider { return a.customSuggestions }

const (
	UsageArgumentOpen  rune = '['
	UsageArgumentClose rune = ']'
)

func (a *ArgumentCommandNode) UsageText() string {
	if a.cachedUsageText == "" {
		a.cachedUsageText = fmt.Sprintf("%c%s%c", UsageArgumentOpen, a.name, UsageArgumentClose)
	}
	return a.cachedUsageText
}
