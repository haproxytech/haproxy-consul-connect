package haproxy_cmd

import (
	"io"
	"testing"

	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/stretchr/testify/require"
)

var nilLogger = func(io.Reader) {}

func Test_runCommand_ok(t *testing.T) {
	t.Parallel()
	sd := lib.NewShutdown()
	cmd, err := runCommand(sd, nilLogger, "ls", ".")
	require.NoError(t, err)
	err = cmd.Wait()
	require.NoError(t, err)
}

func Test_runCommand_nok_wrong_path(t *testing.T) {
	t.Parallel()
	sd := lib.NewShutdown()
	cmd, err := runCommand(sd, nilLogger, "/path/to/nowhere/that/can/be/found/myExec", "--help")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
	require.Nil(t, cmd)
}
