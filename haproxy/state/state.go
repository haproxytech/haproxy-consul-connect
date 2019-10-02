package state

import "github.com/haproxytech/models"

type FrontendFilter struct {
	Filter models.Filter
	Rule   models.TCPRequestRule
}

type Frontend struct {
	Frontend  models.Frontend
	Bind      models.Bind
	LogTarget *models.LogTarget
	Filter    *FrontendFilter
}

type Backend struct {
	Backend   models.Backend
	LogTarget *models.LogTarget
	Servers   []models.Server
}

type State struct {
	Frontends []Frontend
	Backends  []Backend
}

func (s State) findBackend(name string) (Backend, bool) {
	for _, b := range s.Backends {
		if b.Backend.Name == name {
			return b, true
		}
	}

	return Backend{}, false
}
