package haproxy

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/aestek/haproxy-connect/consul"
	"github.com/aestek/haproxy-connect/lib"
	spoe "github.com/criteo/haproxy-spoe-go"
	"github.com/haproxytech/models"
	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

type baseParams struct {
	SocketPath string
	User       string
	Password   string
}

type HAProxy struct {
	opts            Options
	dataplaneClient *dataplaneClient
	consulClient    *api.Client
	cfgC            chan consul.Config
	currentCfg      *consul.Config

	haConfig *haConfig
}

func New(consulClient *api.Client, cfg chan consul.Config, opts Options) *HAProxy {
	return &HAProxy{
		opts:         opts,
		consulClient: consulClient,
		cfgC:         cfg,
	}
}

func (h *HAProxy) Run(sd *lib.Shutdown) error {
	hc, err := newHaConfig(h.opts.ConfigBaseDir, sd)
	if err != nil {
		return err
	}
	h.haConfig = hc

	haCmd, err := runCommand(sd,
		syscall.SIGUSR1,
		h.opts.HAProxyBin,
		"-f",
		h.haConfig.HAProxy,
	)
	if err != nil {
		return err
	}

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

	_, err = runCommand(sd,
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

	h.dataplaneClient = &dataplaneClient{
		addr:     "http://fake",
		userName: "admin",
		password: "mypassword",
		client: &http.Client{
			Timeout: time.Second,
			Transport: &http.Transport{
				Dial: func(proto, addr string) (conn net.Conn, err error) {
					return net.Dial("unix", h.haConfig.DataplaneSock)
				},
			},
		},
		version: 1,
	}

	// wait for startup
	for {
		select {
		case <-sd.Stop:
			return nil
		default:
		}

		err := h.dataplaneClient.Ping()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		break
	}

	//go h.stats()

	err = h.init()
	if err != nil {
		return err
	}

	for {
		select {
		case c := <-h.cfgC:
			err := h.handleChange(c)
			if err != nil {
				log.Error(err)
			}
		case <-sd.Stop:
			return nil
		}
	}
}

func (h *HAProxy) init() error {
	tx, err := h.dataplaneClient.Tnx()
	if err != nil {
		return err
	}

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

func (h *HAProxy) handleChange(cfg consul.Config) error {
	tx, err := h.dataplaneClient.Tnx()
	if err != nil {
		return err
	}

	err = h.handleDownstream(tx, cfg.Downstream)
	if err != nil {
		return err
	}

	for _, up := range cfg.Upstreams {
		err := h.handleUpstream(tx, up)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	h.currentCfg = &cfg

	return nil
}

func (h *HAProxy) handleDownstream(tx *tnx, ds consul.Downstream) error {
	if h.currentCfg != nil && h.currentCfg.Downstream.Equal(ds) {
		return nil
	}

	feName := "front_downstream"
	beName := "back_downstream"

	if h.currentCfg != nil {
		err := tx.DeleteFrontend(feName)
		if err != nil {
			return err
		}
		err = tx.DeleteBackend(beName)
		if err != nil {
			return err
		}
	}

	timeout := int64(1000)
	err := tx.CreateFrontend(models.Frontend{
		Name:           feName,
		DefaultBackend: beName,
		ClientTimeout:  &timeout,
		Mode:           models.FrontendModeHTTP,
	})
	if err != nil {
		return err
	}

	crtPath, caPath, err := h.haConfig.CertsPath(ds.TLS)
	if err != nil {
		return err
	}

	port := int64(ds.LocalBindPort)
	err = tx.CreateBind(feName, models.Bind{
		Name:           fmt.Sprintf("%s_bind", feName),
		Address:        ds.LocalBindAddress,
		Port:           &port,
		Ssl:            true,
		SslCertificate: crtPath,
		SslCafile:      caPath,
	})
	if err != nil {
		return err
	}

	filterID := int64(0)
	err = tx.CreateFilter("frontend", feName, models.Filter{
		Type:       models.FilterTypeSpoe,
		ID:         &filterID,
		SpoeEngine: "intentions",
		SpoeConfig: h.haConfig.SPOE,
	})
	if err != nil {
		return err
	}
	err = tx.CreateTCPRequestRule("frontend", feName, models.TCPRequestRule{
		Action:   models.TCPRequestRuleActionAccept,
		Cond:     models.TCPRequestRuleCondIf,
		CondTest: "{ var(sess.connect.auth) -m int eq 1 }",
		Type:     models.TCPRequestRuleTypeContent,
		ID:       &filterID,
	})
	if err != nil {
		return err
	}

	err = tx.CreateBackend(models.Backend{
		Name:           beName,
		ServerTimeout:  &timeout,
		ConnectTimeout: &timeout,
		Mode:           models.BackendModeHTTP,
	})
	if err != nil {
		return err
	}

	bePort := int64(ds.TargetPort)
	err = tx.CreateServer(beName, models.Server{
		Name:    "downstream_node",
		Address: ds.TargetAddress,
		Port:    &bePort,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *HAProxy) handleUpstream(tx *tnx, up consul.Upstream) error {
	feName := fmt.Sprintf("front_%s", up.Service)
	beName := fmt.Sprintf("back_%s", up.Service)

	var current *consul.Upstream
	if h.currentCfg != nil {
		for _, u := range h.currentCfg.Upstreams {
			if u.Service == up.Service {
				current = &u
				break
			}
		}
	}

	backendDeleted := false
	if current != nil && !current.Equal(up) {
		err := tx.DeleteFrontend(feName)
		if err != nil {
			return err
		}
		err = tx.DeleteBackend(beName)
		if err != nil {
			return err
		}
		backendDeleted = true
	}

	if backendDeleted || current == nil {
		timeout := int64(1000)
		err := tx.CreateFrontend(models.Frontend{
			Name:           feName,
			DefaultBackend: beName,
			ClientTimeout:  &timeout,
			Mode:           models.FrontendModeHTTP,
		})
		if err != nil {
			return err
		}

		port := int64(up.LocalBindPort)
		err = tx.CreateBind(feName, models.Bind{
			Name:    fmt.Sprintf("%s_bind", feName),
			Address: up.LocalBindAddress,
			Port:    &port,
		})
		if err != nil {
			return err
		}

		err = tx.CreateBackend(models.Backend{
			Name:           beName,
			ServerTimeout:  &timeout,
			ConnectTimeout: &timeout,
			Balance: &models.Balance{
				Algorithm: models.BalanceAlgorithmLeastconn,
			},
			Mode: models.BackendModeHTTP,
		})
		if err != nil {
			return err
		}
	}

	currentServers := map[string]consul.UpstreamNode{}
	if !backendDeleted && current != nil {
		for _, b := range current.Nodes {
			currentServers[fmt.Sprintf("%s:%d", b.Host, b.Port)] = b
		}
	}

	certPath, caPath, err := h.haConfig.CertsPath(up.TLS)
	if err != nil {
		return err
	}

	newServers := map[string]consul.UpstreamNode{}
	for _, srv := range up.Nodes {
		id := fmt.Sprintf("%s:%d", srv.Host, srv.Port)
		newServers[id] = srv

		currentSrv, currentExists := currentServers[id]

		changed := currentExists && currentSrv.Weight != srv.Weight
		if !changed && currentExists {
			fmt.Println("already exists")
			continue
		}

		f := tx.CreateServer
		if changed {
			f = tx.ReplaceServer
		}

		fmt.Println(id, srv.Port)
		port := int64(srv.Port)
		weight := int64(srv.Weight)
		serverDef := models.Server{
			Name:           hex.EncodeToString([]byte(id)),
			Address:        srv.Host,
			Port:           &port,
			Weight:         &weight,
			Ssl:            models.ServerSslEnabled,
			SslCertificate: certPath,
			SslCafile:      caPath,
		}

		err := f(beName, serverDef)
		if err != nil {
			return err
		}
	}

	for current := range currentServers {
		_, ok := newServers[current]
		if !ok {
			err := tx.DeleteServer(beName, hex.EncodeToString([]byte(current)))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *HAProxy) stats() {
	for {
		s, err := h.dataplaneClient.Stats()
		if err != nil {
			log.Error(err)
			continue
		}

		fmt.Printf("%+v\n", s)
		time.Sleep(time.Second)
	}
}
