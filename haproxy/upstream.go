package haproxy

import (
	"fmt"

	"github.com/aestek/haproxy-connect/consul"
	"github.com/haproxytech/models"
)

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

func (h *HAProxy) handleUpstream(tx *tnx, up consul.Upstream) error {
	feName := fmt.Sprintf("front_%s", up.Service)
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
		err := tx.DeleteFrontend(feName)
		if err != nil {
			return err
		}
		err = tx.DeleteBackend(beName)
		if err != nil {
			return err
		}
		backendDeleted = true
	}

	if backendDeleted || current == nil {
		timeout := int64(1000)
		err := tx.CreateFrontend(models.Frontend{
			Name:           feName,
			DefaultBackend: beName,
			ClientTimeout:  &timeout,
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
			ServerTimeout:  &timeout,
			ConnectTimeout: &timeout,
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
	}

	currentServers := map[string]consul.UpstreamNode{}
	if !backendDeleted && current != nil {
		for _, b := range current.Nodes {
			currentServers[fmt.Sprintf("%s:%d", b.Host, b.Port)] = b
		}
	}

	certPath, caPath, err := h.haConfig.CertsPath(up.TLS)
	if err != nil {
		return err
	}

	newServers := map[string]consul.UpstreamNode{}
	for _, srv := range up.Nodes {
		id := fmt.Sprintf("%s:%d", srv.Host, srv.Port)
		newServers[id] = srv

		currentSrv, currentExists := currentServers[id]

		changed := currentExists && currentSrv.Weight != srv.Weight
		if !changed && currentExists {
			continue
		}

		f := tx.CreateServer
		if changed {
			f = tx.ReplaceServer
		}

		fmt.Println(id, srv.Port)
		port := int64(srv.Port)
		weight := int64(srv.Weight)
		serverDef := models.Server{
			Name:           id,
			Address:        srv.Host,
			Port:           &port,
			Weight:         &weight,
			Ssl:            models.ServerSslEnabled,
			SslCertificate: certPath,
			SslCafile:      caPath,
		}

		err := f(beName, serverDef)
		if err != nil {
			return err
		}
	}

	for current := range currentServers {
		_, ok := newServers[current]
		if !ok {
			err := tx.DeleteServer(beName, current)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
