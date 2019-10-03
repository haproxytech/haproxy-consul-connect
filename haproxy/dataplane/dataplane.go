package dataplane

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

type Dataplane struct {
	addr               string
	userName, password string
	client             *http.Client
	lock               sync.Mutex
	version            int
}

func New(addr, userName, password string, client *http.Client) *Dataplane {
	return &Dataplane{
		addr:     addr,
		userName: userName,
		password: password,
		client:   client,
		version:  1,
	}
}

type tnx struct {
	txID   string
	client *Dataplane

	after []func() error
}

func (c *Dataplane) Tnx() *tnx {
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

func (c *Dataplane) Info() (*models.ProcessInfo, error) {
	res := &models.ProcessInfo{}
	err := c.makeReq(http.MethodGet, "/services/haproxy/info", nil, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Dataplane) Ping() error {
	return c.makeReq(http.MethodGet, "/v1/specification", nil, nil)
}

func (c *Dataplane) Stats() (models.NativeStats, error) {
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

func (c *Dataplane) makeReq(method, url string, reqData, resData interface{}) error {
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
