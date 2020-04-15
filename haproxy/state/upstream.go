package state

import (
	"fmt"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
)

func generateUpstream(opts Options, certStore CertificateStore, cfg consul.Upstream, oldState, newState State) (State, error) {
	feName := fmt.Sprintf("front_%s", cfg.Service)
	beName := fmt.Sprintf("back_%s", cfg.Service)

	fePort64 := int64(cfg.LocalBindPort)
	fe := Frontend{
		Frontend: models.Frontend{
			Name:           feName,
			DefaultBackend: beName,
			ClientTimeout:  &clientTimeout,
			Mode:           models.FrontendModeHTTP,
			Httplog:        opts.LogRequests,
		},
		Bind: models.Bind{
			Name:    fmt.Sprintf("%s_bind", feName),
			Address: cfg.LocalBindAddress,
			Port:    &fePort64,
		},
	}
	if opts.LogRequests && opts.LogSocket != "" {
		fe.LogTarget = &models.LogTarget{
			ID:       int64p(0),
			Address:  opts.LogSocket,
			Facility: models.LogTargetFacilityLocal0,
			Format:   models.LogTargetFormatRfc5424,
		}
	}

	newState.Frontends = append(newState.Frontends, fe)

	be := Backend{
		Backend: models.Backend{
			Name:           beName,
			ServerTimeout:  &serverTimeout,
			ConnectTimeout: &connectTimeout,
			Balance: &models.Balance{
				Algorithm: models.BalanceAlgorithmLeastconn,
			},
			Mode: models.BackendModeHTTP,
		},
	}
	if opts.LogRequests && opts.LogSocket != "" {
		be.LogTarget = &models.LogTarget{
			ID:       int64p(0),
			Address:  opts.LogSocket,
			Facility: models.LogTargetFacilityLocal0,
			Format:   models.LogTargetFormatRfc5424,
		}
	}

	servers, err := generateUpstreamServers(opts, certStore, cfg, beName, oldState)
	if err != nil {
		return newState, err
	}
	be.Servers = servers
	newState.Backends = append(newState.Backends, be)

	return newState, nil
}

func generateUpstreamServers(opts Options, certStore CertificateStore, cfg consul.Upstream, beName string, oldState State) ([]models.Server, error) {
	oldBackend, _ := oldState.findBackend(beName)

	idxHANode := func(s models.Server) string {
		if s.Maintenance == models.ServerMaintenanceEnabled {
			return "maint"
		}
		return fmt.Sprintf("%s:%d", s.Address, *s.Port)
	}
	idxConsulNode := func(s consul.UpstreamNode) string {
		return fmt.Sprintf("%s:%d", s.Host, s.Port)
	}

	servers := make([]models.Server, len(oldBackend.Servers))
	copy(servers, oldBackend.Servers)
	serversIdx := index(servers, func(i int) string {
		return idxHANode(servers[i])
	})

	newServersIdx := index(cfg.Nodes, func(i int) string {
		return idxConsulNode(cfg.Nodes[i])
	})

	caPath, crtPath, err := certStore.CertsPath(cfg.TLS)
	if err != nil {
		return nil, err
	}

	disabledServer := models.Server{
		Address:        "127.0.0.1",
		Port:           int64p(1),
		Weight:         int64p(1),
		Ssl:            models.ServerSslEnabled,
		SslCertificate: crtPath,
		SslCafile:      caPath,
		Verify:         models.BindVerifyRequired,
		Maintenance:    models.ServerMaintenanceEnabled,
	}

	emptyServerSlots := make([]int, 0, len(servers))

	// Disable removed servers
	for i, s := range servers {
		_, ok := newServersIdx[idxHANode(s)]
		if ok {
			continue
		}

		servers[i] = disabledServer
		servers[i].Name = fmt.Sprintf("srv_%d", i)
		emptyServerSlots = append(emptyServerSlots, i)
	}

	// Add new servers
	for _, s := range cfg.Nodes {
		_, ok := serversIdx[idxConsulNode(s)]
		if ok {
			continue
		}

		if len(emptyServerSlots) == 0 {
			l := len(servers)
			add := l
			if add == 0 {
				add = 1
			}
			for i := 0; i < add; i++ {
				server := disabledServer
				server.Name = fmt.Sprintf("srv_%d", i+l)
				servers = append(servers, server)
				emptyServerSlots = append(emptyServerSlots, i+l)
			}
		}

		i := emptyServerSlots[0]
		emptyServerSlots = emptyServerSlots[1:]

		servers[i].Address = s.Host
		servers[i].Port = int64p(s.Port)
		servers[i].Weight = int64p(s.Weight)
		servers[i].Maintenance = models.ServerMaintenanceDisabled
	}

	return servers, nil
}
