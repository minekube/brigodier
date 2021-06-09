package brigodier

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_CreateBuilder_Executes(t *testing.T) {
	cmd := CommandFunc(func(c *CommandContext) error { return nil })
	node := Literal("test").Executes(cmd).Build()
	build := node.CreateBuilder().Build()
	require.NotNil(t, build.Command())
}
