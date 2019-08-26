package haproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/haproxytech/models"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	dataplaneUser = "haproxy"
	dataplanePass = "pass"
)

type dataplaneClient struct {
	addr               string
	userName, password string
	client             *http.Client
	lock               sync.Mutex
	version            int
}

type tnx struct {
	txID   string
	client *dataplaneClient

	after []func() error
}

func (c *dataplaneClient) Tnx() *tnx {
	return &tnx{
		client: c,
	}
}

func (t *tnx) ensureTnx() error {
	if t.txID != "" {
		return nil
	}
	res := models.Transaction{}
	err := t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/transactions?version=%d", t.client.version), nil, &res)
	if err != nil {
		return err
	}

	t.txID = res.ID

	return nil
}

func (c *dataplaneClient) Info() (*models.ProcessInfo, error) {
	res := &models.ProcessInfo{}
	err := c.makeReq(http.MethodGet, "/services/haproxy/info", nil, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *dataplaneClient) Ping() error {
	return c.makeReq(http.MethodGet, "/v1/specification", nil, nil)
}

func (c *dataplaneClient) Stats() (models.NativeStats, error) {
	res := models.NativeStats{}
	return res, c.makeReq(http.MethodGet, "/v1/services/haproxy/stats/native", nil, &res)
}

func (t *tnx) Commit() error {
	if t.txID != "" {
		err := t.client.makeReq(http.MethodPut, fmt.Sprintf("/v1/services/haproxy/transactions/%s", t.txID), nil, nil)
		if err != nil {
			return err
		}

		t.client.version++
	}

	for _, f := range t.after {
		err := f()
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *tnx) After(fn func() error) {
	t.after = append(t.after, fn)
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

func (t *tnx) DeleteBackend(name string) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodDelete, fmt.Sprintf("/v1/services/haproxy/configuration/backends/%s?transaction_id=%s", name, t.txID), nil, nil)
}

func (t *tnx) CreateBackend(be models.Backend) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/backends?transaction_id=%s", t.txID), be, nil)
}

func (t *tnx) CreateServer(beName string, srv models.Server) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/servers?backend=%s&transaction_id=%s", beName, t.txID), srv, nil)
}

func (t *tnx) ReplaceServer(beName string, srv models.Server) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPut, fmt.Sprintf("/v1/services/haproxy/configuration/servers/%s?backend=%s&transaction_id=%s", srv.Name, beName, t.txID), srv, nil)
}

func (c *dataplaneClient) ReplaceServer(beName string, srv models.Server) error {
	err := c.makeReq(http.MethodPut, fmt.Sprintf("/v1/services/haproxy/configuration/servers/%s?backend=%s&version=%d", srv.Name, beName, c.version), srv, nil)
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

func (t *tnx) CreateFilter(parentType, parentName string, filter models.Filter) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/filters?parent_type=%s&parent_name=%s&transaction_id=%s", parentType, parentName, t.txID), filter, nil)
}

func (t *tnx) CreateTCPRequestRule(parentType, parentName string, rule models.TCPRequestRule) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/tcp_request_rules?parent_type=%s&parent_name=%s&transaction_id=%s", parentType, parentName, t.txID), rule, nil)
}

func (t *tnx) CreateLogTargets(parentType, parentName string, rule models.LogTarget) error {
	if err := t.ensureTnx(); err != nil {
		return err
	}
	return t.client.makeReq(http.MethodPost, fmt.Sprintf("/v1/services/haproxy/configuration/log_targets?parent_type=%s&parent_name=%s&transaction_id=%s", parentType, parentName, t.txID), rule, nil)
}

func (c *dataplaneClient) makeReq(method, url string, reqData, resData interface{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var reqBody io.Reader
	if reqData != nil {
		buf := &bytes.Buffer{}
		err := json.NewEncoder(buf).Encode(reqData)
		if err != nil {
			return errors.Wrapf(err, "error calling %s %s", method, url)
		}
		reqBody = buf
	}

	req, err := http.NewRequest(method, c.addr+url, reqBody)
	if err != nil {
		return errors.Wrapf(err, "error calling %s %s", method, url)
	}
	req.Header.Add("Content-Type", "application/json")

	req.SetBasicAuth(c.userName, c.password)

	log.Debugf("sending dataplane req: %s %s", method, url)
	res, err := c.client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "error calling %s %s", method, url)
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		body, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("error calling %s %s: response was %d: \"%s\"", method, url, res.StatusCode, string(body))
	}

	if resData != nil {
		err = json.NewDecoder(res.Body).Decode(&resData)
		if err != nil {
			return errors.Wrapf(err, "error calling %s %s", method, url)
		}
	}

	return nil
}
