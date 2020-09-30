package dataplane

import (
	"fmt"
	"net/http"

	"github.com/haproxytech/models/v2"
)

func (c *Dataplane) LogTargets(parentType, parentName string) ([]models.LogTarget, error) {
	type resT struct {
		Data []models.LogTarget `json:"data"`
	}

	var res resT

	err := c.makeReq(http.MethodGet, fmt.Sprintf("/v2/services/haproxy/configuration/log_targets?parent_type=%s&parent_name=%s", parentType, parentName), nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (t *tnx) CreateLogTargets(parentType, parentName string, rule models.LogTarget) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v2/services/haproxy/configuration/log_targets?parent_type=%s&parent_name=%s&transaction_id=%s", parentType, parentName, t.txID), rule, nil)
}
