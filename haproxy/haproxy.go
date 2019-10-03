package haproxy

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/criteo/haproxy-consul-connect/consul"
	"github.com/criteo/haproxy-consul-connect/haproxy/dataplane"
	"github.com/criteo/haproxy-consul-connect/haproxy/state"
	"github.com/criteo/haproxy-consul-connect/lib"
	spoe "github.com/criteo/haproxy-spoe-go"
	"github.com/haproxytech/models"
	"github.com/hashicorp/consul/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/mcuadros/go-syslog.v2"
)

type HAProxy struct {
	opts            Options
	dataplaneClient *dataplane.Dataplane
	consulClient    *api.Client
	cfgC            chan consul.Config
	currentCfg      *consul.Config

	oldState state.State

	haConfig *haConfig

	Ready chan (struct{})
}

func New(consulClient *api.Client, cfg chan consul.Config, opts Options) *HAProxy {
	if opts.HAProxyBin == "" {
		opts.HAProxyBin = "haproxy"
	}
	if opts.DataplaneBin == "" {
		opts.DataplaneBin = "dataplaneapi"
	}
	return &HAProxy{
		opts:         opts,
		consulClient: consulClient,
		cfgC:         cfg,
		Ready:        make(chan struct{}),
	}
}

func (h *HAProxy) Run(sd *lib.Shutdown) error {
	init := false
	statsStarted := false
	for {
		select {
		case c := <-h.cfgC:
			if !init {
				err := h.start(sd)
				if err != nil {
					return err
				}
				init = true
				close(h.Ready)
			}
			err := h.handleChange(c)
			if err != nil {
				log.Error(err)
			}
			if !statsStarted {
				err = h.startStats()
				if err != nil {
					log.Error(err)
				}
				statsStarted = true
			}
		case <-sd.Stop:
			return nil
		}
	}
}

func (h *HAProxy) start(sd *lib.Shutdown) error {
	hc, err := newHaConfig(h.opts.ConfigBaseDir, sd)
	if err != nil {
		return err
	}
	h.haConfig = hc

	h.dataplaneClient = dataplane.New(
		"http://unix-sock",
		dataplaneUser,
		dataplanePass,
		&http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Dial: func(proto, addr string) (conn net.Conn, err error) {
					return net.Dial("unix", h.haConfig.DataplaneSock)
				},
			},
		},
	)

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

	haCmd, err := h.startHAProxy(sd)
	if err != nil {
		return err
	}
	err = h.startDataplane(sd, haCmd)
	if err != nil {
		return err
	}

	tx := h.dataplaneClient.Tnx()

	timeout := int64(30000)
	err = tx.CreateBackend(models.Backend{
		Name:           "spoe_back",
		ServerTimeout:  &timeout,
		ConnectTimeout: &timeout,
		Mode:           models.BackendModeTCP,
	})
	if err != nil {
		return err
	}

	err = tx.CreateServer("spoe_back", models.Server{
		Name:    "haproxy_connect",
		Address: fmt.Sprintf("unix@%s", h.haConfig.SPOESock),
	})

	err = tx.Commit()
	if err != nil {
		return err
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

func (h *HAProxy) startHAProxy(sd *lib.Shutdown) (*exec.Cmd, error) {
	haCmd, err := runCommand(sd,
		syscall.SIGTERM,
		h.opts.HAProxyBin,
		"-f",
		h.haConfig.HAProxy,
	)
	if err != nil {
		return nil, err
	}

	return haCmd, nil
}

func (h *HAProxy) startSPOA() error {
	spoeAgent := spoe.New(NewSPOEHandler(h.consulClient, func() consul.Config {
		return *h.currentCfg
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

func (h *HAProxy) startDataplane(sd *lib.Shutdown, haCmd *exec.Cmd) error {
	_, err := runCommand(sd,
		syscall.SIGTERM,
		h.opts.DataplaneBin,
		"--scheme", "unix",
		"--socket-path", h.haConfig.DataplaneSock,
		"--haproxy-bin", h.opts.HAProxyBin,
		"--config-file", h.haConfig.HAProxy,
		"--reload-cmd", fmt.Sprintf("kill -SIGUSR2 %d", haCmd.Process.Pid),
		"--reload-delay", "0",
		"--userlist", "controller",
		"--transaction-dir", h.haConfig.DataplaneTransactionDir,
	)
	if err != nil {
		return err
	}

	// wait for startup
	for i := time.Duration(0); i < (5*time.Second)/(100*time.Millisecond); i++ {
		select {
		case <-sd.Stop:
			return nil
		default:
		}

		err = h.dataplaneClient.Ping()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		break
	}
	if err != nil {
		return fmt.Errorf("timeout waiting for dataplaneapi: %s", err)
	}

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
				ID:   fmt.Sprintf("%s-connect-stats", h.currentCfg.ServiceID),
				Name: fmt.Sprintf("%s-connect-stats", h.currentCfg.ServiceName),
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
		service: h.currentCfg.ServiceName,
	}).Run()
	go func() {
		http.Handle("/metrics", promhttp.Handler())

		log.Infof("Starting stats server at %s", h.opts.StatsListenAddr)
		http.ListenAndServe(h.opts.StatsListenAddr, nil)
	}()

	return nil
}
