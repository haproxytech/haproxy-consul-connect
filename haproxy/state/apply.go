package state

import (
	"reflect"

	"github.com/haproxytech/models"
)

func Apply(ha HAProxy, old, new State) error {
	err := applyFrontends(ha, old.Frontends, new.Frontends)
	if err != nil {
		return err
	}

	err = applyBackends(ha, old.Backends, new.Backends)
	if err != nil {
		return err
	}

	return nil
}

func applyFrontends(ha HAProxy, old, new []Frontend) error {
	oldIdx := index(old, func(i int) string {
		return old[i].Frontend.Name
	})
	newIdx := index(new, func(i int) string {
		return new[i].Frontend.Name
	})

	for _, oldUp := range old {
		_, ok := newIdx[oldUp.Frontend.Name]
		if ok {
			continue
		}

		err := ha.DeleteFrontend(oldUp.Frontend.Name)
		if err != nil {
			return err
		}
	}

	for _, newUp := range new {
		oldi, exists := oldIdx[newUp.Frontend.Name]
		if exists {
			if shouldRecreateFrontend(old[oldi], newUp) {
				err := ha.DeleteFrontend(newUp.Frontend.Name)
				if err != nil {
					return err
				}
			} else {
				continue
			}
		}

		err := ha.CreateFrontend(newUp.Frontend)
		if err != nil {
			return err
		}

		err = ha.CreateBind(newUp.Frontend.Name, newUp.Bind)
		if err != nil {
			return err
		}

		if newUp.LogTarget != nil {
			err = ha.CreateLogTargets("frontend", newUp.Frontend.Name, *newUp.LogTarget)
			if err != nil {
				return err
			}
		}

		if newUp.Filter != nil {
			err = ha.CreateFilter("frontend", newUp.Frontend.Name, newUp.Filter.Filter)
			if err != nil {
				return err
			}

			err = ha.CreateTCPRequestRule("frontend", newUp.Frontend.Name, newUp.Filter.Rule)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func shouldRecreateFrontend(old, new Frontend) bool {
	return !reflect.DeepEqual(old, new)
}

func applyBackends(ha HAProxy, old, new []Backend) error {
	oldIdx := index(old, func(i int) string {
		return old[i].Backend.Name
	})
	newIdx := index(new, func(i int) string {
		return new[i].Backend.Name
	})

	for _, oldUp := range old {
		_, ok := newIdx[oldUp.Backend.Name]
		if ok {
			continue
		}

		err := ha.DeleteBackend(oldUp.Backend.Name)
		if err != nil {
			return err
		}
	}

	for _, newBack := range new {
		var oldServers []models.Server

		needCreate := true
		oldi, exists := oldIdx[newBack.Backend.Name]
		if exists {
			if shouldRecreateBackend(old[oldi], newBack) {
				err := ha.DeleteBackend(newBack.Backend.Name)
				if err != nil {
					return err
				}
			} else {
				oldServers = old[oldi].Servers
				needCreate = false
			}
		}

		if needCreate {
			err := ha.CreateBackend(newBack.Backend)
			if err != nil {
				return err
			}

			if newBack.LogTarget != nil {
				err = ha.CreateLogTargets("backend", newBack.Backend.Name, *newBack.LogTarget)
				if err != nil {
					return err
				}
			}

			for _, r := range newBack.HTTPRequestRules {
				err = ha.CreateHTTPRequestRule("backend", newBack.Backend.Name, r)
			}
		}

		if !needCreate {
			for i, s := range newBack.Servers {
				if !shouldUpdateServer(oldServers[i], s) {
					continue
				}

				err := ha.ReplaceServer(newBack.Backend.Name, s)
				if err != nil {
					return err
				}
			}
		} else {
			for _, s := range newBack.Servers {
				err := ha.CreateServer(newBack.Backend.Name, s)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func shouldRecreateBackend(old, new Backend) bool {
	return !reflect.DeepEqual(old.Backend, new.Backend) ||
		!reflect.DeepEqual(old.LogTarget, new.LogTarget) ||
		len(old.Servers) != len(new.Servers)
}

func shouldUpdateServer(old, new models.Server) bool {
	return !reflect.DeepEqual(old, new)
}
