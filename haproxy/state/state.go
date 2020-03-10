package state

import (
	"reflect"

	"github.com/haproxytech/models/v2"
)

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
	Backend          models.Backend
	LogTarget        *models.LogTarget
	Servers          []models.Server
	HTTPRequestRules []models.HTTPRequestRule
}

type State struct {
	Frontends []Frontend
	Backends  []Backend
}

func (s State) Equal(o State) bool {
	return reflect.DeepEqual(s, o)
}

func (s State) findBackend(name string) (Backend, bool) {
	for _, b := range s.Backends {
		if b.Backend.Name == name {
			return b, true
		}
	}

	return Backend{}, false
}

// Backends implements methods to sort, will sort by Name
type Backends []Backend

func (a Backends) Len() int           { return len(a) }
func (a Backends) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Backends) Less(i, j int) bool { return a[i].Backend.Name < a[j].Backend.Name }

// Frontends implement methods to sort, will sort by Name
type Frontends []Frontend

func (a Frontends) Len() int           { return len(a) }
func (a Frontends) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Frontends) Less(i, j int) bool { return a[i].Frontend.Name < a[j].Frontend.Name }
