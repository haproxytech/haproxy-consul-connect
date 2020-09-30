package state

import (
	"fmt"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/models/v2"
)

func generateDownstream(opts Options, certStore CertificateStore, cfg consul.Downstream, state State) (State, error) {
	feName := "front_downstream"
	beName := "back_downstream"
	feMode := models.FrontendModeHTTP
	beMode := models.BackendModeHTTP

	caPath, crtPath, err := certStore.CertsPath(cfg.TLS)
	if err != nil {
		return state, err
	}

	if cfg.Protocol != "" && cfg.Protocol == "tcp" {
		feMode = models.FrontendModeTCP
		beMode = models.BackendModeTCP
	}

	// Main config
	fe := Frontend{
		Frontend: models.Frontend{
			Name:           feName,
			DefaultBackend: beName,
			ClientTimeout:  int64p(int(cfg.ReadTimeout.Milliseconds())),
			Mode:           feMode,
			Httplog:        opts.LogRequests,
		},
		Bind: models.Bind{
			Name:           fmt.Sprintf("%s_bind", feName),
			Address:        cfg.LocalBindAddress,
			Port:           int64p(cfg.LocalBindPort),
			Ssl:            true,
			SslCertificate: crtPath,
			SslCafile:      caPath,
			Verify:         models.BindVerifyRequired,
		},
	}

	// Logging
	if opts.LogRequests && opts.LogSocket != "" {
		fe.LogTarget = &models.LogTarget{
			Index:    int64p(0),
			Address:  opts.LogSocket,
			Facility: models.LogTargetFacilityLocal0,
			Format:   models.LogTargetFormatRfc5424,
		}
	}

	// Intentions
	if opts.EnableIntentions {
		fe.Filter = &FrontendFilter{
			Filter: models.Filter{
				Index:      int64p(0),
				Type:       models.FilterTypeSpoe,
				SpoeEngine: "intentions",
				SpoeConfig: opts.SPOEConfigPath,
			},
			Rule: models.TCPRequestRule{
				Index:    int64p(0),
				Action:   models.TCPRequestRuleActionReject,
				Cond:     models.TCPRequestRuleCondUnless,
				CondTest: "{ var(sess.connect.auth) -m int eq 1 }",
				Type:     models.TCPRequestRuleTypeContent,
			},
		}
	}

	state.Frontends = append(state.Frontends, fe)

	var forwardFor *models.Forwardfor
	if cfg.EnableForwardFor && beMode == models.BackendModeHTTP {
		forwardFor = &models.Forwardfor{
			Enabled: stringp(models.ForwardforEnabledEnabled),
		}
	}

	// Backend
	be := Backend{
		Backend: models.Backend{
			Name:           beName,
			ServerTimeout:  int64p(int(cfg.ReadTimeout.Milliseconds())),
			ConnectTimeout: int64p(int(cfg.ConnectTimeout.Milliseconds())),
			Mode:           beMode,
			Forwardfor:     forwardFor,
		},
		Servers: []models.Server{
			{
				Name:    "downstream_node",
				Address: cfg.TargetAddress,
				Port:    int64p(cfg.TargetPort),
			},
		},
	}

	// Logging
	if opts.LogRequests && opts.LogSocket != "" {
		be.LogTarget = &models.LogTarget{
			Index:    int64p(0),
			Address:  opts.LogSocket,
			Facility: models.LogTargetFacilityLocal0,
			Format:   models.LogTargetFormatRfc5424,
		}
	}

	// App name header
	if cfg.AppNameHeaderName != "" && beMode == models.BackendModeHTTP {
		be.HTTPRequestRules = append(be.HTTPRequestRules, models.HTTPRequestRule{
			Index:     int64p(0),
			Type:      models.HTTPRequestRuleTypeAddHeader,
			HdrName:   cfg.AppNameHeaderName,
			HdrFormat: "%[var(sess.connect.source_app)]",
		})
	}

	state.Backends = append(state.Backends, be)

	return state, nil
}
