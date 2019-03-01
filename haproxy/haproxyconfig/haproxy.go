package haproxyconfig

import (
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/aestek/haproxy-connect/haproxy"
	"github.com/aestek/haproxy-connect/lib"
	log "github.com/sirupsen/logrus"
)

const (
	configFileName = "haproxy.conf"
)

type Options struct {
	Bin           string
	ConfigBaseDir string
	SPOEAddress   string
}

type Haproxy struct {
	cfg        chan haproxy.Configuration
	tmpl       *template.Template
	cfgBaseDir string
	opts       Options
	certPaths  map[string]string
}

func New(cfg chan haproxy.Configuration, opts Options) *Haproxy {
	tmpl, err := template.New("cfg").Parse(haproxyConfTmpl)
	if err != nil {
		panic(err)
	}

	return &Haproxy{
		cfg:       cfg,
		tmpl:      tmpl,
		opts:      opts,
		certPaths: make(map[string]string),
	}
}

func (h *Haproxy) Run(sd *lib.Shutdown) error {
	cfgDir, err := ioutil.TempDir(h.opts.ConfigBaseDir, "haproxy-connect-")
	if err != nil {
		return err
	}

	sd.Add(1)
	go func() {
		<-sd.Stop
		log.Infof("cleaning up confg directory (%s)", cfgDir)
		if err := os.RemoveAll(cfgDir); err != nil {
			log.Errorf("error cleaning up confg: %s", err)
		}
		sd.Done()
	}()

	var firstConfig haproxy.Configuration
	select {
	case firstConfig = <-h.cfg:
	case <-sd.Stop:
		return nil
	}

	h.cfgBaseDir = cfgDir

	cfgPath, err := h.genCfg(firstConfig)
	if err != nil {
		return err
	}

	cmd := newCommand(h.opts.Bin, cfgPath, path.Join(h.cfgBaseDir, "haproxy.sock"), sd)
	if err := cmd.Start(); err != nil {
		return err
	}

	for {
		select {
		case <-cmd.Stopped:
			sd.Shutdown()
			return nil
		case <-sd.Stop:
			return cmd.Stop()
		case newCfg := <-h.cfg:
			cfgPath, err := h.genCfg(newCfg)
			if err != nil {
				log.Errorf("haproxy config gen: %s", err)
			} else {
				if err := cmd.Reload(cfgPath); err != nil {
					log.Errorf("haproxy reload: %s", err)
				} else {
					log.Info("haproxy: reload successful")
				}
			}
		}
	}
}

func (h *Haproxy) genCfg(cfg haproxy.Configuration) (string, error) {
	err := os.MkdirAll(h.cfgBaseDir, 0755)
	if err != nil {
		return "", err
	}

	cfg.SocketPath = path.Join(h.cfgBaseDir, "haproxy.sock")
	cfg.SPOEConfPath = path.Join(h.cfgBaseDir, "spoe.conf")

	err = ioutil.WriteFile(cfg.SPOEConfPath, []byte(spoeConfTmpl), 0644)
	if err != nil {
		return "", err
	}

	cfgFile, err := ioutil.TempFile(h.cfgBaseDir, "conf-")
	if err != nil {
		return "", err
	}

	for i, f := range cfg.Frontends {
		if len(f.ClientCA) > 0 {
			cas := ""
			for _, ca := range f.ClientCA {
				cas += string(ca)
			}
			path, err := h.getCertPath(cas)
			if err != nil {
				return "", err
			}
			cfg.Frontends[i].ClientCAPath = path
		}

		if f.ServerCRT != "" {
			path, err := h.getCertPath(string(f.ServerCRT) + string(f.ServerKey))
			if err != nil {
				return "", err
			}
			cfg.Frontends[i].ServerCRTPath = path
		}
	}

	for i, f := range cfg.Backends {
		for j, s := range f.Servers {
			if len(s.ServerCA) > 0 {
				cas := ""
				for _, ca := range s.ServerCA {
					cas += string(ca)
				}
				path, err := h.getCertPath(cas)
				if err != nil {
					return "", err
				}
				cfg.Backends[i].Servers[j].ServerCAPath = path
			}

			if s.ClientCRT != "" {
				path, err := h.getCertPath(string(s.ClientCRT) + string(s.ClientKey))
				if err != nil {
					return "", err
				}
				cfg.Backends[i].Servers[j].ClientCRTPath = path
			}
		}
	}

	defer func() {
		err := cfgFile.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	log.Infof("config updated: %+v", cfg)

	return cfgFile.Name(), h.tmpl.Execute(cfgFile, cfg)
}

func (h *Haproxy) getCertPath(contents string) (string, error) {
	if path, ok := h.certPaths[contents]; ok {
		return path, nil
	}
	file, err := ioutil.TempFile(h.cfgBaseDir, "cert-")
	if err != nil {
		return "", err
	}
	_, err = file.Write([]byte(contents))
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	h.certPaths[contents] = file.Name()
	return file.Name(), nil
}
