package main

import (
	"testing"

	"github.com/haproxytech/haproxy-consul-connect/haproxy"
	"github.com/stretchr/testify/require"
)

func TestMakeHAProxyParams(t *testing.T) {
	flags := stringSliceFlag{
		"defaults.test.with.dots=3",
		"defaults.another=abdc",
		"global.with.spaces=hey I have spaces",
		"global.with.dots=hey.I.have.dots",
	}

	r, err := makeHAProxyParams(flags)
	require.NoError(t, err)

	require.Equal(t, haproxy.HAProxyParams{
		Defaults: map[string]string{
			"test.with.dots": "3",
			"another":        "abdc",
		},
		Globals: map[string]string{
			"with.spaces": "hey I have spaces",
			"with.dots":   "hey.I.have.dots",
		},
	}, r)
}
