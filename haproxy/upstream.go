package haproxy

import (
	"fmt"
	"math"

	"github.com/criteo/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
	log "github.com/sirupsen/logrus"
)

type upstreamSlot struct {
	consul.UpstreamNode
	Enabled bool
}

func (h *HAProxy) deleteUpstream(tx *tnx, service string) error {
	feName := fmt.Sprintf("front_%s", service)
	beName := fmt.Sprintf("back_%s", service)

	err := tx.DeleteFrontend(feName)
	if err != nil {
		return err
	}
	err = tx.DeleteBackend(beName)
	if err != nil {
		return err
	}

	return nil
}

func (h *HAProxy) createUpstream(tx *tnx, up consul.Upstream) error {
	feName := fmt.Sprintf("front_%s", up.Service)
	beName := fmt.Sprintf("back_%s", up.Service)

	err := tx.CreateFrontend(models.Frontend{
		Name:           feName,
		DefaultBackend: beName,
		ClientTimeout:  &clientTimeout,
		Mode:           models.FrontendModeHTTP,
		Httplog:        true,
	})
	if err != nil {
		return err
	}
	logID := int64(0)
	err = tx.CreateLogTargets("frontend", feName, models.LogTarget{
		ID:       &logID,
		Address:  h.haConfig.LogsSock,
		Facility: models.LogTargetFacilityLocal0,
		Format:   models.LogTargetFormatRfc5424,
	})
	if err != nil {
		return err
	}

	port := int64(up.LocalBindPort)
	err = tx.CreateBind(feName, models.Bind{
		Name:    fmt.Sprintf("%s_bind", feName),
		Address: up.LocalBindAddress,
		Port:    &port,
	})
	if err != nil {
		return err
	}

	err = tx.CreateBackend(models.Backend{
		Name:           beName,
		ServerTimeout:  &serverTimeout,
		ConnectTimeout: &connectTimeout,
		Balance: &models.Balance{
			Algorithm: models.BalanceAlgorithmLeastconn,
		},
		Mode: models.BackendModeHTTP,
	})
	if err != nil {
		return err
	}
	logID = int64(0)
	err = tx.CreateLogTargets("backend", beName, models.LogTarget{
		ID:       &logID,
		Address:  h.haConfig.LogsSock,
		Facility: models.LogTargetFacilityLocal0,
		Format:   models.LogTargetFormatRfc5424,
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *HAProxy) handleUpstream(tx *tnx, up consul.Upstream) error {
	beName := fmt.Sprintf("back_%s", up.Service)

	var current *consul.Upstream
	if h.currentCfg != nil {
		for _, u := range h.currentCfg.Upstreams {
			if u.Service == up.Service {
				current = &u
				break
			}
		}
	}

	backendDeleted := false
	if current != nil && !current.Equal(up) {
		h.deleteUpstream(tx, up.Service)
		backendDeleted = true
	}

	if backendDeleted || current == nil {
		h.createUpstream(tx, up)
	}

	certPath, caPath, err := h.haConfig.CertsPath(up.TLS)
	if err != nil {
		return err
	}

	one := int64(1)
	disabledServer := models.Server{
		Address:        "127.0.0.1",
		Port:           &one,
		Weight:         &one,
		Ssl:            models.ServerSslEnabled,
		SslCertificate: certPath,
		SslCafile:      caPath,
		Maintenance:    models.ServerMaintenanceEnabled,
	}

	serverSlots := h.upstreamServerSlots[up.Service]
	if len(serverSlots) < len(up.Nodes) {
		serverCount := int(math.Pow(2, math.Ceil(math.Log(float64(len(up.Nodes)))/math.Log(2))))
		log.Infof("increasing upstreams %s server pool size to %d", up.Service, serverCount)
		newServerSlots := make([]upstreamSlot, serverCount)
		copy(newServerSlots, serverSlots)

		for i := len(serverSlots); i < len(newServerSlots); i++ {
			srv := disabledServer
			srv.Name = fmt.Sprintf("srv_%d", i)
			err := tx.CreateServer(beName, srv)
			if err != nil {
				return err
			}
		}

		serverSlots = newServerSlots
	}

	for i, slot := range serverSlots {
		if slot.Host == "" {
			continue
		}

		found := false
		for _, n := range up.Nodes {
			if slot.Enabled && n.Equal(slot.UpstreamNode) {
				found = true
				break
			}
		}
		if found {
			continue
		}

		(func(i int) {
			tx.After(func() error {
				srv := disabledServer
				srv.Name = fmt.Sprintf("srv_%d", i)

				return h.dataplaneClient.ReplaceServer(beName, srv)
			})
		})(i)
		serverSlots[i].Enabled = false
	}

Next:
	for _, node := range up.Nodes {
		for _, s := range serverSlots {
			if s.Enabled && node.Equal(s.UpstreamNode) {
				continue Next
			}
		}

		for i, slot := range serverSlots {
			if slot.Host != "" {
				continue
			}

			(func(i int, node consul.UpstreamNode) {
				port := int64(node.Port)
				weight := int64(node.Weight)
				tx.After(func() error {
					return h.dataplaneClient.ReplaceServer(beName, models.Server{
						Name:           fmt.Sprintf("srv_%d", i),
						Address:        node.Host,
						Port:           &port,
						Weight:         &weight,
						Ssl:            models.ServerSslEnabled,
						SslCertificate: certPath,
						SslCafile:      caPath,
						Maintenance:    models.ServerMaintenanceDisabled,
					})
				})
			})(i, node)

			serverSlots[i] = upstreamSlot{
				UpstreamNode: node,
				Enabled:      true,
			}
			break
		}
	}

	tx.After(func() error {
		h.upstreamServerSlots[up.Service] = serverSlots
		return nil
	})

	return nil
}
