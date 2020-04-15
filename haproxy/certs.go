package haproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	log "github.com/sirupsen/logrus"
)

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

	log.Debugf("wrote new config file %s", path)

	return path, nil
}

func (h *haConfig) CertsPath(t consul.TLS) (string, string, error) {
	crt := []byte{}
	crt = append(crt, t.Cert...)
	crt = append(crt, t.Key...)

	crtPath, err := h.FilePath(crt)
	if err != nil {
		return "", "", err
	}

	ca := []byte{}
	for _, c := range t.CAs {
		ca = append(ca, c...)
	}

	caPath, err := h.FilePath(ca)
	if err != nil {
		return "", "", err
	}

	return caPath, crtPath, nil
}
