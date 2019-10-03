package state

import (
	"fmt"
	"sort"
	"strings"

	"github.com/criteo/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
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
}

func Generate(opts Options, certStore CertificateStore, oldState State, cfg consul.Config) (State, error) {
	newState := State{}

	var err error

	if opts.EnableIntentions {
		newState.Backends = append(newState.Backends, Backend{
			Backend: models.Backend{
				Name:           "spoe_back",
				ServerTimeout:  int64p(30000),
				ConnectTimeout: int64p(30000),
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

	sort.Slice(newState.Frontends, func(i, j int) bool {
		return strings.Compare(newState.Frontends[i].Frontend.Name, newState.Frontends[j].Frontend.Name) < 0
	})

	sort.Slice(newState.Backends, func(i, j int) bool {
		return strings.Compare(newState.Backends[i].Backend.Name, newState.Backends[j].Backend.Name) < 0
	})

	return newState, nil
}
