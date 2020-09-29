package consul

import (
	"testing"
	"time"

	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func startAgent(t *testing.T, sd *lib.Shutdown) *api.Client {
	a := agent.NewTestAgent(t, t.Name(), ``)
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	sd.Add(1)
	go func() {
		defer sd.Done()
		<-sd.Stop
		a.Shutdown()
	}()

	client, err := api.NewClient(&api.Config{Address: a.HTTPAddr()})
	require.NoError(t, err)

	return client
}

var testCases = []struct {
	name     string
	reg      *api.AgentServiceRegistration
	expected Config
}{
	{
		name: "simple",
		reg: &api.AgentServiceRegistration{
			Name: "client",
			ID:   "client-inst",
			Port: 8080,
			Connect: &api.AgentServiceConnect{
				SidecarService: &api.AgentServiceRegistration{
					Proxy: &api.AgentServiceConnectProxyConfig{},
				},
			},
		},
		expected: Config{
			ServiceName: "client",
			ServiceID:   "client-inst",
			Downstream: Downstream{
				LocalBindAddress: "0.0.0.0",
				LocalBindPort:    21000,
				TargetAddress:    "127.0.0.1",
				TargetPort:       8080,
				ConnectTimeout:   DefaultConnectTimeout,
				ReadTimeout:      DefaultReadTimeout,
			},
		},
	},
	{
		name: "downstream timeouts",
		reg: &api.AgentServiceRegistration{
			Name: "client",
			ID:   "client-inst",
			Port: 8080,
			Connect: &api.AgentServiceConnect{
				SidecarService: &api.AgentServiceRegistration{
					Proxy: &api.AgentServiceConnectProxyConfig{
						Config: map[string]interface{}{
							"connect_timeout": "1m",
							"read_timeout":    "2m",
						},
					},
				},
			},
		},
		expected: Config{
			ServiceName: "client",
			ServiceID:   "client-inst",
			Downstream: Downstream{
				LocalBindAddress: "0.0.0.0",
				LocalBindPort:    21000,
				TargetAddress:    "127.0.0.1",
				TargetPort:       8080,
				ConnectTimeout:   time.Minute,
				ReadTimeout:      2 * time.Minute,
			},
		},
	},
	{
		name: "upstreams",
		reg: &api.AgentServiceRegistration{
			Name: "client",
			ID:   "client-inst",
			Port: 8080,
			Connect: &api.AgentServiceConnect{
				SidecarService: &api.AgentServiceRegistration{
					Proxy: &api.AgentServiceConnectProxyConfig{
						Upstreams: []api.Upstream{
							{
								DestinationType: "service",
								DestinationName: "server",
								LocalBindPort:   8081,
							},
							{
								DestinationType: "prepared_query",
								DestinationName: "pq-service",
								LocalBindPort:   8082,
							},
						},
					},
				},
			},
		},
		expected: Config{
			ServiceName: "client",
			ServiceID:   "client-inst",
			Downstream: Downstream{
				LocalBindAddress: "0.0.0.0",
				LocalBindPort:    21000,
				TargetAddress:    "127.0.0.1",
				TargetPort:       8080,
				ConnectTimeout:   DefaultConnectTimeout,
				ReadTimeout:      DefaultReadTimeout,
			},
			Upstreams: []Upstream{
				{
					Name:             "prepared_query_pq-service",
					LocalBindAddress: "127.0.0.1",
					LocalBindPort:    8082,
					ConnectTimeout:   DefaultConnectTimeout,
					ReadTimeout:      DefaultReadTimeout,
				},
				{
					Name:             "service_server",
					LocalBindAddress: "127.0.0.1",
					LocalBindPort:    8081,
					ConnectTimeout:   DefaultConnectTimeout,
					ReadTimeout:      DefaultReadTimeout,
				},
			},
		},
	},
	{
		name: "upstream timeouts",
		reg: &api.AgentServiceRegistration{
			Name: "client",
			ID:   "client-inst",
			Port: 8080,
			Connect: &api.AgentServiceConnect{
				SidecarService: &api.AgentServiceRegistration{
					Proxy: &api.AgentServiceConnectProxyConfig{
						Upstreams: []api.Upstream{
							{
								DestinationType: "service",
								DestinationName: "server",
								LocalBindPort:   8081,
								Config: map[string]interface{}{
									"connect_timeout": "1m",
									"read_timeout":    "2m",
								},
							},
							{
								DestinationType: "prepared_query",
								DestinationName: "pq-service",
								LocalBindPort:   8082,
								Config: map[string]interface{}{
									"connect_timeout": "3m",
									"read_timeout":    "4m",
								},
							},
						},
					},
				},
			},
		},
		expected: Config{
			ServiceName: "client",
			ServiceID:   "client-inst",
			Downstream: Downstream{
				LocalBindAddress: "0.0.0.0",
				LocalBindPort:    21000,
				TargetAddress:    "127.0.0.1",
				TargetPort:       8080,
				ConnectTimeout:   DefaultConnectTimeout,
				ReadTimeout:      DefaultReadTimeout,
			},
			Upstreams: []Upstream{
				{
					Name:             "prepared_query_pq-service",
					LocalBindAddress: "127.0.0.1",
					LocalBindPort:    8082,
					ConnectTimeout:   3 * time.Minute,
					ReadTimeout:      4 * time.Minute,
				},
				{
					Name:             "service_server",
					LocalBindAddress: "127.0.0.1",
					LocalBindPort:    8081,
					ConnectTimeout:   time.Minute,
					ReadTimeout:      2 * time.Minute,
				},
			},
		},
	},
}

func TestWatcher(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sd := lib.NewShutdown()
			defer sd.Shutdown("test end")

			consul := startAgent(t, sd)

			err := consul.Agent().ServiceRegister(tc.reg)
			require.NoError(t, err)

			w := New(tc.reg.ID, consul, log.New())

			errs := make(chan error)
			go func() {
				err := w.Run()
				if err != nil {
					errs <- err
				}
			}()

			select {
			case err := <-errs:
				require.NoError(t, err)
			case cfg := <-w.C:
				// clear Certs
				cfg.Downstream.CAs = nil
				cfg.Downstream.Cert = nil
				cfg.Downstream.Key = nil
				for i := range cfg.Upstreams {
					cfg.Upstreams[i].CAs = nil
					cfg.Upstreams[i].Cert = nil
					cfg.Upstreams[i].Key = nil
				}

				require.Equal(t, tc.expected, cfg)
			}
		})
	}

}
