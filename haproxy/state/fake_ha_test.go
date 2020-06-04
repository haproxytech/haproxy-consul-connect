package state

import (
	"testing"

	"github.com/haproxytech/models"
	"github.com/stretchr/testify/require"
)

type fakeHAOpType int

const (
	haOpCreateFrontend fakeHAOpType = iota
	haOpDeleteFrontend
	haOpCreateBind
	haOpDeleteBackend
	haOpCreateBackend
	haOpCreateServer
	haOpReplaceServer
	haOpDeleteServer
	haOpCreateFilter
	haOpCreateTCPRequestRule
	haOpCreateLogTargets
	haOpCreateHTTPRequestRule
)

type fakeHAOp struct {
	Type fakeHAOpType
	Name string
	Args interface{}
}

type fakeHA struct {
	ops []fakeHAOp
}

type requireFn func(t *testing.T, o fakeHAOp)

func (h *fakeHA) RequireOps(t *testing.T, fns ...requireFn) {
	require.Equal(t, len(fns), len(h.ops))
	for i := range fns {
		fns[i](t, h.ops[i])
	}
}

func RequireOp(opType fakeHAOpType, name string) requireFn {
	return func(t *testing.T, o fakeHAOp) {
		require.Equal(t, opType, o.Type)
		require.Equal(t, name, o.Name)
	}
}

func (h *fakeHA) CreateFrontend(fe models.Frontend) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateFrontend,
		Name: fe.Name,
	})
	return nil
}

func (h *fakeHA) DeleteFrontend(name string) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpDeleteFrontend,
		Name: name,
	})
	return nil
}

func (h *fakeHA) CreateBind(feName string, bind models.Bind) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateBind,
		Name: bind.Name,
	})
	return nil
}

func (h *fakeHA) DeleteBackend(name string) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpDeleteBackend,
		Name: name,
	})
	return nil
}

func (h *fakeHA) CreateBackend(be models.Backend) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateBackend,
		Name: be.Name,
	})
	return nil
}

func (h *fakeHA) CreateServer(beName string, srv models.Server) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateServer,
		Name: srv.Name,
	})
	return nil
}

func (h *fakeHA) ReplaceServer(beName string, oldSrvName string, newSrv models.Server) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpReplaceServer,
		Name: oldSrvName,
	})
	return nil
}

func (h *fakeHA) DeleteServer(beName string, name string) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpDeleteServer,
		Name: name,
	})
	return nil
}

func (h *fakeHA) CreateFilter(parentType, parentName string, filter models.Filter) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateFilter,
		Name: parentName,
	})
	return nil
}

func (h *fakeHA) CreateTCPRequestRule(parentType, parentName string, rule models.TCPRequestRule) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateTCPRequestRule,
		Name: parentName,
	})
	return nil
}

func (h *fakeHA) CreateLogTargets(parentType, parentName string, rule models.LogTarget) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateLogTargets,
		Name: parentName,
	})
	return nil
}

func (h *fakeHA) CreateHTTPRequestRule(parentType, parentName string, rule models.HTTPRequestRule) error {
	h.ops = append(h.ops, fakeHAOp{
		Type: haOpCreateHTTPRequestRule,
		Name: parentName,
	})
	return nil
}
