package dataplane

import (
	"fmt"
	"net/http"

	"github.com/haproxytech/models"
)

func (c *Dataplane) Backends() ([]models.Backend, error) {
	type resT struct {
		Data []models.Backend `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, "/v1/services/haproxy/configuration/backends", nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Dataplane) Servers(beName string) ([]models.Server, error) {
	type resT struct {
		Data []models.Server `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, fmt.Sprintf("/v1/services/haproxy/configuration/servers?backend=%s", beName), nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (t *tnx) CreateBackend(be models.Backend) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/backends?transaction_id=%s", t.txID), be, nil)
}

func (t *tnx) DeleteBackend(name string) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodDelete, fmt.Sprintf("/v1/services/haproxy/configuration/backends/%s?transaction_id=%s", name, t.txID), nil, nil)
}

func (t *tnx) CreateServer(beName string, srv models.Server) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/servers?backend=%s&transaction_id=%s", beName, t.txID), srv, nil)
}

func (t *tnx) ReplaceServer(beName string, oldSrvName string, newSrv models.Server) error {
	t.After(func() error {
		return t.client.ReplaceServer(beName, oldSrvName, newSrv)
	})
	return nil
}

func (c *Dataplane) ReplaceServer(beName string, oldSrvName string, newSrv models.Server) error {
	err := c.makeReq(http.MethodPut, fmt.Sprintf("/v1/services/haproxy/configuration/servers/%s?backend=%s&version=%d", oldSrvName, beName, c.version), newSrv, nil)
	if err != nil {
		return err
	}

	c.version++
	return nil
}

func (t *tnx) DeleteServer(beName string, name string) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodDelete, fmt.Sprintf("/v1/services/haproxy/configuration/servers/%s?backend=%s&transaction_id=%s", name, beName, t.txID), nil, nil)
}
