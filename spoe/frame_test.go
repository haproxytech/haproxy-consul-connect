package spoe

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrameEncoding(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	frame := frame{
		ftype:    frameTypeAgentACK,
		flags:    frameFlagFin,
		streamID: 42,
		frameID:  53,
		data:     []byte("this is the frame data"),
	}

	err := encodeFrame(buf, frame)
	require.Nil(t, err)

	encoded := buf.Bytes()

	require.Equal(t, 11+len(frame.data), len(encoded))

	decoded, err := decodeFrame(bytes.NewBuffer(encoded), make([]byte, maxFrameSize))
	require.Nil(t, err)

	require.Equal(t, frame, decoded)
}

func TestVarIntEncoding(t *testing.T) {
	nums := []int{
		1,
		32,
		104,
		2234,
		16844676,
		184141156514464,
	}

	for _, n := range nums {
		buf := make([]byte, 32)
		m1, err := encodeVarint(buf, n)
		require.Nil(t, err)

		decoded, m2, err := decodeVarint(buf[:m1])
		require.Nil(t, err)
		require.Equal(t, m1, m2)

		require.Equal(t, n, decoded)
	}
}

func TestStringEncoding(t *testing.T) {
	str := "zadbadbadbaidba"

	buf := make([]byte, 16)

	n, err := encodeString(buf, str)
	require.Nil(t, err)
	require.Equal(t, 16, n)
	require.Equal(t, byte(15), buf[0])
	require.Equal(t, str, string(buf[1:]))

	decoded, n, err := decodeString(buf)
	require.Nil(t, err)
	require.Equal(t, 16, n)
	require.Equal(t, str, decoded)
}

func TestKVEncoding(t *testing.T) {
	buf := make([]byte, 512)

	vars := map[string]interface{}{
		"string": "value",
		"int":    24,
		"true":   true,
		"false":  false,
	}

	off := 0

	for k, v := range vars {
		n, err := encodeKV(buf[off:], k, v)
		require.Nil(t, err)
		off += n
	}

	decoded, n, err := decodeKVs(buf[:off], -1)
	require.Nil(t, err)
	require.Equal(t, off, n)
	require.Equal(t, vars, decoded)
}
