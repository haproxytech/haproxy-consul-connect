package main

import (
	"fmt"
	"io/ioutil"
	"testing"

	"net/http"

	"github.com/haproxytech/haproxy-consul-connect/haproxy/haproxy_cmd"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	err := haproxy_cmd.CheckEnvironment(haproxy_cmd.DefaultDataplaneBin, haproxy_cmd.DefaultHAProxyBin)
	if err != nil {
		t.Skipf("CANNOT Run test because of missing requirement: %s", err.Error())
	}
	sd := lib.NewShutdown()
	client := startAgent(t, sd)
	defer func() {
		sd.Shutdown("test end")
		sd.Wait()
	}()

	csd, _, upstreamPorts := startConnectService(t, sd, client, &api.AgentServiceRegistration{
		Name: "source",
		ID:   "source-1",

		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{
						api.Upstream{
							DestinationName: "target",
						},
					},
				},
			},
		},
	})

	tsd, servicePort, _ := startConnectService(t, sd, client, &api.AgentServiceRegistration{
		Name: "target",
		ID:   "target-1",

		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{},
			},
		},
	})

	startServer(t, sd, servicePort, "hello connect")
	wait(sd, csd, tsd)
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", upstreamPorts["target"]))
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)

	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, "hello connect", string(body))
}
