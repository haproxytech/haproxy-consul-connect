package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	haproxy "github.com/aestek/haproxy-connect/haproxy"
	"github.com/aestek/haproxy-connect/lib"

	"github.com/hashicorp/consul/api"

	"github.com/aestek/haproxy-connect/consul"
)

func main() {
	log.SetLevel(log.TraceLevel)

	consulAddr := flag.String("http-addr", "127.0.0.1:8500", "Consul agent address")
	service := flag.String("sidecar-for", "", "The consul service to proxy")
	haproxyBin := flag.String("haproxy", "haproxy", "Haproxy binary path")
	dataplaneBin := flag.String("dataplane", "dataplane-api", "Dataplane binary path")
	haproxyCfgBasePath := flag.String("haproxy-cfg-base-path", "/tmp", "Haproxy binary path")
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
	watcher := consul.New(*service, consulClient)
	go func() {
		if err := watcher.Run(); err != nil {
			log.Error(err)
			sd.Shutdown()
		}
	}()

	hap := haproxy.New(consulClient, watcher.C, haproxy.Options{
		HAProxyBin:       *haproxyBin,
		DataplaneBin:     *dataplaneBin,
		ConfigBaseDir:    *haproxyCfgBasePath,
		EnableIntentions: *enableIntentions,
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
