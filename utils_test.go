package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/facebookgo/freeport"
	"github.com/haproxytech/haproxy-consul-connect/consul"
	haproxy "github.com/haproxytech/haproxy-consul-connect/haproxy"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
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

func startConnectService(t *testing.T, sd *lib.Shutdown, client *api.Client, reg *api.AgentServiceRegistration) (chan struct{}, int, map[string]int) {
	sidecarPort, err := freeport.Get()
	require.NoError(t, err)
	reg.Connect.SidecarService.Port = sidecarPort
	reg.Connect.SidecarService.Checks = api.AgentServiceChecks{
		&api.AgentServiceCheck{
			TTL:    time.Hour.String(),
			Status: api.HealthPassing,
		},
	}

	servicePort, err := freeport.Get()
	require.NoError(t, err)
	reg.Port = servicePort

	upstreamPorts := map[string]int{}
	for i, up := range reg.Connect.SidecarService.Proxy.Upstreams {
		upPort, err := freeport.Get()
		require.NoError(t, err)
		reg.Connect.SidecarService.Proxy.Upstreams[i].LocalBindPort = upPort
		upstreamPorts[up.DestinationName] = upPort
	}

	require.NoError(t, client.Agent().ServiceRegister(reg))

	errs := make(chan error, 2)

	watcher := consul.New(reg.ID, client, consul.NewTestingLogger(t))
	go func() {
		err := watcher.Run()
		if err != nil {
			errs <- err
		}
	}()
	watcher.Stop()

	sourceHap := haproxy.New(client, watcher.C, haproxy.Options{
		EnableIntentions: true,
		HAProxyBin:       os.Getenv("HAPROXY"),
		DataplaneBin:     os.Getenv("DATAPLANEAPI"),
	})
	go func() {
		err := sourceHap.Run(sd)
		if err != nil {
			errs <- err
		}
	}()

	done := make(chan struct{})

	go func() {

		select {
		case <-sourceHap.Ready:
		case err := <-errs:
			require.NoError(t, err)
		}

	Outer:
		for {
			res, _, err := client.Health().Connect(reg.Name, "", true, &api.QueryOptions{})
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			for _, i := range res {
				if i.Service.ID != fmt.Sprintf("%s-sidecar-proxy", reg.ID) {
					continue
				}

				if i.Checks.AggregatedStatus() == api.HealthPassing {
					break Outer
				}
			}

			time.Sleep(100 * time.Millisecond)
		}

		for _, up := range reg.Connect.SidecarService.Proxy.Upstreams {
			log.Printf("=== Testing upstreams %s", up.DestinationName)
			for {
				url := fmt.Sprintf("http://127.0.0.1:%d", up.LocalBindPort)
				log.Printf("=== GET %s", url)
				res, _ := http.Get(url)
				if res != nil && res.StatusCode == 200 {
					log.Printf("=== Upstreams %s is ready", up.DestinationName)
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		close(done)
	}()

	return done, servicePort, upstreamPorts
}

func startServer(t *testing.T, sd *lib.Shutdown, port int, response string) {
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)

	sd.Add(1)
	go func() {
		http.Serve(lis, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte(response))
		}))
	}()
	go func() {
		defer sd.Done()
		<-sd.Stop
		lis.Close()
	}()
}

func testGetReq(t *testing.T, port int, expectedCode int, exptectedContent string) {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          0,
			IdleConnTimeout:       0,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d", port), nil)
	req.Close = true
	res, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, expectedCode, res.StatusCode)

	body, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if exptectedContent != "" {
		require.Equal(t, exptectedContent, string(body))
	}
}

func wait(sd *lib.Shutdown, c ...chan struct{}) {
	for _, o := range c {
		select {
		case <-sd.Stop:
			return
		case <-o:
			continue

		}
	}
}
