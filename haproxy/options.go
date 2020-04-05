package haproxy

type Options struct {
	HAProxyBin           string
	DataplaneBin         string
	ConfigBaseDir        string
	SPOEAddress          string
	EnableIntentions     bool
	EnableModeTcp	     bool
	StatsListenAddr      string
	StatsRegisterService bool
	LogRequests          bool
}
