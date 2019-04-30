package spoe

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type frameType byte

const (
	frameTypeUnset = 0

	// Frames sent by HAProxy
	frameTypeHaproxyHello  frameType = 1
	frameTypeHaproxyDiscon frameType = 2
	frameTypeHaproxyNotify frameType = 3

	// Frames sent by the agents
	frameTypeAgentHello  frameType = 101
	frameTypeAgentDiscon frameType = 102
	frameTypeAgentACK    frameType = 103
)

type frameFlag uint32

const (
	frameFlagFin  = 1
	frameFlagAbrt = 2
)

type frame struct {
	ftype          frameType
	flags          frameFlag
	streamID       int
	frameID        int
	originalBuffer []byte
	data           []byte
}

func decodeFrame(r io.Reader, buffer []byte) (frame, error) {
	frame := frame{
		originalBuffer: buffer,
	}

	// read the frame length
	_, err := io.ReadFull(r, buffer[:4])
	if err != nil {
		errors.Wrap(err, "frame read")
	}
	frameLength, _, err := decodeUint32(buffer[:4])
	if err != nil {
		return frame, nil
	}
	
	if frameLength > maxFrameSize {
		return frame, errors.New("frame length")
	}

	frame.data = buffer[:frameLength]

	// read the frame data
	_, err = io.ReadFull(r, frame.data)
	if err != nil {
		return frame, errors.Wrap(err, "frame read")
	}

	off := 0
	if len(frame.data) == 0 {
		return frame, fmt.Errorf("frame read: empty frame")
	}

	frame.ftype = frameType(frame.data[0])
	off++

	flags, n, err := decodeUint32(frame.data[off:])
	if err != nil {
		return frame, errors.Wrap(err, "frame read")
	}

	off += n
	frame.flags = frameFlag(flags)

	streamID, n, err := decodeVarint(frame.data[off:])
	if err != nil {
		return frame, errors.Wrap(err, "frame read")
	}
	off += n

	frameID, n, err := decodeVarint(frame.data[off:])
	if err != nil {
		return frame, errors.Wrap(err, "frame read")
	}
	off += n

	frame.streamID = streamID
	frame.frameID = frameID
	frame.data = frame.data[off:]
	return frame, nil
}

func encodeFrame(w io.Writer, f frame) error {
	header := make([]byte, 19)
	off := 4

	header[off] = byte(f.ftype)
	off++

	binary.BigEndian.PutUint32(header[off:], uint32(f.flags))
	off += 4

	n, err := encodeVarint(header[off:], f.streamID)
	if err != nil {
		return errors.Wrap(err, "write frame")
	}
	off += n

	n, err = encodeVarint(header[off:], f.frameID)
	if err != nil {
		return errors.Wrap(err, "write frame")
	}
	off += n

	binary.BigEndian.PutUint32(header, uint32(off-4+len(f.data)))

	_, err = w.Write(header[:off])
	if err != nil {
		return errors.Wrap(err, "write frame")
	}

	_, err = w.Write(f.data)
	if err != nil {
		return errors.Wrap(err, "write frame")
	}

	return nil
}
