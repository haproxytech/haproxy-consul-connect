package lib

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// Shutdown builds a shutdown that might be used to monitor process
// and catch signals for proper terminaison
type Shutdown struct {
	sync.WaitGroup
	Stop    chan struct{}
	stopped uint32
}

// NewShutdown build a new Shutdown struct
func NewShutdown() *Shutdown {
	sd := &Shutdown{
		Stop: make(chan struct{}),
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		i := 0
		for sig := range sigs {
			log.Infof("Received %s", sig)
			sd.Shutdown(sig.String())

			if i > 0 {
				os.Exit(1)
			}
			i++
		}
	}()

	return sd
}

// Shutdown Ask all processes to shutdown
func (h *Shutdown) Shutdown(reason string) {
	if atomic.SwapUint32(&h.stopped, 1) > 0 {
		return
	}
	log.Infof("Shutting down because %s...", reason)
	close(h.Stop)
}
