package spoe

import (
	"github.com/pkg/errors"
)

type Message struct {
	Name string
	Args map[string]interface{}
}

func (c *conn) handleNotify(f frame) (frame, error) {
	off := 0

	messages := []Message{}

	for off < len(f.data) {
		messageName, n, err := decodeString(f.data[off:])
		if err != nil {
			return f, errors.Wrap(err, "handle notify")
		}
		off += n

		nbArgs := int(f.data[off])
		off++

		kv, n, err := decodeKVs(f.data[off:], nbArgs)
		if err != nil {
			return f, errors.Wrap(err, "handle notify")
		}
		off += n

		messages = append(messages, Message{
			Name: messageName,
			Args: kv,
		})
	}

	actions, err := c.handler(messages)
	if err != nil {
		return f, errors.Wrap(err, "handle notify") // TODO return proper response
	}

	f.ftype = frameTypeAgentACK
	f.flags = frameFlagFin

	off = 0

	for _, a := range actions {
		n, err := a.encode(f.data[off:])
		if err != nil {
			return f, errors.Wrap(err, "handle notify")
		}
		off += n
	}

	f.data = f.data[:off]

	return f, nil
}
