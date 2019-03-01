package spoe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	v, err := parseVersion("2.0")
	require.Nil(t, err)
	require.Equal(t, []int{2, 0}, v)

	v, err = parseVersion("1.3.9")
	require.Nil(t, err)
	require.Equal(t, []int{1, 3, 9}, v)
}
