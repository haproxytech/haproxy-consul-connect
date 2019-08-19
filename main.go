package main

import (
	"flag"
	"strings"

	log "github.com/sirupsen/logrus"

	haproxy "github.com/criteo/haproxy-consul-connect/haproxy"
	"github.com/criteo/haproxy-consul-connect/lib"

	"github.com/hashicorp/consul/api"

	"github.com/criteo/haproxy-consul-connect/consul"
)

func main() {
	log.SetLevel(log.TraceLevel)

	consulAddr := flag.String("http-addr", "127.0.0.1:8500", "Consul agent address")
	service := flag.String("sidecar-for", "", "The consul service id to proxy")
	serviceTag := flag.String("sidecar-for-tag", "", "The consul service id to proxy")
	haproxyBin := flag.String("haproxy", "haproxy", "Haproxy binary path")
	dataplaneBin := flag.String("dataplane", "dataplane-api", "Dataplane binary path")
	haproxyCfgBasePath := flag.String("haproxy-cfg-base-path", "/tmp", "Haproxy binary path")
	statsListenAddr := flag.String("stats-addr", "", "Listen addr for stats server")
	statsServiceRegister := flag.Bool("stats-service-register", false, "Register a consul service for connect stats")
	enableIntentions := flag.Bool("enable-intentions", false, "Enable Connect intentions")
	token := flag.String("token", "", "Consul ACL token")
	flag.Parse()

	sd := lib.NewShutdown()

	consulConfig := &api.Config{
		Address: *consulAddr,
	}
	if token != nil {
		consulConfig.Token = *token
	}
	consulClient, err := api.NewClient(consulConfig)
	if err != nil {
	}

	var serviceID string
	if *serviceTag != "" {
		svcs, err := consulClient.Agent().Services()
		if err != nil {
			log.Fatal(err)
		}
	OUTER:
		for _, s := range svcs {
			if strings.HasSuffix(s.Service, "sidecar-proxy") {
				continue
			}
			for _, t := range s.Tags {
				if t == *serviceTag {
					serviceID = s.ID
					break OUTER
				}
			}
		}
	} else {
		serviceID = *service
	}

	watcher := consul.New(serviceID, consulClient)
	go func() {
		if err := watcher.Run(); err != nil {
			log.Error(err)
			sd.Shutdown()
		}
	}()

	hap := haproxy.New(consulClient, watcher.C, haproxy.Options{
		HAProxyBin:           *haproxyBin,
		DataplaneBin:         *dataplaneBin,
		ConfigBaseDir:        *haproxyCfgBasePath,
		EnableIntentions:     *enableIntentions,
		StatsListenAddr:      *statsListenAddr,
		StatsRegisterService: *statsServiceRegister,
	})
	sd.Add(1)
	go func() {
		defer sd.Done()
		if err := hap.Run(sd); err != nil {
			log.Error(err)
			sd.Shutdown()
		}
	}()

	sd.Wait()
}
