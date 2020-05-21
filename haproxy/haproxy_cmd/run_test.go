package haproxy_cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type version struct {
	v1     string
	v2     string
	status int
}

func TestCompareVersion(t *testing.T) {
	tests := []*version{
		{status: 1, v1: "1.3", v2: "1.2"},
		{status: 1, v1: "1.3.1", v2: "1.3"},
		{status: 1, v1: "2.0.1", v2: "2.0"},
		{status: 0, v1: "2.0", v2: "2.0"},
		{status: 0, v1: "2.0.0", v2: "2.0"},
		{status: 0, v1: "2.0", v2: "2.0.0"},
		{status: -1, v1: "1.2", v2: "1.3"},
		{status: -1, v1: "1.3", v2: "1.3.1"},
		{status: -1, v1: "2.0", v2: "2.0.1"},
		{status: -1, v1: "2", v2: "2.0.1"},
		{status: -1, v1: "2.0", v2: "2"},
		{status: -1, v1: "3.0", v2: "2.2.2"},
		{status: -1, v1: "2.2", v2: "3.0"},
	}
	for _, test := range tests {
		res, _ := compareVersion(test.v1, test.v2)
		require.Equal(t, res, test.status)
	}
}
