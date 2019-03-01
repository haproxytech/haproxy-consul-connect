package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	"github.com/aestek/haproxy-connect/haproxy/haproxyconfig"
	"github.com/aestek/haproxy-connect/lib"

	"github.com/hashicorp/consul/api"

	"github.com/aestek/haproxy-connect/consul"
)

func main() {
	log.SetLevel(log.TraceLevel)

	consulAddr := flag.String("http-addr", "127.0.0.1:8500", "Consul agent address")
	service := flag.String("sidecar-for", "", "The consul service to proxy")
	haproxy := flag.String("haproxy", "haproxy", "Haproxy binary path")
	haproxyCfgBasePath := flag.String("haproxy-cfg-base-path", "/tmp", "Haproxy binary path")
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

	opts := haproxyconfig.Options{
		Bin:           *haproxy,
		ConfigBaseDir: *haproxyCfgBasePath,
	}
	if haproxy != nil {
		opts.Bin = *haproxy
	}
	hap := haproxyconfig.New(watcher.C, opts)
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
