package brigodier

func Literal(literal string) *LiteralArgumentBuilder {
	return &LiteralArgumentBuilder{Literal: literal}
}
func Argument(name string, argType ArgumentType) *RequiredArgumentBuilder {
	return &RequiredArgumentBuilder{Name: name, Type: argType}
}

type (
	LiteralArgumentBuilder struct {
		Literal string
		ArgumentBuilder
	}
	RequiredArgumentBuilder struct {
		Name                string
		Type                ArgumentType
		SuggestionsProvider SuggestionProvider // Optional
		ArgumentBuilder
	}
)

type (
	then  interface{ then() }
	build interface{ build() CommandNode }
)

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

func (b *LiteralArgumentBuilder) Then(arguments ...then) *LiteralArgumentBuilder {
	b.ArgumentBuilder.then(arguments...)
	return b
}
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

func (b *RequiredArgumentBuilder) Suggests(provider SuggestionProvider) *RequiredArgumentBuilder {
	b.SuggestionsProvider = provider
	return b
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
	b.ArgumentBuilder.RedirectWithModifier(target, modifier)
	return b
}
func (b *RequiredArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *RequiredArgumentBuilder {
	b.ArgumentBuilder.RedirectWithModifier(target, modifier)
	return b
}
func (b *ArgumentBuilder) RedirectWithModifier(target CommandNode, modifier RedirectModifier) *ArgumentBuilder {
	return b.forward(target, modifier, false)
}
func (b *LiteralArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *LiteralArgumentBuilder {
	b.ArgumentBuilder.Fork(target, modifier)
	return b
}
func (b *RequiredArgumentBuilder) Fork(target CommandNode, modifier RedirectModifier) *RequiredArgumentBuilder {
	b.ArgumentBuilder.Fork(target, modifier)
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
	b.Arguments.ChildrenOrdered().Range(func(_ string, arg CommandNode) bool {
		n.AddChild(arg)
		return true
	})
	return n
}
