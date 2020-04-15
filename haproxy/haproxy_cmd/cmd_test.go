package haproxy_cmd

import (
	"testing"

	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/stretchr/testify/assert"
)

func Test_runCommand_ok(t *testing.T) {
	t.Parallel()
	sd := lib.NewShutdown()
	cmd, err := runCommand(sd, "ls", ".")
	assert.NoError(t, err)
	cmd.Wait()
}

func Test_runCommand_nok_wrong_path(t *testing.T) {
	t.Parallel()
	sd := lib.NewShutdown()
	cmd, err := runCommand(sd, "/path/to/nowhere/that/can/be/found/myExec", "--help")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
	assert.Nil(t, cmd)
}
