package state

import (
	"fmt"
	"sort"
	"time"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
)

const (
	spoeTimeout = 30 * time.Second
)

type Options struct {
	EnableIntentions bool
	LogRequests      bool
	LogSocket        string
	SPOEConfigPath   string
	SPOESocket       string
}

type CertificateStore interface {
	CertsPath(tls consul.TLS) (string, string, error)
}

type HAProxy interface {
	CreateFrontend(fe models.Frontend) error
	DeleteFrontend(name string) error
	CreateBind(feName string, bind models.Bind) error
	DeleteBackend(name string) error
	CreateBackend(be models.Backend) error
	CreateServer(beName string, srv models.Server) error
	ReplaceServer(beName string, srv models.Server) error
	DeleteServer(beName string, name string) error
	CreateFilter(parentType, parentName string, filter models.Filter) error
	CreateTCPRequestRule(parentType, parentName string, rule models.TCPRequestRule) error
	CreateLogTargets(parentType, parentName string, rule models.LogTarget) error
	CreateHTTPRequestRule(parentType, parentName string, rule models.HTTPRequestRule) error
}

func Generate(opts Options, certStore CertificateStore, oldState State, cfg consul.Config) (State, error) {
	newState := State{}

	var err error

	if opts.EnableIntentions {
		newState.Backends = append(newState.Backends, Backend{
			Backend: models.Backend{
				Name:           "spoe_back",
				ServerTimeout:  int64p(int(spoeTimeout.Milliseconds())),
				ConnectTimeout: int64p(int(spoeTimeout.Milliseconds())),
				Mode:           models.BackendModeTCP,
			},
			Servers: []models.Server{
				models.Server{
					Name:    "haproxy_connect",
					Address: fmt.Sprintf("unix@%s", opts.SPOESocket),
				},
			},
		})
	}

	newState, err = generateDownstream(opts, certStore, cfg.Downstream, newState)
	if err != nil {
		return newState, err
	}

	for _, up := range cfg.Upstreams {
		newState, err = generateUpstream(opts, certStore, up, oldState, newState)
		if err != nil {
			return newState, err
		}
	}

	sort.Sort(Frontends(newState.Frontends))
	sort.Sort(Backends(newState.Backends))

	return newState, nil
}
