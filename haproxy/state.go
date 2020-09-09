package haproxy

import (
	"time"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/state"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	log "github.com/sirupsen/logrus"
	"gopkg.in/d4l3k/messagediff.v1"
)

const (
	stateApplyThrottle   = 500 * time.Millisecond
	resyncConfigInterval = 5 * time.Minute
	retryBackoff         = 3 * time.Second
)

func (h *HAProxy) watch(sd *lib.Shutdown) error {
	throttle := time.Tick(stateApplyThrottle)
	resyncConfig := time.Tick(resyncConfigInterval)
	retry := make(chan struct{})

	var currentState state.State
	var currentConfig consul.Config
	dirty := false
	started := false
	ready := false

	waitAndRetry := func() {
		time.Sleep(retryBackoff)
		select {
		case retry <- struct{}{}:
		default:
		}
	}

	for {
		inputReceived := false
	Throttle:
		for {
			select {
			case <-sd.Stop:
				return nil

			case <-throttle:
				if inputReceived {
					break Throttle
				}

			case c := <-h.cfgC:
				log.Info("handling new configuration")
				h.currentConsulConfig = &c
				currentConfig = c
				inputReceived = true
			case <-resyncConfig:
				log.Info("periodic haproxy config sync check")
				dirty = true
				inputReceived = true
			case <-retry:
				log.Warn("retrying to apply config")
				dirty = true
				inputReceived = true
			}
		}

		if !started {
			err := h.start(sd)
			if err != nil {
				return err
			}
			started = true
		}

		if dirty {
			fromHa, err := state.FromHAProxy(h.dataplaneClient)
			if err != nil {
				log.Errorf("error retrieving haproxy conf: %s", err)
				waitAndRetry()
				continue
			}
			diff, equal := messagediff.PrettyDiff(currentState, fromHa)
			if !equal {
				log.Errorf("diff found between expected state and haproxy state: %s", diff)
			}
			currentState = fromHa
			dirty = false
		}

		newState, err := state.Generate(state.Options{
			EnableIntentions: h.opts.EnableIntentions,
			LogRequests:      h.opts.LogRequests,
			LogSocket:        h.haConfig.LogsSock,
			SPOEConfigPath:   h.haConfig.SPOE,
			SPOESocket:       h.haConfig.SPOESock,
		}, h.haConfig, currentState, currentConfig)
		if err != nil {
			log.Error(err)
			continue
		}

		if currentState.Equal(newState) {
			log.Info("no change to apply to haproxy")
			continue
		}

		tx := h.dataplaneClient.Tnx()

		log.Debugf("applying new state: %+v", newState)

		err = state.Apply(tx, currentState, newState)
		if err != nil {
			log.Error(err)
			waitAndRetry()
			continue
		}

		err = tx.Commit()
		if err != nil {
			log.Error(err)
			waitAndRetry()
			continue
		}

		if !ready {
			close(h.Ready)
			ready = true
		}

		currentState = newState
		log.Info("state applied")
	}
}
