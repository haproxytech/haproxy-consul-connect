package haproxy

import (
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/haproxytech/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

var (
	opsProcessed = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_requests_total",
		Help: "The total number of http requests",
	}, []string{"service"})
)

type Stats struct {
	dpapi *dataplaneClient
}

func (s *Stats) Run() {
	for {
		time.Sleep(time.Second)
		stats, err := s.dpapi.Stats()
		if err != nil {
			log.Error(err)
			continue
		}
		s.handle(stats)
	}
}

func (s *Stats) handle(stats []models.NativeStat) {
	for _, s := range stats {
		spew.Dump(s)
	}
}
