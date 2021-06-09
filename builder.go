package brigodier

// Literal returns a new literal node builder.
func Literal(literal string) *LiteralArgumentBuilder {
	return &LiteralArgumentBuilder{Literal: literal}
}

// Argument returns a new argument node builder.
func Argument(name string, argType ArgumentType) *RequiredArgumentBuilder {
	return &RequiredArgumentBuilder{Name: name, Type: argType}
}

type (
	// Builder is a CommandNode builder.
	Builder interface{ Build() CommandNode }
	// NodeBuilder is a Builder with build-methods.
	NodeBuilder interface {
		Builder
		Then(arguments ...Builder) NodeBuilder

		Executes(command Command) NodeBuilder
		Requires(fn RequireFn) NodeBuilder
		Redirect(target CommandNode) NodeBuilder
		RedirectWithModifier(target CommandNode, modifier RedirectModifier) NodeBuilder
		Fork(target CommandNode, modifier RedirectModifier) NodeBuilder
		Forward(target CommandNode, modifier RedirectModifier, fork bool) NodeBuilder
	}
	// LiteralNodeBuilder is a LiteralCommandNode builder.
	LiteralNodeBuilder interface {
		Builder
		BuildLiteral() *LiteralCommandNode
		NodeBuilder() NodeBuilder // Convert to NodeBuilder
		Then(arguments ...Builder) LiteralNodeBuilder

		Executes(command Command) LiteralNodeBuilder
		Requires(fn RequireFn) LiteralNodeBuilder
		Redirect(target CommandNode) LiteralNodeBuilder
		RedirectWithModifier(target CommandNode, modifier RedirectModifier) LiteralNodeBuilder
		Fork(target CommandNode, modifier RedirectModifier) LiteralNodeBuilder
		Forward(target CommandNode, modifier RedirectModifier, fork bool) LiteralNodeBuilder
	}
	// ArgumentNodeBuilder is an ArgumentCommandNode builder.
	ArgumentNodeBuilder interface {
		Builder
		BuildArgument() *ArgumentCommandNode
		NodeBuilder() NodeBuilder // Convert to NodeBuilder
		Then(arguments ...Builder) ArgumentNodeBuilder

		Suggests(provider SuggestionProvider) ArgumentNodeBuilder
		Executes(command Command) ArgumentNodeBuilder
		Requires(fn RequireFn) ArgumentNodeBuilder
		Redirect(target CommandNode) ArgumentNodeBuilder
		RedirectWithModifier(target CommandNode, modifier RedirectModifier) ArgumentNodeBuilder
		Fork(target CommandNode, modifier RedirectModifier) ArgumentNodeBuilder
		Forward(target CommandNode, modifier RedirectModifier, fork bool) ArgumentNodeBuilder
	}

	// LiteralArgumentBuilder builds a LiteralCommandNode.
	LiteralArgumentBuilder struct {
		Literal string
		ArgumentBuilder
	}
	// RequiredArgumentBuilder builds an ArgumentCommandNode.
	RequiredArgumentBuilder struct {
		Name                string
		Type                ArgumentType
		SuggestionsProvider SuggestionProvider // Optional
		ArgumentBuilder
	}
)

var _ LiteralNodeBuilder = (*LiteralArgumentBuilder)(nil)
var _ ArgumentNodeBuilder = (*RequiredArgumentBuilder)(nil)

// ArgumentBuilder has the common builder fields and is wrapped by
// LiteralArgumentBuilder and RequiredArgumentBuilder
type ArgumentBuilder struct {
	Arguments   RootCommandNode
	Command     Command
	Requirement RequireFn
	Target      CommandNode
	Modifier    RedirectModifier
	Forks       bool
}

func (b *ArgumentBuilder) build() *Node {
	n := &Node{
		requirement: b.Requirement,
		redirect:    b.Target,
		command:     b.Command,
		modifier:    b.Modifier,
		forks:       b.Forks,
	}
	b.Arguments.ChildrenOrdered().Range(func(_ string, arg CommandNode) bool {
		n.AddChild(arg)
		return true
	})
	return n
}

func (b *ArgumentBuilder) then(arguments ...Builder) {
	for _, a := range arguments {
		b.Arguments.AddChild(a.Build())
	}
}

// Then adds arguments to the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Then(arguments ...Builder) LiteralNodeBuilder {
	b.ArgumentBuilder.then(arguments...)
	return b
}

// Then adds arguments to the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Then(arguments ...Builder) ArgumentNodeBuilder {
	b.ArgumentBuilder.then(arguments...)
	return b
}

func (b *RequiredArgumentBuilder) NodeBuilder() NodeBuilder { return &nodeBuilder{a: b} }
func (b *LiteralArgumentBuilder) NodeBuilder() NodeBuilder  { return &nodeBuilder{l: b} }

func (n *LiteralCommandNode) Build() CommandNode { return n }
func (n *LiteralCommandNode) CreateBuilder() NodeBuilder {
	return &nodeBuilder{l: n.CreateLiteralBuilder()}
}
func (n *LiteralCommandNode) CreateLiteralBuilder() LiteralNodeBuilder {
	return Literal(n.Literal).
		Requires(n.Requirement()).
		Forward(n.Redirect(), n.RedirectModifier(), n.IsFork()).
		Executes(n.Command())
}

func (b *LiteralArgumentBuilder) Build() CommandNode { return b.BuildLiteral() }
func (b *LiteralArgumentBuilder) BuildLiteral() *LiteralCommandNode {
	return &LiteralCommandNode{
		Node:    *b.ArgumentBuilder.build(),
		Literal: b.Literal,
	}
}

func (a *ArgumentCommandNode) Build() CommandNode { return a }
func (a *ArgumentCommandNode) CreateBuilder() NodeBuilder {
	return &nodeBuilder{a: a.CreateArgumentBuilder()}
}
func (a *ArgumentCommandNode) CreateArgumentBuilder() ArgumentNodeBuilder {
	return Argument(a.Name(), a.Type()).
		Requires(a.Requirement()).
		Forward(a.Redirect(), a.RedirectModifier(), a.IsFork()).
		Suggests(a.CustomSuggestions()).
		Executes(a.Command())
}

func (b *RequiredArgumentBuilder) Build() CommandNode { return b.BuildArgument() }
func (b *RequiredArgumentBuilder) BuildArgument() *ArgumentCommandNode {
	return &ArgumentCommandNode{
		Node:              *b.ArgumentBuilder.build(),
		name:              b.Name,
		argType:           b.Type,
		customSuggestions: b.SuggestionsProvider,
	}
}

// Suggests defines the SuggestionProvider of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Suggests(provider SuggestionProvider) ArgumentNodeBuilder {
	b.SuggestionsProvider = provider
	return b
}

// Executes defines the Command of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Executes(command Command) LiteralNodeBuilder {
	b.ArgumentBuilder.Executes(command)
	return b
}

// Executes defines the Command of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Executes(command Command) ArgumentNodeBuilder {
	b.ArgumentBuilder.Executes(command)
	return b
}

// Executes defines the Command of the resulting CommandNode.
func (b *ArgumentBuilder) Executes(command Command) *ArgumentBuilder {
	b.Command = command
	return b
}

// Requires defines the RequireFn of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Requires(fn RequireFn) LiteralNodeBuilder {
	b.ArgumentBuilder.Requires(fn)
	return b
}

// Requires defines the RequireFn of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Requires(fn RequireFn) ArgumentNodeBuilder {
	b.ArgumentBuilder.Requires(fn)
	return b
}

// Requires defines the RequireFn of the resulting CommandNode.
func (b *ArgumentBuilder) Requires(fn RequireFn) *ArgumentBuilder {
	b.Requirement = fn
	return b
}

// Redirect defines the redirect node of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Redirect(target CommandNode) LiteralNodeBuilder {
	b.ArgumentBuilder.Redirect(target)
	return b
}

// Redirect defines the redirect node of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Redirect(target CommandNode) ArgumentNodeBuilder {
	b.ArgumentBuilder.Redirect(target)
	return b
}

// Redirect defines the redirect node of the resulting CommandNode.
func (b *ArgumentBuilder) Redirect(target CommandNode) *ArgumentBuilder {
	return b.Forward(target, nil, false)
}

// RedirectWithModifier defines the redirect modifier of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) LiteralNodeBuilder {
	b.ArgumentBuilder.RedirectWithModifier(target, modifier)
	return b
}

// RedirectWithModifier defines the redirect modifier of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) ArgumentNodeBuilder {
	b.ArgumentBuilder.RedirectWithModifier(target, modifier)
	return b
}

// RedirectWithModifier defines the redirect modifier of the resulting CommandNode.
func (b *ArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *ArgumentBuilder {
	return b.Forward(target, modifier, false)
}

// Fork defines the fork of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) LiteralNodeBuilder {
	b.ArgumentBuilder.Fork(target, modifier)
	return b
}

// Fork defines the fork of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) ArgumentNodeBuilder {
	b.ArgumentBuilder.Fork(target, modifier)
	return b
}

// Fork defines the fork of the resulting CommandNode.
func (b *ArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *ArgumentBuilder {
	return b.Forward(target, modifier, true)
}

// Forward defines the forward of the resulting ArgumentCommandNode.
func (b *LiteralArgumentBuilder) Forward(target CommandNode, modifier RedirectModifier, fork bool) LiteralNodeBuilder {
	b.ArgumentBuilder.Forward(target, modifier, fork)
	return b
}

// Forward defines the forward of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Forward(target CommandNode, modifier RedirectModifier, fork bool) ArgumentNodeBuilder {
	b.ArgumentBuilder.Forward(target, modifier, fork)
	return b
}

func (b *ArgumentBuilder) Forward(target CommandNode, modifier RedirectModifier, fork bool) *ArgumentBuilder {
	if len(b.Arguments.children) != 0 {
		return b // cannot forward a node with children
	}
	b.Target = target
	b.Modifier = modifier
	b.Forks = fork
	return b
}

// CreateBuilder cannot create a builder from root node and returns a nop-builder!
func (r *RootCommandNode) CreateBuilder() NodeBuilder {
	return nil
}

// nodeBuilder implements NodeBuilder and wraps either one of:
//  LiteralNodeBuilder
//  ArgumentNodeBuilder
type nodeBuilder struct {
	l LiteralNodeBuilder
	a ArgumentNodeBuilder
}

var _ NodeBuilder = (*nodeBuilder)(nil)

func (b *nodeBuilder) Build() CommandNode {
	if b.l == nil {
		return b.a.Build()
	}
	return b.l.Build()
}

func (b *nodeBuilder) Then(arguments ...Builder) NodeBuilder {
	if b.l == nil {
		b.a.Then(arguments...)
	} else {
		b.l.Then(arguments...)
	}
	return b
}

func (b *nodeBuilder) Executes(command Command) NodeBuilder {
	if b.l == nil {
		b.a.Executes(command)
	} else {
		b.l.Executes(command)
	}
	return b
}

func (b *nodeBuilder) Requires(fn RequireFn) NodeBuilder {
	if b.l == nil {
		b.a.Requires(fn)
	} else {
		b.l.Requires(fn)
	}
	return b
}

func (b *nodeBuilder) Redirect(target CommandNode) NodeBuilder {
	if b.l == nil {
		b.a.Redirect(target)
	} else {
		b.l.Redirect(target)
	}
	return b
}

func (b *nodeBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) NodeBuilder {
	if b.l == nil {
		b.a.RedirectWithModifier(target, modifier)
	} else {
		b.l.RedirectWithModifier(target, modifier)
	}
	return b
}

func (b *nodeBuilder) Fork(target CommandNode, modifier RedirectModifier) NodeBuilder {
	if b.l == nil {
		b.a.Fork(target, modifier)
	} else {
		b.l.Fork(target, modifier)
	}
	return b
}

func (b *nodeBuilder) Forward(target CommandNode, modifier RedirectModifier, fork bool) NodeBuilder {
	if b.l == nil {
		b.a.Forward(target, modifier, fork)
	} else {
		b.l.Forward(target, modifier, fork)
	}
	return b
}

type nopNodeBuilder struct{}

var _ NodeBuilder = (*nopNodeBuilder)(nil)

func (b *nopNodeBuilder) Build() CommandNode                                             { return nil }
func (b *nopNodeBuilder) Then(...Builder) NodeBuilder                                    { return b }
func (b *nopNodeBuilder) Executes(Command) NodeBuilder                                   { return b }
func (b *nopNodeBuilder) Requires(RequireFn) NodeBuilder                                 { return b }
func (b *nopNodeBuilder) Redirect(CommandNode) NodeBuilder                               { return b }
func (b *nopNodeBuilder) RedirectWithModifier(CommandNode, RedirectModifier) NodeBuilder { return b }
func (b *nopNodeBuilder) Fork(CommandNode, RedirectModifier) NodeBuilder                 { return b }
func (b *nopNodeBuilder) Forward(CommandNode, RedirectModifier, bool) NodeBuilder        { return b }
