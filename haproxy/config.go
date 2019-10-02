package haproxy

import (
	"io/ioutil"
	"os"
	"path"
	"runtime"

	"text/template"

	"github.com/criteo/haproxy-consul-connect/lib"
	log "github.com/sirupsen/logrus"
)

const (
	dataplaneUser = "haproxy"
	dataplanePass = "pass"
)

var baseCfgTmpl = `
global
	master-worker
    stats socket {{.SocketPath}} mode 600 level admin expose-fd listeners
    stats timeout 2m
	tune.ssl.default-dh-param 1024
	nbproc 1
	nbthread {{.NbThread}}

userlist controller
	user {{.DataplaneUser}} insecure-password {{.DataplanePass}}
`

const spoeConfTmpl = `
[intentions]

spoe-agent intentions-agent
    messages check-intentions

    option var-prefix connect

    timeout hello      3000ms
    timeout idle       3000s
    timeout processing 3000ms

    use-backend spoe_back

spoe-message check-intentions
    args ip=src cert=ssl_c_der
    event on-frontend-tcp-request
`

type baseParams struct {
	NbThread      int
	SocketPath    string
	DataplaneUser string
	DataplanePass string
	LogsPath      string
}

type haConfig struct {
	Base                    string
	HAProxy                 string
	SPOE                    string
	SPOESock                string
	StatsSock               string
	DataplaneSock           string
	DataplaneTransactionDir string
	LogsSock                string
}

func newHaConfig(baseDir string, sd *lib.Shutdown) (*haConfig, error) {
	cfg := &haConfig{}

	sd.Add(1)
	base, err := ioutil.TempDir(baseDir, "haproxy-connect-")
	if err != nil {
		sd.Done()
		return nil, err
	}
	go func() {
		defer sd.Done()
		<-sd.Stop
		log.Info("cleaning config...")
		os.RemoveAll(base)
	}()

	cfg.Base = base

	cfg.HAProxy = path.Join(base, "haproxy.conf")
	cfg.SPOE = path.Join(base, "spoe.conf")
	cfg.SPOESock = path.Join(base, "spoe.sock")
	cfg.StatsSock = path.Join(base, "haproxy.sock")
	cfg.DataplaneSock = path.Join(base, "dataplane.sock")
	cfg.DataplaneTransactionDir = path.Join(base, "dataplane-transactions")
	cfg.LogsSock = path.Join(base, "logs.sock")

	tmpl, err := template.New("cfg").Parse(baseCfgTmpl)
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.OpenFile(cfg.HAProxy, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()

	err = tmpl.Execute(cfgFile, baseParams{
		NbThread:      runtime.GOMAXPROCS(0),
		SocketPath:    cfg.StatsSock,
		LogsPath:      cfg.LogsSock,
		DataplaneUser: dataplaneUser,
		DataplanePass: dataplanePass,
	})
	if err != nil {
		return nil, err
	}

	spoeCfgFile, err := os.OpenFile(cfg.SPOE, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer spoeCfgFile.Close()
	_, err = spoeCfgFile.WriteString(spoeConfTmpl)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
