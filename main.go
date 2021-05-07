package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	haproxy "github.com/haproxytech/haproxy-consul-connect/haproxy"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/haproxy_cmd"
	"github.com/haproxytech/haproxy-consul-connect/lib"

	"github.com/hashicorp/consul/api"

	"github.com/haproxytech/haproxy-consul-connect/consul"
)

// Version is set by Travis build
var Version string = "v0.1.9-Dev"

// BuildTime is set by Travis
var BuildTime string = "2020-01-01T00:00:00Z"

// GitHash The last reference Hash from Git
var GitHash string = "unknown"

type consulLogger struct{}

// Debugf Display debug message
func (consulLogger) Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// Infof Display info message
func (consulLogger) Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Warnf Display warning message
func (consulLogger) Warnf(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Errorf Display error message
func (consulLogger) Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

// validateRequirements Checks that dependencies are present
func validateRequirements(dataplaneBin, haproxyBin string) error {
	err := haproxy_cmd.CheckEnvironment(dataplaneBin, haproxyBin)
	if err != nil {
		msg := fmt.Sprintf("Some external dependencies are missing: %s", err.Error())
		os.Stderr.WriteString(fmt.Sprintf("%s\n", msg))
		return err
	}
	return nil
}

func main() {
	haproxyParamsFlag := stringSliceFlag{}

	flag.Var(&haproxyParamsFlag, "haproxy-param", "Global or defaults Haproxy config parameter to set in config. Can be specified multiple times. Must be of the form `defaults.name=value` or `global.name=value`")
	versionFlag := flag.Bool("version", false, "Show version and exit")
	logLevel := flag.String("log-level", "INFO", "Log level")
	consulAddr := flag.String("http-addr", "127.0.0.1:8500", "Consul agent address")
	service := flag.String("sidecar-for", "", "The consul service id to proxy")
	serviceTag := flag.String("sidecar-for-tag", "", "The consul service id to proxy")
	haproxyBin := flag.String("haproxy", haproxy_cmd.DefaultHAProxyBin, "Haproxy binary path")
	dataplaneBin := flag.String("dataplane", haproxy_cmd.DefaultDataplaneBin, "Dataplane binary path")
	haproxyCfgBasePath := flag.String("haproxy-cfg-base-path", "/tmp", "Haproxy binary path")
	statsListenAddr := flag.String("stats-addr", "", "Listen addr for stats server")
	statsServiceRegister := flag.Bool("stats-service-register", false, "Register a consul service for connect stats")
	enableIntentions := flag.Bool("enable-intentions", false, "Enable Connect intentions")
	token := flag.String("token", "", "Consul ACL token")
	flag.Parse()
	if versionFlag != nil && *versionFlag {
		fmt.Printf("Version: %s ; BuildTime: %s ; GitHash: %s\n", Version, BuildTime, GitHash)
		status := 0
		if err := validateRequirements(*dataplaneBin, *haproxyBin); err != nil {
			fmt.Printf("ERROR: dataplane API / HAProxy dependencies are not satisfied: %s\n", err)
			status = 4
		}
		os.Exit(status)
	}

	ll, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(ll)

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
		if serviceID == "" {
			log.Fatalf("No sidecar proxy found for service with tag %s", *serviceTag)
		}
	} else if *service != "" {
		serviceID = *service
	} else {
		log.Fatalf("Please specify -sidecar-for or -sidecar-for-tag")
	}

	haproxyParams, err := makeHAProxyParams(haproxyParamsFlag)
	if err != nil {
		log.Fatal(err)
	}

	consulLogger := &consulLogger{}
	watcher := consul.New(serviceID, consulClient, consulLogger)
	go func() {
		if err := watcher.Run(); err != nil {
			log.Error(err)
			sd.Shutdown(err.Error())
		}
	}()

	hap := haproxy.New(consulClient, watcher.C, haproxy.Options{
		HAProxyBin:           *haproxyBin,
		DataplaneBin:         *dataplaneBin,
		ConfigBaseDir:        *haproxyCfgBasePath,
		EnableIntentions:     *enableIntentions,
		StatsListenAddr:      *statsListenAddr,
		StatsRegisterService: *statsServiceRegister,
		LogRequests:          ll == log.TraceLevel,
		HAProxyParams:        haproxyParams,
	})
	sd.Add(1)
	go func() {
		defer sd.Done()
		if err := hap.Run(sd); err != nil {
			log.Error(err)
			sd.Shutdown(err.Error())
		}
	}()

	sd.Wait()
}
