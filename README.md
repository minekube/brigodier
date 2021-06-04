# brigodier

[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/minekube/brigodier?sort=semver)](https://github.com/minekube/brigodier/releases)
[![Doc](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go)](https://pkg.go.dev/go.minekube.com/brigodier)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/minekube/brigodier?logo=go)](https://golang.org/doc/devel/release.html)
[![Go Report Card](https://goreportcard.com/badge/go.minekube.com/brigodier)](https://goreportcard.com/report/go.minekube.com/brigodier)
[![test](https://github.com/minekube/brigodier/workflows/test/badge.svg)](https://github.com/minekube/brigodier/actions?query=workflow%3Atest)
[![Discord](https://img.shields.io/discord/633708750032863232?logo=discord)](https://discord.gg/6vMDqWE)

**Brigodier is a command parser & dispatcher, designed and developed
to provide a simple and flexible command framework.**

It can be used in many command-line environments such as for
chat commands in the Minecraft Java Edition.

It is completely ported to Go from [Mojang's Brigadier](https://github.com/Mojang/brigadier)
(written in Java), including all features and tests.

## Installation

For use in your projects go get it with:

```shell
go get -u go.minekube.com/brigodier
```

## Usage

At the heart of Brigodier, you need a `CommandDispatcher`.

A command dispatcher holds a "command tree", which is a series of
`CommandNode`s that represent the various possible syntax options that form a valid command.

### Registering a new command

Before we can start parsing and dispatching commands, we need to build up our command tree.
Every registration is an append operation, so you can freely extend existing commands in a project
without needing access to the source code that created them.

Command registration also encourages the use of a builder pattern to keep code cruft to a minimum.

A "command" is a fairly loose term, but typically it means an exit point of the command tree.
Every node can have an `Executes` function attached to it, which signifies that if the input stops
here then this function will be called with the context so far.

Consider the following example:
```go
var d Dispatcher

d.Register(
	Literal("foo").Then(
		Argument("bar", Int).
			Executes(CommandFunc(func(c *CommandContext) error {
				fmt.Println("Bar is", c.Int("bar"))
				return nil
			})),
	).Executes(CommandFunc(func(c *CommandContext) error {
		fmt.Println("Called foo with no arguments")
		return nil
	})),
)
``` 

This snippet registers two "commands": `foo` and `foo <bar>`.
It is also common to refer to the `<bar>` as a "subcommand" of `foo`, as it's a child node.

At the start of the tree is a "root node", and it **must** have `LiteralCommandNode`s as children.
Here, we register one command under the root: `literal("foo")`, which means "the user must type the literal string 'foo'".

Under that is two extra definitions: a child node for possible further evaluation, or an `executes` block
if the user input stops here.

The child node works exactly the same way, but is no longer limited to literals.
The other type of node that is now allowed is an `ArgumentCommandNode`, which takes in a name, and an argument type.

Arguments can be anything, and you are encouraged to build your own for seamless integration into your own product.
There are some builtin ArgumentTypes included, such as `Int` or `String`.

Argument types will be asked to parse input as much as they can, and then store the "result" of that argument however
they see fit or throw a relevant error if they can't parse.

For example, an integer argument would parse "123" and store it as `123` (`int`), but throw an error if the input were `onetwothree`.

When a command function runs, it can access these arguments in the context provided to the registered function.

## Parsing user input
So, we've registered some commands, and now we're ready to take in user input.
If you're in a rush, you can just call `dispatcher.Execute(ctx, "foo 123")` and call it a day.

The result of `execute` is an integer that was returned from an evaluated command.
The meaning of this integer depends on the command, and will typically not be useful to programmers.

The `context` can be used to track users/players/etc and
will be provided to the command to give context on what's happening (e.g., who has run the command).

If the command failed or could not parse, some form of `CommandSyntaxError` will be returned,
or the error that the Command returned.

If you wish to have more control over the parsing & executing of commands,
or wish to cache the parse results, so you can execute it multiple times,
you can split it up into two steps:

```go
parse := dispatcher.Parse(ctx, "foo 123")
err := dispatcher.Execute(parse)
``` 

This is highly recommended as the parse step is the most expensive,
and may be easily cached depending on your application.

You can also use this to do further introspection on a command,
before (or without) actually running it.


The convenient method to parse and execute a command is:
```go
err := dispatcher.Do(ctx, "foo 123")
``` 

### Inspecting a command

If you `parse` some input, you can find out what it will perform (if anything) and provide
hints to the user safely and immediately.

The parse will never fail, and the `ParseResults` it returns will contain a *possible* context that a command may be called with
(and from that, you can inspect which nodes the user entered, complete with start/end positions in the input string).
It also contains a map of parse exceptions for each command node it encountered. If it couldn't build a valid context, then
the reason is inside this exception map.

### Displaying usage info

There are two forms of "usage strings" provided by this library, both require a target node.

- `dispatcher.AllUsage(ctx, node, restricted)`
  will return a list of all possible commands (executable end-points)
  under the target node and their human-readable path. If `restricted`,
  it will ignore commands that `ctx` does not have access to.
  This will look like [`foo`, `foo <bar>`].
  
- `dispatcher.SmartUsage(ctx, node)`
  will return a map of the child nodes to their "smart usage" human-readable path.
  This tries to squash future-nodes together and show optional & typed information,
  and can look like `foo (<bar>)`.