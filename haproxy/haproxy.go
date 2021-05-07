package haproxy

import (
	"fmt"
	"net"

	spoe "github.com/criteo/haproxy-spoe-go"
	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/dataplane"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/haproxy_cmd"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/state"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/stats"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/hashicorp/consul/api"
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
	hc, err := newHaConfig(h.opts.ConfigBaseDir, h.opts.HAProxyParams, sd)
	if err != nil {
		return err
	}
	h.haConfig = hc

	return h.watch(sd)
}

func (h *HAProxy) start(sd *lib.Shutdown) error {
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

	var err error
	h.dataplaneClient, err = haproxy_cmd.Start(sd, haproxy_cmd.Config{
		HAProxyPath:             h.opts.HAProxyBin,
		HAProxyConfigPath:       h.haConfig.HAProxy,
		DataplanePath:           h.opts.DataplaneBin,
		DataplaneTransactionDir: h.haConfig.DataplaneTransactionDir,
		DataplaneSock:           h.haConfig.DataplaneSock,
		DataplaneUser:           h.haConfig.DataplaneUser,
		DataplanePass:           h.haConfig.DataplanePass,
	})
	if err != nil {
		return err
	}

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
	err := server.ListenUnixgram(h.haConfig.LogsSock)
	if err != nil {
		return fmt.Errorf("error starting syslog logger: %s", err)
	}
	err = server.Boot()
	if err != nil {
		return fmt.Errorf("error starting syslog logger: %s", err)
	}

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

	s := stats.New(
		h.consulClient,
		h.dataplaneClient,
		h.Ready,
		stats.Config{
			RegisterService: h.opts.StatsRegisterService,
			ListenAddr:      h.opts.StatsListenAddr,
			ServiceName:     h.currentConsulConfig.ServiceName,
			ServiceID:       h.currentConsulConfig.ServiceID,
		})

	go func() {
		err := s.Run()
		if err != nil {
			log.Error(err)
		}
	}()

	return nil
}
