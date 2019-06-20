package consul

import (
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/aestek/haproxy-connect/spoe"

	"github.com/aestek/haproxy-connect/haproxy"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/connect/proxy"
	log "github.com/sirupsen/logrus"
)

const (
	defaultDownstreamBindAddr = "0.0.0.0"
	defaultUpstreamBindAddr   = "127.0.0.1"

	errorWaitTime = 5 * time.Second
)

type upstream struct {
	LocalBindAddress string
	LocalPort        int
	Service          string
	Datacenter       string
	Nodes            []*api.ServiceEntry

	done bool
}

type downstream struct {
	LocalBindAddress string
	LocalPort        int
	RemoteAddress    string
	RemotePort       int
}

type certLeaf struct {
	Cert haproxy.Secret
	Key  haproxy.Secret

	done bool
}

type Watcher struct {
	service     string
	serviceName string
	consul      *api.Client
	token       string
	C           chan haproxy.Configuration

	lock  sync.Mutex
	ready sync.WaitGroup

	upstreams  map[string]*upstream
	downstream downstream
	CertCA     []haproxy.Secret
	CertCAPool *x509.CertPool
	leafs      map[string]*certLeaf
	spoePort   int

	update chan struct{}
}

func New(service string, consul *api.Client) *Watcher {
	return &Watcher{
		service: service,
		consul:  consul,

		C:         make(chan haproxy.Configuration),
		upstreams: make(map[string]*upstream),
		update:    make(chan struct{}, 1),
		leafs:     make(map[string]*certLeaf),
	}
}

func (w *Watcher) Run() error {
	proxyID, err := proxy.LookupProxyIDForSidecar(w.consul, w.service)
	if err != nil {
		return err
	}

	svc, _, err := w.consul.Agent().Service(w.service, &api.QueryOptions{})
	if err != nil {
		return err
	}

	w.serviceName = svc.Service

	w.ready.Add(5)

	go w.runSPOE()
	go w.watchCA()
	go w.watchLeaf(w.serviceName)
	go w.watchService(proxyID, w.handleProxyChange)
	go w.watchService(w.service, func(first bool, srv *api.AgentService) {
		w.downstream.RemotePort = srv.Port
		if first {
			w.ready.Done()
		}
	})

	w.ready.Wait()

	for range w.update {
		w.C <- w.genCfg()
	}

	return nil
}

func (w *Watcher) handleProxyChange(first bool, srv *api.AgentService) {
	w.downstream.LocalBindAddress = defaultDownstreamBindAddr
	w.downstream.LocalPort = srv.Port
	w.downstream.RemoteAddress = defaultUpstreamBindAddr
	if srv.Connect != nil && srv.Connect.Proxy != nil && srv.Connect.Proxy.Config != nil {
		if b, ok := srv.Connect.Proxy.Config["bind_address"].(string); ok {
			w.downstream.LocalBindAddress = b
		}
		if a, ok := srv.Connect.Proxy.Config["local_service_address"].(string); ok {
			w.downstream.RemoteAddress = a
		}
	}

	keep := make(map[string]bool)

	if srv.Proxy != nil {
		for _, up := range srv.Proxy.Upstreams {
			keep[up.DestinationName] = true
			w.lock.Lock()
			_, ok := w.upstreams[up.DestinationName]
			w.lock.Unlock()
			if !ok {
				w.startUpstream(up)
				w.watchLeaf(up.DestinationName)
			}
		}
	}

	for name := range w.upstreams {
		if !keep[name] {
			w.removeUpstream(name)
		}
	}

	if first {
		w.ready.Done()
	}
}

func (w *Watcher) startUpstream(up api.Upstream) {
	log.Infof("consul: watching upstream for service %s", up.DestinationName)

	u := &upstream{
		LocalBindAddress: up.LocalBindAddress,
		LocalPort:        up.LocalBindPort,
		Service:          up.DestinationName,
		Datacenter:       up.Datacenter,
	}

	w.lock.Lock()
	w.upstreams[up.DestinationName] = u
	w.lock.Unlock()

	go func() {
		index := uint64(0)
		for {
			if u.done {
				return
			}
			nodes, meta, err := w.consul.Health().Connect(up.DestinationName, "", true, &api.QueryOptions{
				Datacenter: up.Datacenter,
				WaitTime:   10 * time.Minute,
				WaitIndex:  index,
			})
			if err != nil {
				log.Errorf("consul: error fetching service definition for service %s: %s", up.DestinationName, err)
				time.Sleep(errorWaitTime)
				index = 0
				continue
			}
			changed := index != meta.LastIndex
			index = meta.LastIndex

			if changed {
				w.lock.Lock()
				u.Nodes = nodes
				w.lock.Unlock()
				w.notifyChanged()
			}
		}
	}()
}

func (w *Watcher) removeUpstream(name string) {
	log.Infof("consul: removing upstream for service %s", name)

	w.lock.Lock()
	w.upstreams[name].done = true
	delete(w.upstreams, name)
	w.lock.Unlock()
}

func (w *Watcher) watchLeaf(service string) {
	log.Debugf("consul: watching leaf cert for %s", service)

	var lastIndex uint64
	first := true
	for {
		// if the upsteam was removed, stop watching its leaf
		_, upstreamRunning := w.upstreams[service]
		if service != w.serviceName && !upstreamRunning {
			log.Debugf("consul: stopping watching leaf cert for %s", service)
			return
		}

		cert, meta, err := w.consul.Agent().ConnectCALeaf(service, &api.QueryOptions{
			WaitTime:  10 * time.Minute,
			WaitIndex: lastIndex,
		})
		if err != nil {
			log.Errorf("consul error fetching leaf cert for service %s: %s", service, err)
			time.Sleep(errorWaitTime)
			lastIndex = 0
			continue
		}

		changed := lastIndex != meta.LastIndex
		lastIndex = meta.LastIndex

		if changed {
			log.Debugf("consul: leaf cert for service %s changed", service)
			w.lock.Lock()
			if _, ok := w.leafs[service]; !ok {
				w.leafs[service] = &certLeaf{}
			}
			w.leafs[service].Cert = haproxy.Secret(cert.CertPEM)
			w.leafs[service].Key = haproxy.Secret(cert.PrivateKeyPEM)
			w.lock.Unlock()
			w.notifyChanged()
		}

		if first {
			log.Debugf("consul: leaf cert for %s ready", service)
			w.ready.Done()
			first = false
		}
	}
}

func (w *Watcher) watchService(service string, handler func(first bool, srv *api.AgentService)) {
	log.Infof("consul: wacthing service %s", service)

	hash := ""
	first := true
	for {
		srv, meta, err := w.consul.Agent().Service(service, &api.QueryOptions{
			WaitHash: hash,
			WaitTime: 10 * time.Minute,
		})
		if err != nil {
			log.Errorf("consul: error fetching service definition: %s", err)
			time.Sleep(errorWaitTime)
			hash = ""
			continue
		}

		changed := hash != meta.LastContentHash
		hash = meta.LastContentHash

		if changed {
			log.Debugf("consul: service %s changed", service)
			handler(first, srv)
			w.notifyChanged()
		}

		first = false
	}
}

func (w *Watcher) watchCA() {
	log.Debugf("consul: watching ca certs")

	first := true
	var lastIndex uint64
	for {
		caList, meta, err := w.consul.Agent().ConnectCARoots(&api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  10 * time.Minute,
		})
		if err != nil {
			log.Errorf("consul: error fetching cas: %s", err)
			time.Sleep(errorWaitTime)
			lastIndex = 0
			continue
		}

		changed := lastIndex != meta.LastIndex
		lastIndex = meta.LastIndex

		if changed {
			log.Debugf("consul: CA certs changed")
			w.lock.Lock()
			w.CertCA = w.CertCA[:0]
			w.CertCAPool = x509.NewCertPool()
			for _, ca := range caList.Roots {
				w.CertCA = append(w.CertCA, haproxy.Secret(ca.RootCertPEM))
				ok := w.CertCAPool.AppendCertsFromPEM([]byte(ca.RootCertPEM))
				if !ok {
					log.Warn("consul: unable to add CA certificate to pool")
				}
			}
			w.lock.Unlock()
			w.notifyChanged()
		}

		if first {
			log.Debugf("consul: CA certs ready")
			w.ready.Done()
			first = false
		}
	}
}

func (w *Watcher) runSPOE() {
	spoeAgent := spoe.New(NewSPOEHandler(w.consul, w.service, func() *x509.CertPool {
		w.lock.Lock()
		p := w.CertCAPool
		w.lock.Unlock()
		return p
	}).Handler)

	lis, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		log.Fatal("error starting spoe agent:", err)
	}

	go func() {
		err = spoeAgent.Serve(lis)
		if err != nil {
			log.Fatal("error starting spoe agent:", err)
		}
	}()

	_, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)

	w.lock.Lock()
	defer w.lock.Unlock()
	w.spoePort = port
	w.ready.Done()
}

func (w *Watcher) genCfg() haproxy.Configuration {
	w.lock.Lock()
	defer w.lock.Unlock()

	config := haproxy.Configuration{
		Frontends: []haproxy.Frontend{
			haproxy.Frontend{
				Name:           "inbound_front",
				BindAddr:       w.downstream.LocalBindAddress,
				BindPort:       w.downstream.LocalPort,
				DefaultBackend: "inbound_back",

				SPOE: true,

				TLS:      true,
				ClientCA: w.CertCA,

				ServerCRT: w.leafs[w.serviceName].Cert,
				ServerKey: w.leafs[w.serviceName].Key,
			},
		},
		Backends: []haproxy.Backend{
			haproxy.Backend{
				Name: "inbound_back",
				Servers: []haproxy.BackendServer{
					haproxy.BackendServer{
						Name: "inbound_back_srv",
						Host: w.downstream.RemoteAddress,
						Port: w.downstream.RemotePort,
					},
				},
			},
			haproxy.Backend{
				Name: "spoe_back",
				Servers: []haproxy.BackendServer{
					haproxy.BackendServer{
						Name: "spoe_back_srv",
						Host: "127.0.0.1",
						Port: w.spoePort,
					},
				},
			},
		},
	}

	for _, up := range w.upstreams {
		config.Frontends = append(config.Frontends, haproxy.Frontend{
			Name:           fmt.Sprintf("%s_front", up.Service),
			BindAddr:       up.LocalBindAddress,
			BindPort:       up.LocalPort,
			DefaultBackend: fmt.Sprintf("%s_back", up.Service),
		})

		backend := haproxy.Backend{
			Name: fmt.Sprintf("%s_back", up.Service),
		}

		for i, s := range up.Nodes {
			host := s.Service.Address
			if host == "" {
				host = s.Node.Address
			}
			backend.Servers = append(backend.Servers, haproxy.BackendServer{
				Name: fmt.Sprintf("%s_back_%d", up.Service, i),
				Host: host,
				Port: s.Service.Port,

				TLS:       true,
				ClientCRT: w.leafs[w.serviceName].Cert,
				ClientKey: w.leafs[w.serviceName].Key,
				ServerCA:  w.CertCA,
			})
		}

		config.Backends = append(config.Backends, backend)
	}

	return config
}

func (w *Watcher) notifyChanged() {
	select {
	case w.update <- struct{}{}:
	default:
	}
}
