package consul

import (
	"crypto/x509"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/aestek/haproxy-connect/spoe"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	consul "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

type SPOEHandler struct {
	serviceName string
	c           *consul.Client
	rootCAs     func() *x509.CertPool
}

func NewSPOEHandler(c *consul.Client, serviceName string, rootCAs func() *x509.CertPool) *SPOEHandler {
	return &SPOEHandler{
		c:           c,
		serviceName: serviceName,
		rootCAs:     rootCAs,
	}
}

func (h *SPOEHandler) Handler(args []spoe.Message) ([]spoe.Action, error) {
	for _, m := range args {
		if m.Name != "check-intentions" {
			continue
		}

		certBytes, ok := m.Args["cert"].([]byte)
		if !ok {
			return nil, fmt.Errorf("spoe handler: expected cert bytes in message")
		}

		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, errors.Wrap(err, "spoe handler")
		}

		_, err = cert.Verify(x509.VerifyOptions{
			Roots: h.rootCAs(),
		})
		if err != nil {
			log.Warn("connect: error validating certificate: %s", err)
		}

		authorized := err == nil

		if authorized {
			certURI, err := connect.ParseCertURI(cert.URIs[0])
			if err != nil {
				log.Printf("connect: invalid leaf certificate URI")
				return nil, errors.New("connect: invalid leaf certificate URI")
			}

			// Perform AuthZ
			resp, err := h.c.Agent().ConnectAuthorize(&api.AgentAuthorizeParams{
				Target:           h.serviceName,
				ClientCertURI:    certURI.URI().String(),
				ClientCertSerial: connect.HexString(cert.SerialNumber.Bytes()),
			})
			if err != nil {
				return nil, errors.Wrap(err, "spoe handler: authz call failed")
			}

			log.Debugf("spoe: auth response from %s authorized=%v", certURI.URI().String(), resp.Authorized)

			authorized = resp.Authorized
		}

		res := 1
		if !authorized {
			res = 0
		}
		return []spoe.Action{
			spoe.ActionSetVar{
				Name:  "auth",
				Scope: spoe.VarScopeSession,
				Value: res,
			},
		}, nil
	}
	return nil, nil
}
