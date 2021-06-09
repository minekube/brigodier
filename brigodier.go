package brigodier

import (
	"context"
	"errors"
	"fmt"
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

var (
	// ErrDispatcherUnknownCommand indicates that an input command was not found.
	ErrDispatcherUnknownCommand = errors.New("dispatcher: unknown command")
	// ErrDispatcherUnknownArgument indicates that the argument of an input command was not found.
	ErrDispatcherUnknownArgument = errors.New("dispatcher: unknown argument")
)

// Do parses and then executes the specified command and returns the execution error, if any.
func (d *Dispatcher) Do(ctx context.Context, command string) error {
	return d.Execute(d.Parse(ctx, command))
}

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

// RedirectModifier modifies
type RedirectModifier interface {
	Apply(ctx *CommandContext) (context.Context, error)
}

// ModifierFunc is a convenient function type implementing the RedirectModifier interface.
type ModifierFunc func(c *CommandContext) (context.Context, error)

// Apply implements RedirectModifier.
func (m ModifierFunc) Apply(c *CommandContext) (context.Context, error) { return m(c) }

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

// Command is the command run by Dispatcher.Execute for a matching input.
type Command interface {
	Run(c *CommandContext) error
}

// CommandFunc is a convenient function type implementing the Command interface.
type CommandFunc func(c *CommandContext) error

// Run implements Command.
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
	// Requirement is the optional condition used to run CanUse.
	Requirement() RequireFn
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
	// RemoveChild removes child nodes from the node
	RemoveChild(names ...string)
	// UsageText returns the usage text of the node.
	UsageText() string
}

// RequireFn is the function used for CommandNode.CanUse.
type RequireFn func(context.Context) bool

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

func (n *Node) RemoveChild(names ...string) {
	for _, name := range names {
		delete(n.Children(), name)
		delete(n.Arguments(), name)
		delete(n.Literals(), name)
		n.ChildrenOrdered().Remove(name)
	}
}

func (n *Node) RedirectModifier() RedirectModifier { return n.modifier }
func (n *Node) Requirement() RequireFn             { return n.requirement }

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

	cachedLiteralLowerCase string
}

func (n *LiteralCommandNode) String() string    { return fmt.Sprintf("<literal %s>", n.Literal) }
func (n *LiteralCommandNode) Name() string      { return n.Literal }
func (n *LiteralCommandNode) UsageText() string { return n.Literal }

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

func (a *ArgumentCommandNode) String() string {
	return fmt.Sprintf("<argument %s:%s>", a.name, a.argType)
}
func (a *ArgumentCommandNode) Name() string                          { return a.name }
func (a *ArgumentCommandNode) Type() ArgumentType                    { return a.argType }
func (a *ArgumentCommandNode) CustomSuggestions() SuggestionProvider { return a.customSuggestions }

const (
	// UsageArgumentOpen is the open rune for ArgumentCommandNode.UsageText.
	UsageArgumentOpen rune = '['
	// UsageArgumentClose is the close rune for ArgumentCommandNode.UsageText.
	UsageArgumentClose rune = ']'
)

func (a *ArgumentCommandNode) UsageText() string {
	if a.cachedUsageText == "" {
		a.cachedUsageText = fmt.Sprintf("%c%s%c", UsageArgumentOpen, a.name, UsageArgumentClose)
	}
	return a.cachedUsageText
}
