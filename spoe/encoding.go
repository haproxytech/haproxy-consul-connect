package spoe

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/pkg/errors"
)

type dataType byte

const (
	dataTypeNull   dataType = 0
	dataTypeBool   dataType = 1
	dataTypeInt32  dataType = 2
	dataTypeUInt32 dataType = 3
	dataTypeInt64  dataType = 4
	dataTypeUInt64 dataType = 5
	dataTypeIPV4   dataType = 6
	dataTypeIPV6   dataType = 7
	dataTypeString dataType = 8
	dataTypeBinary dataType = 9
)

const (
	dataTypeMask byte = 0x0F
	dataFlagMask byte = 0xF0

	dataFlagTrue byte = 0x10
)

func decodeUint32(b []byte) (uint32, int, error) {
	// read the frame length
	if len(b) < 4 {
		return 0, 0, fmt.Errorf("decode uint32: need at least 4 bytes, got %d", len(b))
	}

	v := binary.BigEndian.Uint32(b)
	return v, 4, nil
}

func decodeVarint(b []byte) (int, int, error) {
	val := int(b[0])
	off := 1

	if val < 240 {
		return val, 1, nil
	}

	r := uint(4)
	for {
		if off > len(b)-1 {
			return 0, 0, fmt.Errorf("decode varint: unterminated sequence")
		}

		v := int(b[off])
		val += v << r
		off++
		r += 7

		if v < 128 {
			break
		}
	}

	return val, off, nil
}

func encodeVarint(b []byte, i int) (int, error) {
	if len(b) == 0 {
		return 0, fmt.Errorf("encode varint: insufficient space in buffer")
	}

	if i < 240 {
		b[0] = byte(i)
		return 1, nil
	}

	n := 0

	b[n] = byte(i) | 240
	n++
	i = (i - 240) >> 4
	for i >= 128 {
		if n > len(b)-1 {
			return 0, fmt.Errorf("encode varint: insufficient space in buffer")
		}

		b[n] = byte(i) | 128
		n++
		i = (i - 128) >> 7
	}

	if n > len(b)-1 {
		return 0, fmt.Errorf("encode varint: insufficient space in buffer")
	}

	b[n] = byte(i)
	n++

	return n, nil
}

func decodeBytes(b []byte) ([]byte, int, error) {
	l, off, err := decodeVarint(b)
	if err != nil {
		return nil, 0, errors.Wrap(err, "decode bytes")
	}

	if len(b) < l+off {
		return nil, 0, fmt.Errorf("decode bytes: unterminated sequence")
	}

	return b[off : off+l], off + l, nil
}

func encodeBytes(b []byte, v []byte) (int, error) {
	l := len(v)
	n, err := encodeVarint(b, l)
	if err != nil {
		return 0, err
	}

	if l+n > len(b) {
		return 0, fmt.Errorf("encode bytes: insufficient space in buffer")
	}

	copy(b[n:], v)
	return n + l, nil
}

func decodeIPV4(b []byte) (net.IP, int, error) {
	if len(b) < net.IPv4len {
		return nil, 0, fmt.Errorf("decode ipv4: unterminated sequence")
	}

	return net.IP(b[:net.IPv4len]), net.IPv4len, nil
}

func encodeIPV4(b []byte, ip net.IP) (int, error) {
	if len(b) < net.IPv4len {
		return 0, fmt.Errorf("decode ipv4: unterminated sequence")
	}

	copy(b, ip)
	return net.IPv4len, nil
}

func encodeIPV6(b []byte, ip net.IP) (int, error) {
	if len(b) < net.IPv6len {
		return 0, fmt.Errorf("decode ipv4: unterminated sequence")
	}

	copy(b, ip)
	return net.IPv6len, nil
}

func decodeIPV6(b []byte) (net.IP, int, error) {
	if len(b) < net.IPv4len {
		return nil, 0, fmt.Errorf("decode ipv6: unterminated sequence")
	}

	return net.IP(b[:net.IPv4len]), net.IPv4len, nil
}

func decodeString(b []byte) (string, int, error) {
	b, n, err := decodeBytes(b)
	return string(b), n, err
}

func encodeString(b []byte, v string) (int, error) {
	return encodeBytes(b, []byte(v))
}

func decodeKVs(b []byte, count int) (map[string]interface{}, int, error) {
	res := make(map[string]interface{})
	off := 0

	for off < len(b) && (count == -1 || len(res) < count) {
		name, n, err := decodeString(b[off:])
		if err != nil {
			return nil, 0, errors.Wrap(err, "decode k/v")
		}
		off += n

		var value interface{}

		dbyte := b[off]
		dtype := dataType(dbyte & dataTypeMask)
		off++

		switch dtype {
		case dataTypeNull:
			// noop
		case dataTypeBool:
			value = dbyte&dataFlagTrue > 0

		case dataTypeInt32, dataTypeInt64:
			v, n, err := decodeVarint(b[off:])
			if err != nil {
				return nil, 0, errors.Wrap(err, "decode k/v")
			}
			off += n
			value = int(v)

		case dataTypeUInt32, dataTypeUInt64:
			v, n, err := decodeVarint(b[off:])
			if err != nil {
				return nil, 0, errors.Wrap(err, "decode k/v")
			}
			off += n
			value = uint(v)

		case dataTypeIPV4:
			v, n, err := decodeIPV4(b[off:])
			if err != nil {
				return nil, 0, errors.Wrap(err, "decode k/v")
			}
			off += n
			value = v

		case dataTypeIPV6:
			v, n, err := decodeIPV6(b[off:])
			if err != nil {
				return nil, 0, errors.Wrap(err, "decode k/v")
			}
			off += n
			value = v
		case dataTypeString:
			v, n, err := decodeString(b[off:])
			if err != nil {
				return nil, 0, errors.Wrap(err, "decode k/v")
			}
			off += n
			value = v

		case dataTypeBinary:
			v, n, err := decodeBytes(b[off:])
			if err != nil {
				return nil, 0, errors.Wrap(err, "decode k/v")
			}
			off += n
			value = v
		default:
			return nil, 0, fmt.Errorf("decode k/v: unknown data type %x", dtype)
		}

		res[name] = value
	}

	return res, off, nil
}

func encodeKV(b []byte, name string, v interface{}) (int, error) {
	n, err := encodeString(b, name)
	if err != nil {
		return 0, errors.Wrapf(err, "encode k/v (%s): %s", name, err)
	}

	if len(b) == n {
		return 0, fmt.Errorf("encode k/v (%s): insufficient space", name)
	}

	var m int
	switch val := v.(type) {
	case int:
		b[n] = byte(dataTypeInt64)
		n++
		m, err = encodeVarint(b[n:], int(val))
	case int64:
		b[n] = byte(dataTypeInt64)
		n++
		m, err = encodeVarint(b[n:], int(val))
	case uint:
		b[n] = byte(dataTypeUInt64)
		n++
		m, err = encodeVarint(b[n:], int(val))
	case uint64:
		b[n] = byte(dataTypeUInt64)
		n++
		m, err = encodeVarint(b[n:], int(val))
	case int32:
		b[n] = byte(dataTypeInt32)
		n++
		m, err = encodeVarint(b[n:], int(val))
	case uint32:
		b[n] = byte(dataTypeUInt32)
		n++
		m, err = encodeVarint(b[n:], int(val))
	case string:
		b[n] = byte(dataTypeString)
		n++
		m, err = encodeString(b[n:], val)
	case []byte:
		b[n] = byte(dataTypeBinary)
		n++
		m, err = encodeBytes(b[n:], val)
	case net.IP:
		if v4 := val.To4(); len(v4) > 0 {
			b[n] = byte(dataTypeIPV4)
			n++
			m, err = encodeIPV4(b[n:], v4)
		} else {
			b[n] = byte(dataTypeIPV6)
			n++
			m, err = encodeIPV6(b[n:], val)
		}
	case bool:
		v := byte(dataTypeBool)
		if val {
			v |= dataFlagTrue
		}
		b[n] = v
		n++
	default:
		return 0, fmt.Errorf("encode k/v (%s): type %T is not handled", name, v)
	}

	return n + m, nil
}
