package haproxy

import (
	"strings"
	"time"

	"github.com/haproxytech/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

var (
	upMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_up",
		Help: "The total number of http requests",
	}, []string{"service"})

	reqOutRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_http_request_out_rate",
		Help: "The total number of http requests",
	}, []string{"service", "target"})
	reqInRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_http_request_in_rate",
		Help: "The total number of http requests",
	}, []string{"service"})
	resInTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_http_response_in_total",
		Help: "The total number of http requests",
	}, []string{"service", "code"})
	resOutTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_http_response_out_total",
		Help: "The total number of http requests",
	}, []string{"service", "target", "code"})

	resTimeIn = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_http_response_in_avg_time_second",
		Help: "The total number of http requests",
	}, []string{"service"})
	resTimeOut = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_http_response_out_avg_time_second",
		Help: "The total number of http requests",
	}, []string{"service", "target"})

	connOutCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_connection_out_rate",
		Help: "The total number of http requests",
	}, []string{"service", "target"})
	connInCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_connection_in_count",
		Help: "The total number of http requests",
	}, []string{"service"})

	bytesInOut = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_bytes_in_out_total",
		Help: "The total number of http requests",
	}, []string{"service", "target"})
	bytesOutOut = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_bytes_out_out_total",
		Help: "The total number of http requests",
	}, []string{"service", "target"})
	bytesInIn = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_bytes_in_in_total",
		Help: "The total number of http requests",
	}, []string{"service"})
	bytesOutIn = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "haproxy_connect_bytes_out_in_total",
		Help: "The total number of http requests",
	}, []string{"service"})
)

type Stats struct {
	service string
	dpapi   *dataplaneClient
}

func (s *Stats) Run() {
	upMetric.WithLabelValues(s.service).Set(1)
	for {
		time.Sleep(time.Second)
		stats, err := s.dpapi.Stats()
		if err != nil {
			log.Error(err)
			continue
		}
		for _, stat := range stats {
			s.handle(stat)
		}
	}
}

func (s *Stats) handle(stats *models.NativeStatsCollection) {
	for _, stats := range stats.Stats {
		switch stats.Type {
		case models.NativeStatTypeFrontend:
			s.handleFrontend(stats)
		case models.NativeStatTypeBackend:
			s.handlebackend(stats)
		case models.NativeStatTypeServer:
			s.handleServer(stats)
		}
	}
}

func statVal(i *int64) float64 {
	if i == nil {
		return 0
	}
	return float64(*i)
}

func (s *Stats) handleFrontend(stats *models.NativeStat) {
	targetService := strings.TrimPrefix(stats.Name, "front_")

	if targetService == "downstream" {
		reqInRate.WithLabelValues(s.service).Set(statVal(stats.Stats.ReqRate))
		connInCount.WithLabelValues(s.service).Set(statVal(stats.Stats.Scur))
		bytesInIn.WithLabelValues(s.service).Set(statVal(stats.Stats.Bin))
		bytesOutIn.WithLabelValues(s.service).Set(statVal(stats.Stats.Bout))

		resInTotal.WithLabelValues(s.service, "1xx").Set(statVal(stats.Stats.Hrsp1xx))
		resInTotal.WithLabelValues(s.service, "2xx").Set(statVal(stats.Stats.Hrsp2xx))
		resInTotal.WithLabelValues(s.service, "3xx").Set(statVal(stats.Stats.Hrsp3xx))
		resInTotal.WithLabelValues(s.service, "4xx").Set(statVal(stats.Stats.Hrsp4xx))
		resInTotal.WithLabelValues(s.service, "5xx").Set(statVal(stats.Stats.Hrsp5xx))
		resInTotal.WithLabelValues(s.service, "other").Set(statVal(stats.Stats.HrspOther))
	} else {
		reqOutRate.WithLabelValues(s.service, targetService).Set(statVal(stats.Stats.ReqRate))
		connOutCount.WithLabelValues(s.service, targetService).Set(statVal(stats.Stats.Scur))
		bytesInOut.WithLabelValues(s.service, targetService).Set(statVal(stats.Stats.Bin))
		bytesOutOut.WithLabelValues(s.service, targetService).Set(statVal(stats.Stats.Bout))

		resOutTotal.WithLabelValues(s.service, targetService, "1xx").Set(statVal(stats.Stats.Hrsp1xx))
		resOutTotal.WithLabelValues(s.service, targetService, "2xx").Set(statVal(stats.Stats.Hrsp2xx))
		resOutTotal.WithLabelValues(s.service, targetService, "3xx").Set(statVal(stats.Stats.Hrsp3xx))
		resOutTotal.WithLabelValues(s.service, targetService, "4xx").Set(statVal(stats.Stats.Hrsp4xx))
		resOutTotal.WithLabelValues(s.service, targetService, "5xx").Set(statVal(stats.Stats.Hrsp5xx))
		resOutTotal.WithLabelValues(s.service, targetService, "other").Set(statVal(stats.Stats.HrspOther))
	}
}

func (s *Stats) handlebackend(stats *models.NativeStat) {
	targetService := strings.TrimPrefix(stats.Name, "back_")

	if targetService == "downstream" {
		resTimeIn.WithLabelValues(s.service).Set(statVal(stats.Stats.Ttime) / 1000)
	} else {
		resTimeOut.WithLabelValues(s.service, targetService).Set(statVal(stats.Stats.Ttime) / 1000)
	}
}

func (s *Stats) handleServer(stats *models.NativeStat) {
	resTimeOut.WithLabelValues(s.service, stats.Name).Set(statVal(stats.Stats.Ttime) / 1000)
}
