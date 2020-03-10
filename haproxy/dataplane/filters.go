package dataplane

import (
	"fmt"
	"net/http"

	"github.com/haproxytech/models/v2"
)

func (c *Dataplane) Filters(parentType, parentName string) ([]models.Filter, error) {
	type resT struct {
		Data []models.Filter `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, fmt.Sprintf("/v2/services/haproxy/configuration/filters?parent_type=%s&parent_name=%s", parentType, parentName), nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Dataplane) TCPRequestRules(parentType, parentName string) ([]models.TCPRequestRule, error) {
	type resT struct {
		Data []models.TCPRequestRule `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, fmt.Sprintf("/v2/services/haproxy/configuration/tcp_request_rules?parent_type=%s&parent_name=%s", parentType, parentName), nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (t *tnx) CreateFilter(parentType, parentName string, filter models.Filter) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v2/services/haproxy/configuration/filters?parent_type=%s&parent_name=%s&transaction_id=%s", parentType, parentName, t.txID), filter, nil)
}

func (t *tnx) CreateTCPRequestRule(parentType, parentName string, rule models.TCPRequestRule) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v2/services/haproxy/configuration/tcp_request_rules?parent_type=%s&parent_name=%s&transaction_id=%s", parentType, parentName, t.txID), rule, nil)
}
