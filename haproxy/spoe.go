package haproxy

import (
	"crypto/x509"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"zvelo.io/ttlru"

	spoe "github.com/criteo/haproxy-spoe-go"
	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

const (
	authzTimeout = time.Second
	cacheTTL     = time.Second
)

var (
	certCacheAccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cert_cache_access",
		Help: "The total number certificate cache access by hit/miss",
	}, []string{"type"})

	authCacheAccess = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_cache_access",
		Help: "The total number auth cache access by hit/miss",
	}, []string{"type"})
)

type cacheEntry struct {
	Value bool
	At    time.Time
	C     chan struct{}
}

type SPOEHandler struct {
	c   *api.Client
	cfg func() consul.Config

	certCache     ttlru.Cache
	authCache     map[string]*cacheEntry
	authCacheLock sync.Mutex
}

func NewSPOEHandler(c *api.Client, cfg func() consul.Config) *SPOEHandler {
	return &SPOEHandler{
		c:         c,
		cfg:       cfg,
		certCache: ttlru.New(2048, ttlru.WithTTL(time.Minute)),
		authCache: map[string]*cacheEntry{},
	}
}

func (h *SPOEHandler) Handler(msgs *spoe.MessageIterator) ([]spoe.Action, error) {
	cfg := h.cfg()
	for msgs.Next() {
		m := msgs.Message

		if m.Name != "check-intentions" {
			continue
		}

		var certBytes []byte
		for m.Args.Next() {
			arg := m.Args.Arg

			switch arg.Name {
			case "cert":
				var ok bool
				certBytes, ok = arg.Value.([]byte)
				if !ok {
					return nil, fmt.Errorf("spoe handler: expected cert bytes in message, got: %+v", m.Args)
				}
			}
		}

		if err := msgs.Error(); err != nil {
			return nil, fmt.Errorf("spoe handler: %s", err)
		}

		if certBytes == nil {
			return nil, fmt.Errorf("spoe handler: cert is required")
		}

		cert, err := h.decodeCertificate(certBytes)
		if err != nil {
			log.Errorf("spoe handler: %s", err)
			return nil, err
		}

		certURI, err := connect.ParseCertURI(cert.URIs[0])
		if err != nil {
			log.Error("connect: invalid leaf certificate URI")
			return nil, errors.New("connect: invalid leaf certificate URI")
		}

		sourceApp := ""
		authorized, err := h.isAuthorized(cfg.ServiceName, certURI.URI().String(), cert.SerialNumber.Bytes())
		if err != nil {
			log.Errorf("spoe handler: %s", err)
			return nil, err
		}

		if sis, ok := certURI.(*connect.SpiffeIDService); ok {
			sourceApp = sis.Service
		}

		res := 1
		if !authorized {
			res = 0
		}
		return []spoe.Action{
			spoe.ActionSetVar{
				Name:  "auth",
				Scope: spoe.VarScopeSession,
				Value: res,
			},
			spoe.ActionSetVar{
				Name:  "source_app",
				Scope: spoe.VarScopeSession,
				Value: sourceApp,
			},
		}, nil
	}
	return nil, nil
}

func (h *SPOEHandler) isAuthorized(target, uri string, serial []byte) (bool, error) {
	h.authCacheLock.Lock()
	entry, ok := h.authCache[uri]
	now := time.Now()
	if !ok || now.Sub(entry.At) > cacheTTL {
		authCacheAccess.WithLabelValues("miss").Inc()
		entry = &cacheEntry{
			At: now,
			C:  make(chan struct{}),
		}
		h.authCache[uri] = entry
		h.authCacheLock.Unlock()

		go func() {
			auth, err := h.fetchAutz(target, uri, serial)

			h.authCacheLock.Lock()
			defer h.authCacheLock.Unlock()

			if err != nil {
				log.Error(err)
				entry.Value = false
				// force refech on next request
				entry.At = time.Time{}
			} else {
				entry.Value = auth
			}

			// notify waiting requets
			close(entry.C)
		}()
	} else {
		authCacheAccess.WithLabelValues("hit").Inc()
		h.authCacheLock.Unlock()
	}

	select {
	case <-time.After(authzTimeout):
		return false, fmt.Errorf("authz call failed: timeout after %s", authzTimeout)
	case <-entry.C:
		return entry.Value, nil
	}
}

func (h *SPOEHandler) fetchAutz(target, uri string, serial []byte) (bool, error) {
	resp, err := h.c.Agent().ConnectAuthorize(&api.AgentAuthorizeParams{
		Target:           target,
		ClientCertURI:    uri,
		ClientCertSerial: connect.HexString(serial),
	})
	if err != nil {
		return false, errors.Wrap(err, "authz call failed")
	}

	return resp.Authorized, nil
}

func (h *SPOEHandler) decodeCertificate(b []byte) (*x509.Certificate, error) {
	certCacheKey := string(b)
	if v, ok := h.certCache.Get(certCacheKey); ok {
		certCacheAccess.WithLabelValues("hit").Inc()
		return v.(*x509.Certificate), nil
	}

	certCacheAccess.WithLabelValues("miss").Inc()
	cert, err := x509.ParseCertificate(b)
	if err != nil {
		return nil, err
	}
	h.certCache.Set(certCacheKey, cert)

	return cert, nil
}
