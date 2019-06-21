package haproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"

	"text/template"

	"github.com/aestek/haproxy-connect/consul"
	"github.com/aestek/haproxy-connect/lib"
	log "github.com/sirupsen/logrus"
)

var baseCfgTmpl = `
global
    master-worker
    stats socket {{.SocketPath}} mode 600 level admin expose-fd listeners
    stats timeout 2m
    tune.ssl.default-dh-param 1024

userlist controller
    user admin password $5$aVnIFECJ$2QYP64eTTXZ1grSjwwdoQxK/AP8kcOflEO1Q5fc.5aA
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

type haConfig struct {
	Base                    string
	HAProxy                 string
	SPOE                    string
	SPOESock                string
	StatsSock               string
	DataplaneSock           string
	DataplaneTransactionDir string
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

	tmpl, err := template.New("cfg").Parse(baseCfgTmpl)
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.Create(cfg.HAProxy)
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()

	err = tmpl.Execute(cfgFile, baseParams{
		SocketPath: cfg.StatsSock,
	})
	if err != nil {
		return nil, err
	}

	spoeCfgFile, err := os.Create(cfg.SPOE)
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

func (h *haConfig) FilePath(content []byte) (string, error) {
	sum := sha256.Sum256(content)

	path := path.Join(h.Base, hex.EncodeToString(sum[:]))

	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err == nil {
		return path, nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.Write(content)
	if err != nil {
		return "", err
	}

	return path, nil
}

func (h *haConfig) CertsPath(t consul.TLS) (string, string, error) {
	crt := []byte{}
	crt = append(crt, t.TLSCert...)
	crt = append(crt, t.TLSKey...)

	crtPath, err := h.FilePath(crt)
	if err != nil {
		return "", "", err
	}

	ca := []byte{}
	for _, c := range t.TLSCAs {
		ca = append(ca, c...)
	}

	caPath, err := h.FilePath(ca)
	if err != nil {
		return "", "", err
	}

	return crtPath, caPath, nil
}
