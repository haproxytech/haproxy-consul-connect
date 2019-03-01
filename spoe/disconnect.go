package spoe

import (
	"fmt"

	"github.com/pkg/errors"
)

func (c *conn) handleDisconnect(f frame) error {
	data, _, err := decodeKVs(f.data, -1)
	if err != nil {
		return errors.Wrap(err, "disconnect")
	}

	if code, ok := data["status-code"].(uint32); ok && code == 0 {
		return nil
	}

	message, _ := data["message"].(string)
	if message == "" {
		message = "unknown error "
	}

	return fmt.Errorf("disconnect error: %s", message)
}
