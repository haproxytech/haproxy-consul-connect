package haproxy

import (
	"sync/atomic"
	"time"

	"github.com/haproxytech/haproxy-consul-connect/consul"
	"github.com/haproxytech/haproxy-consul-connect/haproxy/state"
	"github.com/haproxytech/haproxy-consul-connect/lib"
	log "github.com/sirupsen/logrus"
	"gopkg.in/d4l3k/messagediff.v1"
)

func (h *HAProxy) watch(sd *lib.Shutdown) error {
	throttle := time.Tick(50 * time.Millisecond)
	currentState := state.State{}
	nextState := &atomic.Value{}
	next := make(chan struct{}, 1)
	dirty := false

	go func() {
		for c := range h.cfgC {
			select {
			case <-sd.Stop:
				return
			default:
			}

			log.Info("received consul config update")
			nextState.Store(c)
			h.currentConsulConfig = &c
			select {
			case next <- struct{}{}:
			default:
			}
		}
	}()

	go func() {
		for range time.Tick(5 * time.Minute) {
			select {
			case <-sd.Stop:
				return
			default:
			}

			dirty = true
		}
	}()

	retry := func() {
		time.Sleep(3 * time.Second)
		select {
		case next <- struct{}{}:
		default:
		}
	}

	started := false
	for {
		for {
			select {
			case <-sd.Stop:
				return nil
			case <-next:
			}

			<-throttle

			log.Info("handling new configuration")
			if !started {
				err := h.start(sd)
				if err != nil {
					return err
				}
				started = true
				close(h.Ready)
			}

			if dirty {
				log.Info("refreshing haproxy state")
				fromHa, err := state.FromHAProxy(h.dataplaneClient)
				if err != nil {
					log.Errorf("error retrieving haproxy conf: %s", err)
					retry()
					continue
				}
				diff, equal := messagediff.PrettyDiff(currentState, fromHa)
				if !equal {
					log.Errorf("diff found between expected state and haproxy state: %s", diff)
				}
				currentState = fromHa
				dirty = false
			}

			newConsulCfg := nextState.Load().(consul.Config)

			logAddress := h.haConfig.LogsSock
			if h.opts.HAProxyLogAddress != "" {
				logAddress = h.opts.HAProxyLogAddress
			}

			newState, err := state.Generate(state.Options{
				EnableIntentions: h.opts.EnableIntentions,
				LogRequests:      h.opts.HAProxyLogRequests,
				LogAddress:       logAddress,
				SPOEConfigPath:   h.haConfig.SPOE,
				SPOESocket:       h.haConfig.SPOESock,
			}, h.haConfig, currentState, newConsulCfg)
			if err != nil {
				log.Error(err)
				retry()
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
				retry()
				continue
			}

			err = tx.Commit()
			if err != nil {
				log.Error(err)
				retry()
				continue
			}

			currentState = newState

			log.Info("state applied")
		}
	}
}
