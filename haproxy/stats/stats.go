package stats

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/haproxytech/haproxy-consul-connect/haproxy/dataplane"
	"github.com/hashicorp/consul/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	RegisterService bool
	ListenAddr      string
	ServiceName     string
	ServiceID       string
	ServiceAddr     string
}

type Stats struct {
	cfg          Config
	consulClient *api.Client
	dpapi        *dataplane.Dataplane
	ready        chan struct{}
}

func New(consulClient *api.Client, dpapi *dataplane.Dataplane, ready chan struct{}, cfg Config) *Stats {
	return &Stats{
		cfg:          cfg,
		consulClient: consulClient,
		dpapi:        dpapi,
		ready:        ready,
	}
}

func (s *Stats) Run() error {
	if s.cfg.RegisterService {
		go s.register()
	}

	go s.runMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/ready", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		<-s.ready
		rw.Write([]byte("ready"))
	}))
	mux.Handle("/health", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		ok := false
		select {
		case <-s.ready:
			ok = true
		default:
		}

		if ok {
			rw.Write([]byte("ok"))
		} else {
			rw.WriteHeader(500)
			rw.Write([]byte("starting..."))
		}
	}))

	log.Infof("Starting stats server at %s", s.cfg.ListenAddr)
	err := http.ListenAndServe(s.cfg.ListenAddr, mux)
	if err != nil {
		log.Errorf("error starting stats server: %s", err)
	}

	return nil
}

func (s *Stats) register() {
	_, portStr, err := net.SplitHostPort(s.cfg.ListenAddr)
	if err != nil {
		log.Errorf("cannot parse stats listen addr: %s", err)
	}
	port, _ := strconv.Atoi(portStr)

	serviceCheckAddr := "localhost"
	if s.cfg.ServiceAddr != "" {
		serviceCheckAddr = s.cfg.ServiceAddr
	}

	reg := func() {
		err = s.consulClient.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID:      fmt.Sprintf("%s-connect-stats", s.cfg.ServiceID),
			Name:    fmt.Sprintf("%s-connect-stats", s.cfg.ServiceName),
			Address: s.cfg.ServiceAddr,
			Port:    port,
			Checks: api.AgentServiceChecks{
				&api.AgentServiceCheck{
					HTTP:                           fmt.Sprintf("http://%s:%d/metrics", serviceCheckAddr, port),
					Interval:                       (10 * time.Second).String(),
					DeregisterCriticalServiceAfter: time.Minute.String(),
				},
			},
			Tags: []string{"connect-stats"},
		})
		if err != nil {
			log.Errorf("cannot register stats service: %s", err)
		}
	}

	reg()

	for range time.Tick(time.Minute) {
		reg()
	}
}
