package haproxy

type HAProxyParams struct {
	Defaults map[string]string
	Globals  map[string]string
}

func (p HAProxyParams) With(other HAProxyParams) HAProxyParams {
	new := HAProxyParams{
		Defaults: map[string]string{},
		Globals:  map[string]string{},
	}
	for k, v := range p.Defaults {
		new.Defaults[k] = v
	}
	for k, v := range other.Defaults {
		new.Defaults[k] = v
	}
	for k, v := range p.Globals {
		new.Globals[k] = v
	}
	for k, v := range other.Globals {
		new.Globals[k] = v
	}
	return new
}

type Options struct {
	HAProxyBin           string
	DataplaneBin         string
	ConfigBaseDir        string
	SPOEAddress          string
	EnableIntentions     bool
	StatsListenAddr      string
	StatsRegisterService bool
	LogRequests          bool
	HAProxyParams        HAProxyParams
}
