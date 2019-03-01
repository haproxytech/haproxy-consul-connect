package spoe

import (
	"fmt"

	"github.com/pkg/errors"
)

type varScope byte

const (
	VarScopeProcess     varScope = 0
	VarScopeSession     varScope = 1
	VarScopeTransaction varScope = 2
	VarScopeRequest     varScope = 3
	VarScopeResponse    varScope = 4
)

const (
	actionTypeSetVar   byte = 1
	actionTypeUnsetVar byte = 2
)

type Action interface {
	encode([]byte) (int, error)
}

type ActionSetVar struct {
	Name  string
	Scope varScope
	Value interface{}
}

func (a ActionSetVar) encode(b []byte) (int, error) {
	if len(b) < 3 {
		return 0, fmt.Errorf("encode action: insufficient space in buffer")
	}

	b[0] = actionTypeSetVar
	b[1] = 3
	b[2] = byte(a.Scope)

	off := 3

	n, err := encodeKV(b[off:], a.Name, a.Value)
	if err != nil {
		return 0, errors.Wrap(err, "encode action")
	}
	off += n

	return off, nil
}

type ActionUnsetVar struct {
	Name  string
	Scope varScope
}

func (a ActionUnsetVar) encode(b []byte) (int, error) {
	if len(b) < 3 {
		return 0, fmt.Errorf("encode action: insufficient space in buffer")
	}

	b[0] = actionTypeUnsetVar
	b[1] = 2
	b[2] = byte(a.Scope)

	off := 3

	n, err := encodeString(b[off:], a.Name)
	if err != nil {
		return 0, errors.Wrap(err, "encode action")
	}
	off += n

	return off, nil
}
