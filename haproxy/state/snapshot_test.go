package state

import (
	"sort"
	"strings"
	"testing"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
	"github.com/stretchr/testify/require"
)

func GetTestConsulConfig() consul.Config {
	return consul.Config{
		Downstream: consul.Downstream{
			LocalBindAddress:  "127.0.0.2",
			LocalBindPort:     9999,
			TargetAddress:     "128.0.0.5",
			TargetPort:        8888,
			AppNameHeaderName: "X-App",
		},
		Upstreams: []consul.Upstream{
			consul.Upstream{
				Name:             "service_1",
				LocalBindAddress: "127.0.0.1",
				LocalBindPort:    10000,
				Nodes: []consul.UpstreamNode{
					consul.UpstreamNode{
						Name:    "some-server",
						Address: "1.2.3.4",
						Port:    8080,
						Weight:  5,
					},
					consul.UpstreamNode{
						Name:    "different-server",
						Address: "1.2.3.5",
						Port:    8081,
						Weight:  8,
					},
				},
			},
		},
	}
}

func GetTestHAConfig(baseCfg string) State {
	s := State{
		Frontends: []Frontend{

			// downstream front
			Frontend{
				Frontend: models.Frontend{
					Name:           "front_downstream",
					DefaultBackend: "back_downstream",
					ClientTimeout:  &clientTimeout,
					Mode:           models.FrontendModeHTTP,
					Httplog:        true,
				},
				Bind: models.Bind{
					Name:           "front_downstream_bind",
					Address:        "127.0.0.2",
					Port:           int64p(9999),
					Ssl:            true,
					SslCafile:      baseCfg + "/ca",
					SslCertificate: baseCfg + "/cert",
					Verify:         models.BindVerifyRequired,
				},
				LogTarget: &models.LogTarget{
					ID:       int64p(0),
					Address:  baseCfg + "/logs.sock",
					Facility: models.LogTargetFacilityLocal0,
					Format:   models.LogTargetFormatRfc5424,
				},
				Filter: &FrontendFilter{
					Filter: models.Filter{
						ID:         int64p(0),
						Type:       models.FilterTypeSpoe,
						SpoeEngine: "intentions",
						SpoeConfig: baseCfg + "/spoe",
					},
					Rule: models.TCPRequestRule{
						ID:       int64p(0),
						Action:   models.TCPRequestRuleActionReject,
						Cond:     models.TCPRequestRuleCondUnless,
						CondTest: "{ var(sess.connect.auth) -m int eq 1 }",
						Type:     models.TCPRequestRuleTypeContent,
					},
				},
			},

			// upstream front
			Frontend{
				Frontend: models.Frontend{
					Name:           "front_service_1",
					DefaultBackend: "back_service_1",
					ClientTimeout:  &clientTimeout,
					Mode:           models.FrontendModeHTTP,
					Httplog:        true,
				},
				Bind: models.Bind{
					Name:    "front_service_1_bind",
					Address: "127.0.0.1",
					Port:    int64p(10000),
				},
				LogTarget: &models.LogTarget{
					ID:       int64p(0),
					Address:  baseCfg + "/logs.sock",
					Facility: models.LogTargetFacilityLocal0,
					Format:   models.LogTargetFormatRfc5424,
				},
			},
		},

		Backends: []Backend{

			// downstream backend
			Backend{
				Backend: models.Backend{
					Name:           "back_downstream",
					ServerTimeout:  &serverTimeout,
					ConnectTimeout: &connectTimeout,
					Mode:           models.BackendModeHTTP,
				},
				Servers: []models.Server{
					models.Server{
						Name:    "downstream_node",
						Address: "128.0.0.5",
						Port:    int64p(8888),
					},
				},
				LogTarget: &models.LogTarget{
					ID:       int64p(0),
					Address:  baseCfg + "/logs.sock",
					Facility: models.LogTargetFacilityLocal0,
					Format:   models.LogTargetFormatRfc5424,
				},
				HTTPRequestRules: []models.HTTPRequestRule{
					{
						ID:        int64p(0),
						Type:      models.HTTPRequestRuleTypeAddHeader,
						HdrName:   "X-App",
						HdrFormat: "%[var(sess.connect.source_app)]",
					},
				},
			},

			// upstream backend
			Backend{
				Backend: models.Backend{
					Name:           "back_service_1",
					ServerTimeout:  &serverTimeout,
					ConnectTimeout: &connectTimeout,
					Mode:           models.BackendModeHTTP,
					Balance: &models.Balance{
						Algorithm: models.BalanceAlgorithmLeastconn,
					},
				},
				Servers: []models.Server{
					models.Server{
						Name:           "some-server",
						Address:        "1.2.3.4",
						Port:           int64p(8080),
						Weight:         int64p(5),
						Ssl:            models.ServerSslEnabled,
						SslCafile:      baseCfg + "/ca",
						SslCertificate: baseCfg + "/cert",
						Verify:         models.BindVerifyRequired,
						Maintenance:    models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:           "different-server",
						Address:        "1.2.3.5",
						Port:           int64p(8081),
						Weight:         int64p(8),
						Ssl:            models.ServerSslEnabled,
						SslCafile:      baseCfg + "/ca",
						SslCertificate: baseCfg + "/cert",
						Verify:         models.BindVerifyRequired,
						Maintenance:    models.ServerMaintenanceDisabled,
					},
				},
				LogTarget: &models.LogTarget{
					ID:       int64p(0),
					Address:  baseCfg + "/logs.sock",
					Facility: models.LogTargetFacilityLocal0,
					Format:   models.LogTargetFormatRfc5424,
				},
			},

			// spoe backend
			Backend{
				Backend: models.Backend{
					Name:           "spoe_back",
					ServerTimeout:  &spoeTimeout,
					ConnectTimeout: &spoeTimeout,
					Mode:           models.BackendModeTCP,
				},
				Servers: []models.Server{
					models.Server{
						Name:    "haproxy_connect",
						Address: "unix@",
					},
				},
			},
		},
	}

	sort.Slice(s.Frontends, func(i, j int) bool {
		return strings.Compare(s.Frontends[i].Frontend.Name, s.Frontends[j].Frontend.Name) < 0
	})

	sort.Slice(s.Backends, func(i, j int) bool {
		return strings.Compare(s.Backends[i].Backend.Name, s.Backends[j].Backend.Name) < 0
	})

	return s
}

var TestOpts = Options{
	EnableIntentions: true,
	LogRequests:      true,
	LogSocket:        "//logs.sock",
	SPOEConfigPath:   "//spoe",
}

var TestCertStore = fakeCertStore{}

func TestSnapshotDownstream(t *testing.T) {
	generated, err := Generate(TestOpts, TestCertStore, State{}, GetTestConsulConfig())
	require.Nil(t, err)

	require.Equal(t, GetTestHAConfig("/"), generated)
}

func TestServerUpdate(t *testing.T) {
	consulCfg := GetTestConsulConfig()
	consulCfg.Upstreams[0].Nodes = consulCfg.Upstreams[0].Nodes[1:]

	// remove first server
	expectedRemovedFirstServer := GetTestHAConfig("/")
	expectedRemovedFirstServer.Backends[1].Servers[0].Maintenance = models.ServerMaintenanceEnabled
	expectedRemovedFirstServer.Backends[1].Servers[0].Name = "disabled_server_0"
	expectedRemovedFirstServer.Backends[1].Servers[0].Address = "127.0.0.1"
	expectedRemovedFirstServer.Backends[1].Servers[0].Port = int64p(1)
	expectedRemovedFirstServer.Backends[1].Servers[0].Weight = int64p(1)

	originalState := GetTestHAConfig("/")
	actualRemovedFirstServer, err := Generate(TestOpts, TestCertStore, originalState, consulCfg)

	require.Nil(t, err)
	require.Equal(t, expectedRemovedFirstServer, actualRemovedFirstServer)

	// re-add first server
	originalConsulCfg := GetTestConsulConfig()
	readdedFirstServer, err2 := Generate(TestOpts, TestCertStore, actualRemovedFirstServer, originalConsulCfg)

	require.Nil(t, err2)
	require.Equal(t, originalState, readdedFirstServer)

	// add another one
	addedOneServerConsulCfg := GetTestConsulConfig()
	addedOneServerConsulCfg.Upstreams[0].Nodes = append(addedOneServerConsulCfg.Upstreams[0].Nodes, consul.UpstreamNode{
		Name:    "even-different-server",
		Address: "1.2.3.6",
		Port:    8082,
		Weight:  10,
	})

	expectedAddedOneServer := GetTestHAConfig("/")
	expectedAddedOneServer.Backends[1].Servers = append(expectedAddedOneServer.Backends[1].Servers,
		models.Server{
			Name:           "even-different-server",
			Address:        "1.2.3.6",
			Port:           int64p(8082),
			Weight:         int64p(10),
			Ssl:            models.ServerSslEnabled,
			SslCafile:      "//ca",
			SslCertificate: "//cert",
			Verify:         models.BindVerifyRequired,
			Maintenance:    models.ServerMaintenanceDisabled,
		},
		models.Server{ // because we always make slot for one more
			Name:           "disabled_server_3",
			Address:        "127.0.0.1",
			Port:           int64p(1),
			Weight:         int64p(1),
			Ssl:            models.ServerSslEnabled,
			SslCafile:      "//ca",
			SslCertificate: "//cert",
			Verify:         models.BindVerifyRequired,
			Maintenance:    models.ServerMaintenanceEnabled,
		},
	)

	actualAddedOneServer, err := Generate(TestOpts, TestCertStore, expectedAddedOneServer, addedOneServerConsulCfg)
	require.Nil(t, err)
	require.Equal(t, expectedAddedOneServer, actualAddedOneServer)
}

type fakeCertStore struct{}

func (s fakeCertStore) CertsPath(t consul.TLS) (string, string, error) {
	return "//ca", "//cert", nil
}
