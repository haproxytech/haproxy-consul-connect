package dataplane

import (
	"fmt"
	"net/http"

	"github.com/haproxytech/models"
)

func (c *Dataplane) Frontends() ([]models.Frontend, error) {
	type resT struct {
		Data []models.Frontend `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, "/v1/services/haproxy/configuration/frontends", nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Dataplane) Binds(feName string) ([]models.Bind, error) {
	type resT struct {
		Data []models.Bind `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, fmt.Sprintf("/v1/services/haproxy/configuration/binds?frontend=%s", feName), nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (t *tnx) CreateFrontend(fe models.Frontend) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/frontends?transaction_id=%s", t.txID), fe, nil)
}

func (t *tnx) DeleteFrontend(name string) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodDelete, fmt.Sprintf("/v1/services/haproxy/configuration/frontends/%s?transaction_id=%s", name, t.txID), nil, nil)
}

func (t *tnx) CreateBind(feName string, bind models.Bind) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/binds?frontend=%s&transaction_id=%s", feName, t.txID), bind, nil)
}
