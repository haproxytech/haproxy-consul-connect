package haproxy

import (
	"fmt"

	"github.com/criteo/haproxy-consul-connect/consul"
	"github.com/haproxytech/models"
)

func (h *HAProxy) handleDownstream(tx *tnx, ds consul.Downstream) error {
	if h.currentCfg != nil && h.currentCfg.Downstream.Equal(ds) {
		return nil
	}

	feName := "front_downstream"
	beName := "back_downstream"

	if h.currentCfg != nil {
		err := tx.DeleteFrontend(feName)
		if err != nil {
			return err
		}
		err = tx.DeleteBackend(beName)
		if err != nil {
			return err
		}
	}

	err := tx.CreateFrontend(models.Frontend{
		Name:           feName,
		DefaultBackend: beName,
		ClientTimeout:  &clientTimeout,
		Mode:           models.FrontendModeHTTP,
		Httplog:        h.opts.LogRequests,
	})
	if err != nil {
		return err
	}

	crtPath, caPath, err := h.haConfig.CertsPath(ds.TLS)
	if err != nil {
		return err
	}

	port := int64(ds.LocalBindPort)
	err = tx.CreateBind(feName, models.Bind{
		Name:           fmt.Sprintf("%s_bind", feName),
		Address:        ds.LocalBindAddress,
		Port:           &port,
		Ssl:            true,
		SslCertificate: crtPath,
		SslCafile:      caPath,
		Verify:         models.BindVerifyRequired,
	})
	if err != nil {
		return err
	}

	if h.opts.LogRequests {
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
	}

	if h.opts.EnableIntentions {
		filterID := int64(0)
		err = tx.CreateFilter("frontend", feName, models.Filter{
			Type:       models.FilterTypeSpoe,
			ID:         &filterID,
			SpoeEngine: "intentions",
			SpoeConfig: h.haConfig.SPOE,
		})
		if err != nil {
			return err
		}

		err = tx.CreateTCPRequestRule("frontend", feName, models.TCPRequestRule{
			Action:   models.TCPRequestRuleActionReject,
			Cond:     models.TCPRequestRuleCondUnless,
			CondTest: "{ var(sess.connect.auth) -m int eq 1 }",
			Type:     models.TCPRequestRuleTypeContent,
			ID:       &filterID,
		})
		if err != nil {
			return err
		}
	}

	err = tx.CreateBackend(models.Backend{
		Name:           beName,
		ServerTimeout:  &serverTimeout,
		ConnectTimeout: &connectTimeout,
		Mode:           models.BackendModeHTTP,
	})
	if err != nil {
		return err
	}

	if h.opts.LogRequests {
		logID := int64(0)
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

	bePort := int64(ds.TargetPort)
	err = tx.CreateServer(beName, models.Server{
		Name:    "downstream_node",
		Address: ds.TargetAddress,
		Port:    &bePort,
	})
	if err != nil {
		return err
	}

	return nil
}
