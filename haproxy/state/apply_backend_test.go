package state

import (
	"testing"

	"github.com/haproxytech/models"
	"github.com/stretchr/testify/require"
)

func TestAddBackend(t *testing.T) {
	old := State{}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t, RequireOp(haOpCreateBackend, "back"))
}

func TestNoChangeBackend(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t)
}

func TestRemoveBackend(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
			},
		},
	}
	new := State{}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t, RequireOp(haOpDeleteBackend, "back"))
}

func TestAddServerDifferentSize(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
			},
		},
	}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpDeleteBackend, "back"),
		RequireOp(haOpCreateBackend, "back"),
		RequireOp(haOpCreateServer, "srv_0"),
	)
}

func TestAddServerSameSize(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:        "srv_1",
						Address:     "127.0.0.1",
						Port:        int64p(1),
						Maintenance: models.ServerMaintenanceEnabled,
					},
				},
			},
		},
	}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:        "srv_1",
						Address:     "1.2.3.5",
						Port:        int64p(8081),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpReplaceServer, "srv_1"),
	)
}

func TestRemoveServerSameSize(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:        "srv_1",
						Address:     "1.2.3.5",
						Port:        int64p(8081),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "127.0.0.1",
						Port:        int64p(1),
						Maintenance: models.ServerMaintenanceEnabled,
					},
					models.Server{
						Name:        "srv_1",
						Address:     "1.2.3.5",
						Port:        int64p(8081),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpReplaceServer, "srv_0"),
	)
}

func TestBackendChange(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name:           "back",
					ConnectTimeout: int64p(1),
				},
				Servers: []models.Server{
					models.Server{
						Name:        "srv_0",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpDeleteBackend, "back"),
		RequireOp(haOpCreateBackend, "back"),
		RequireOp(haOpCreateServer, "srv_0"),
	)
}
