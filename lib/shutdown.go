package lib

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	log "github.com/sirupsen/logrus"
)

type Shutdown struct {
	sync.WaitGroup
	Stop    chan struct{}
	stopped uint32
}

func NewShutdown() *Shutdown {
	sd := &Shutdown{
		Stop: make(chan struct{}),
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Infof("received %s, shutting down...", sig)
		sd.Shutdown()
	}()

	return sd
}

func (h *Shutdown) Shutdown() {
	if atomic.SwapUint32(&h.stopped, 1) > 0 {
		return
	}
	close(h.Stop)
}
