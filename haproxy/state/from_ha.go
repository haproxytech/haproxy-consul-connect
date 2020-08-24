package state

import (
	"fmt"
	"sort"

	"github.com/haproxytech/models"
)

type HAProxyRead interface {
	Frontends() ([]models.Frontend, error)
	Binds(feName string) ([]models.Bind, error)
	LogTargets(parentType, parentName string) ([]models.LogTarget, error)
	Filters(parentType, parentName string) ([]models.Filter, error)
	TCPRequestRules(parentType, parentName string) ([]models.TCPRequestRule, error)
	HTTPRequestRules(parentType, parentName string) ([]models.HTTPRequestRule, error)
	Backends() ([]models.Backend, error)
	Servers(beName string) ([]models.Server, error)
}

func FromHAProxy(ha HAProxyRead) (State, error) {
	state := State{}

	haFrontends, err := ha.Frontends()
	if err != nil {
		return state, err
	}

	for _, f := range haFrontends {
		binds, err := ha.Binds(f.Name)
		if err != nil {
			return state, err
		}
		if len(binds) != 1 {
			return state, fmt.Errorf("expected 1 bind for frontend %s, got %d", f.Name, len(binds))
		}
		logTargets, err := ha.LogTargets("frontend", f.Name)
		if err != nil {
			return state, err
		}
		if len(logTargets) > 1 {
			return state, fmt.Errorf("expected at most 1 log target for frontend %s, got %d", f.Name, len(logTargets))
		}

		var lt *models.LogTarget
		if len(logTargets) == 1 {
			lt = &logTargets[0]
		}

		filters, err := ha.Filters("frontend", f.Name)
		if err != nil {
			return state, err
		}
		if len(logTargets) > 1 {
			return state, fmt.Errorf("expected at most 1 filter for frontend %s, got %d", f.Name, len(filters))
		}
		var filter *FrontendFilter
		if len(filters) == 1 {
			filter = &FrontendFilter{
				Filter: filters[0],
			}

			rules, err := ha.TCPRequestRules("frontend", f.Name)
			if err != nil {
				return state, err
			}
			if len(binds) != 1 {
				return state, fmt.Errorf("expected 1 tcp request rule for frontend %s, got %d", f.Name, len(rules))
			}
			filter.Rule = rules[0]
		}

		state.Frontends = append(state.Frontends, Frontend{
			Frontend:  f,
			Bind:      binds[0],
			LogTarget: lt,
			Filter:    filter,
		})
	}

	sort.Sort(Frontends(state.Frontends))

	haBackends, err := ha.Backends()
	if err != nil {
		return state, err
	}

	for _, b := range haBackends {
		servers, err := ha.Servers(b.Name)
		if err != nil {
			return state, err
		}

		logTargets, err := ha.LogTargets("backend", b.Name)
		if err != nil {
			return state, err
		}
		if len(logTargets) > 1 {
			return state, fmt.Errorf("expected at most 1 log target for backend %s, got %d", b.Name, len(logTargets))
		}

		var lt *models.LogTarget
		if len(logTargets) == 1 {
			lt = &logTargets[0]
		}

		reqRules, err := ha.HTTPRequestRules("backend", b.Name)
		if err != nil {
			return state, err
		}
		if len(reqRules) == 0 {
			reqRules = nil
		}

		state.Backends = append(state.Backends, Backend{
			Backend:          b,
			Servers:          servers,
			LogTarget:        lt,
			HTTPRequestRules: reqRules,
		})
	}

	sort.Sort(Backends(state.Backends))

	return state, nil
}
