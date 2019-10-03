package haproxy

import (
	"github.com/criteo/haproxy-consul-connect/consul"
	"github.com/criteo/haproxy-consul-connect/haproxy/state"
	log "github.com/sirupsen/logrus"
)

func (h *HAProxy) handleChange(cfg consul.Config) error {
	log.Info("handling new configuration")

	newState, err := state.Generate(state.Options{
		EnableIntentions: h.opts.EnableIntentions,
		LogRequests:      h.opts.LogRequests,
		LogSocket:        h.haConfig.LogsSock,
		SPOEConfigPath:   h.haConfig.SPOE,
		SPOESocket:       h.haConfig.SPOESock,
	}, h.haConfig, h.oldState, cfg)
	if err != nil {
		return err
	}

	tx := h.dataplaneClient.Tnx()

	log.Debugf("applying new state: %+v", newState)

	err = state.Apply(tx, h.oldState, newState)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	h.oldState = newState
	h.currentCfg = &cfg

	log.Info("state successfully applied")

	return nil
}
