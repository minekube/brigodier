package brigodier

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestB(t *testing.T) {
	var d Dispatcher
	d.Register(
		Literal("foo").
			Then(
				Argument("bar", Bool).
					Executes(CommandFunc(func(c *CommandContext) error {
						fmt.Printf("Bar is %v\n", c.Int("bar"))
						return nil
					})),
			).
			Executes(CommandFunc(func(c *CommandContext) error {
				fmt.Println("Called foo with no arguments")
				return nil
			})),
	)

	parse := d.Parse(context.TODO(), "foo")
	require.NotNil(t, parse)

	spew.Dump(parse)

	err := d.Execute(parse)
	require.NoError(t, err)
}
