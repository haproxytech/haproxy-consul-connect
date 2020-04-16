package haproxy

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	spoe "github.com/criteo/haproxy-spoe-go"
	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/dataplane"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/haproxy_cmd"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/state"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/hashicorp/consul/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/mcuadros/go-syslog.v2"
)

type HAProxy struct {
	opts            Options
	dataplaneClient *dataplane.Dataplane
	consulClient    *api.Client

	cfgC chan consul.Config

	currentConsulConfig *consul.Config
	currentHAProxyState state.State

	haConfig *haConfig

	Ready chan struct{}
}

func New(consulClient *api.Client, cfg chan consul.Config, opts Options) *HAProxy {
	if opts.HAProxyBin == "" {
		opts.HAProxyBin = haproxy_cmd.DefaultHAProxyBin
	}
	if opts.DataplaneBin == "" {
		opts.DataplaneBin = haproxy_cmd.DefaultDataplaneBin
	}
	return &HAProxy{
		opts:         opts,
		consulClient: consulClient,
		cfgC:         cfg,
		Ready:        make(chan struct{}),
	}
}

func (h *HAProxy) Run(sd *lib.Shutdown) error {
	return h.watch(sd)
}

func (h *HAProxy) start(sd *lib.Shutdown) error {
	hc, err := newHaConfig(h.opts.ConfigBaseDir, sd)
	if err != nil {
		return err
	}
	h.haConfig = hc

	if h.opts.LogRequests {
		err := h.startLogger()
		if err != nil {
			return err
		}
	}

	if h.opts.EnableIntentions {
		err := h.startSPOA()
		if err != nil {
			return err
		}
	}

	dpc, err := haproxy_cmd.Start(sd, haproxy_cmd.Config{
		HAProxyPath:             h.opts.HAProxyBin,
		HAProxyConfigPath:       hc.HAProxy,
		DataplanePath:           h.opts.DataplaneBin,
		DataplaneTransactionDir: hc.DataplaneTransactionDir,
		DataplaneSock:           hc.DataplaneSock,
		DataplaneUser:           dataplaneUser,
		DataplanePass:           dataplanePass,
	})
	if err != nil {
		return err
	}
	h.dataplaneClient = dpc

	err = h.startStats()
	if err != nil {
		log.Error(err)
	}

	return nil
}

func (h *HAProxy) startLogger() error {
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	server.ListenUnixgram(h.haConfig.LogsSock)
	server.Boot()

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			log.Infof("%s: %s", logParts["app_name"], logParts["message"])
		}
	}(channel)

	return nil
}

func (h *HAProxy) startSPOA() error {
	spoeAgent := spoe.New(NewSPOEHandler(h.consulClient, func() consul.Config {
		return *h.currentConsulConfig
	}).Handler)

	lis, err := net.Listen("unix", h.haConfig.SPOESock)
	if err != nil {
		log.Fatal("error starting spoe agent:", err)
	}

	go func() {
		err = spoeAgent.Serve(lis)
		if err != nil {
			log.Fatal("error starting spoe agent:", err)
		}
	}()

	return nil
}

func (h *HAProxy) startStats() error {
	if h.opts.StatsListenAddr == "" {
		return nil
	}
	go func() {
		if !h.opts.StatsRegisterService {
			return
		}

		_, portStr, err := net.SplitHostPort(h.opts.StatsListenAddr)
		if err != nil {
			log.Errorf("cannot parse stats listen addr: %s", err)
		}
		port, _ := strconv.Atoi(portStr)

		reg := func() {
			err = h.consulClient.Agent().ServiceRegister(&api.AgentServiceRegistration{
				ID:   fmt.Sprintf("%s-connect-stats", h.currentConsulConfig.ServiceID),
				Name: fmt.Sprintf("%s-connect-stats", h.currentConsulConfig.ServiceName),
				Port: port,
				Checks: api.AgentServiceChecks{
					&api.AgentServiceCheck{
						HTTP:                           fmt.Sprintf("http://localhost:%d/metrics", port),
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
	}()
	go (&Stats{
		dpapi:   h.dataplaneClient,
		service: h.currentConsulConfig.ServiceName,
	}).Run()
	go func() {
		http.Handle("/metrics", promhttp.Handler())

		log.Infof("Starting stats server at %s", h.opts.StatsListenAddr)
		http.ListenAndServe(h.opts.StatsListenAddr, nil)
	}()

	return nil
}
