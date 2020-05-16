package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/asaskevich/govalidator"
	log "github.com/sirupsen/logrus"

	"github.com/haproxytech/haproxy-consul-connect/haproxy"
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
	log.Warnf(format, args...)
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

// HAproxy doesn't exit immediately when you pass incorrect log address, so we try to do it on our own to fail fast
func validateHaproxyLogAddress(logAddress string) error {
	// allowed values taken from https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4.2-log
	fi, err := os.Stat(logAddress)
	if err != nil {
		match, err := regexp.Match(`(fd@<[0-9]+>|stdout|stderr)`, []byte(logAddress))
		if err != nil && match {
			return nil
		}
		if !govalidator.IsHost(logAddress) && !govalidator.IsDialString(logAddress) {
			return errors.New(fmt.Sprintf("%s should be either syslog host[:port] or a socket", logAddress))
		}
	} else {
		if fi.Mode()&os.ModeSocket == 0 {
			return errors.New(fmt.Sprintf("%s is a file but not a socket", logAddress))
		}
	}
	return nil
}

func main() {
	versionFlag := flag.Bool("version", false, "Show version and exit")
	consulAddr := flag.String("http-addr", "127.0.0.1:8500", "Consul agent address")
	service := flag.String("sidecar-for", "", "The consul service id to proxy")
	serviceTag := flag.String("sidecar-for-tag", "", "The consul service id to proxy")
	haproxyBin := flag.String("haproxy", haproxy_cmd.DefaultHAProxyBin, "Haproxy binary path")
	dataplaneBin := flag.String("dataplane", haproxy_cmd.DefaultDataplaneBin, "Dataplane binary path")
	haproxyCfgBasePath := flag.String("haproxy-cfg-base-path", "/tmp", "Generated Haproxy configs path")
	haproxyLogRequests := flag.Bool("haproxy-log-requests", false, "Enable logging requests by Haproxy")
	haproxyLogAddress := flag.String("haproxy-log-address", "", "Address for Haproxy logs (default stderr with this app logs)")
	logLevel := flag.String("log-level", "INFO", "This app log level")
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

	if *haproxyLogAddress != "" {
		if err := validateHaproxyLogAddress(*haproxyLogAddress); err != nil {
			log.Fatal(err)
		}
	}

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
		HAProxyLogRequests:   *haproxyLogRequests,
		HAProxyLogAddress:    *haproxyLogAddress,
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
