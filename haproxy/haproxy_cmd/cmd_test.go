package haproxy_cmd

import (
	"testing"

	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/stretchr/testify/require"
)

func Test_runCommand_ok(t *testing.T) {
	t.Parallel()
	sd := lib.NewShutdown()
	cmd, err := runCommand(sd, "ls", ".")
	require.NoError(t, err)
	cmd.Wait()
}

func Test_runCommand_nok_wrong_path(t *testing.T) {
	t.Parallel()
	sd := lib.NewShutdown()
	cmd, err := runCommand(sd, "/path/to/nowhere/that/can/be/found/myExec", "--help")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
	require.Nil(t, cmd)
}
