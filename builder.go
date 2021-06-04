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

type (
	// Types that implement this interface can be passed into the "Then" builder methods
	// and must implement build as well.
	then  interface{ then() }
	build interface{ build() CommandNode }
)

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

func (b *ArgumentBuilder) then(arguments ...then) {
	for _, a := range arguments {
		if i, ok := a.(build); ok {
			b.Arguments.AddChild(i.build())
		}
	}
}

// Then adds arguments to the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Then(arguments ...then) *LiteralArgumentBuilder {
	b.ArgumentBuilder.then(arguments...)
	return b
}

// Then adds arguments to the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Then(arguments ...then) *RequiredArgumentBuilder {
	b.ArgumentBuilder.then(arguments...)
	return b
}

func (n *LiteralCommandNode) then()              {}
func (n *LiteralCommandNode) build() CommandNode { return n }

func (b *LiteralArgumentBuilder) then()              {}
func (b *LiteralArgumentBuilder) build() CommandNode { return b.Build() }
func (b *LiteralArgumentBuilder) Build() *LiteralCommandNode {
	return &LiteralCommandNode{
		Node:    *b.ArgumentBuilder.build(),
		Literal: b.Literal,
	}
}

func (a *ArgumentCommandNode) then()              {}
func (a *ArgumentCommandNode) build() CommandNode { return a }

func (b *RequiredArgumentBuilder) then()              {}
func (b *RequiredArgumentBuilder) build() CommandNode { return b.Build() }
func (b *RequiredArgumentBuilder) Build() *ArgumentCommandNode {
	return &ArgumentCommandNode{
		Node:              *b.ArgumentBuilder.build(),
		name:              b.Name,
		argType:           b.Type,
		customSuggestions: b.SuggestionsProvider,
	}
}

// Suggests defines the SuggestionProvider of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Suggests(provider SuggestionProvider) *RequiredArgumentBuilder {
	b.SuggestionsProvider = provider
	return b
}

// Executes defines the Command of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Executes(command Command) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Executes(command)
	return b
}

// Executes defines the Command of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Executes(command Command) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Executes(command)
	return b
}

// Executes defines the Command of the resulting CommandNode.
func (b *ArgumentBuilder) Executes(command Command) *ArgumentBuilder {
	b.Command = command
	return b
}

// Requires defines the RequireFn of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Requires(fn RequireFn) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Requires(fn)
	return b
}

// Requires defines the RequireFn of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Requires(fn RequireFn) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Requires(fn)
	return b
}

// Requires defines the RequireFn of the resulting CommandNode.
func (b *ArgumentBuilder) Requires(fn RequireFn) *ArgumentBuilder {
	b.Requirement = fn
	return b
}

// Redirect defines the redirect node of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Redirect(target CommandNode) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Redirect(target)
	return b
}

// Redirect defines the redirect node of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Redirect(target CommandNode) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Redirect(target)
	return b
}

// Redirect defines the redirect node of the resulting CommandNode.
func (b *ArgumentBuilder) Redirect(target CommandNode) *ArgumentBuilder {
	return b.forward(target, nil, false)
}

// RedirectWithModifier defines the redirect modifier of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *LiteralArgumentBuilder {
	b.ArgumentBuilder.RedirectWithModifier(target, modifier)
	return b
}

// RedirectWithModifier defines the redirect modifier of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *RequiredArgumentBuilder {
	b.ArgumentBuilder.RedirectWithModifier(target, modifier)
	return b
}

// RedirectWithModifier defines the redirect modifier of the resulting CommandNode.
func (b *ArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *ArgumentBuilder {
	return b.forward(target, modifier, false)
}

// Fork defines the fork of the resulting LiteralCommandNode.
func (b *LiteralArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Fork(target, modifier)
	return b
}

// Fork defines the fork of the resulting ArgumentCommandNode.
func (b *RequiredArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Fork(target, modifier)
	return b
}

// Fork defines the fork of the resulting CommandNode.
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
	b.Arguments.ChildrenOrdered().Range(func(_ string, arg CommandNode) bool {
		n.AddChild(arg)
		return true
	})
	return n
}
