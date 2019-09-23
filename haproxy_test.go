package main

import (
	"fmt"
	"testing"
	"time"

	"net/http"

	"github.com/criteo/haproxy-consul-connect/consul"
	haproxy "github.com/criteo/haproxy-consul-connect/haproxy"
	"github.com/criteo/haproxy-consul-connect/lib"
	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	a := agent.NewTestAgent(t, t.Name(), ``)
	defer a.Shutdown()

	fmt.Println("Waiting for leader")
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	client, err := api.NewClient(&api.Config{Address: a.HTTPAddr()})
	require.NoError(t, err)

	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "source",
		Port: 1500,
		Proxy: &api.AgentServiceConnectProxyConfig{
			DestinationServiceID: "source",
			Upstreams: []api.Upstream{
				api.Upstream{
					DestinationName: "target",
					LocalBindPort:   1501,
				},
			},
		},
	}))

	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name:  "target",
		Port:  1600,
		Proxy: &api.AgentServiceConnectProxyConfig{},
	}))

	fmt.Println("Services registered")

	sd := lib.NewShutdown()
	defer sd.Shutdown()

	errs := make(chan error, 20)

	sourceWatcher := consul.New("source", client)
	go func() {
		err := sourceWatcher.Run()
		if err != nil {
			errs <- err
		}
	}()

	sourceHap := haproxy.New(client, sourceWatcher.C, haproxy.Options{})
	go func() {
		err := sourceHap.Run(sd)
		if err != nil {
			errs <- err
		}
	}()

	targetWatcher := consul.New("target", client)
	go func() {
		err := targetWatcher.Run()
		if err != nil {
			errs <- err
		}
	}()

	targetHap := haproxy.New(client, targetWatcher.C, haproxy.Options{})
	go func() {
		err := targetHap.Run(sd)
		if err != nil {
			errs <- err
		}
	}()

	go func() {
		err := http.ListenAndServe(":1500", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("ok"))
		}))
		if err != nil {
			errs <- err
		}
	}()

	fmt.Println("Waiting for haproxies")
	select {
	case <-sourceHap.Ready:
	case err := <-errs:
		t.Error(err)
		return
	}
	select {
	case <-targetHap.Ready:
	case err := <-errs:
		t.Error(err)
		return
	}

	for range time.Tick(time.Second) {
		res, err := http.Get("http://127.0.0.1:1501")
		require.NoError(t, err)

		fmt.Println(res)
	}

}
