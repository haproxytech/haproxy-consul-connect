package main

import (
	"fmt"
	"io/ioutil"
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

		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{
						api.Upstream{
							DestinationName: "target",
							LocalBindPort:   1501,
						},
					},
				},
			},
		},
	}))

	require.NoError(t, client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: "target",
		Port: 1600,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{},
			},
		},
	}))

	fmt.Println("Services registered")

	sd := lib.NewShutdown()

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
		err := http.ListenAndServe(":1600", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("hello connect"))
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

	for i := 0; i < 300; i++ {
		var res *http.Response
		res, err = http.Get("http://127.0.0.1:1501")
		if err == nil {
			ioutil.ReadAll(res.Body)
			res.Body.Close()
		}
		if err == nil && res.StatusCode != 503 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Get("http://127.0.0.1:1501")
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)

	body, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Equal(t, "hello connect", string(body))

	sd.Shutdown("test end")
	sd.Wait()
}
