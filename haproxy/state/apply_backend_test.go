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

func TestAddBackendHTTPRule(t *testing.T) {
	old := State{}
	new := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				HTTPRequestRules: []models.HTTPRequestRule{
					{
						HdrName:   "X-App",
						HdrFormat: "%[var(sess.connect.source_app)]",
					},
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpCreateBackend, "back"),
		RequireOp(haOpCreateHTTPRequestRule, "back"),
	)
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
						Name:        "some-server",
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
						Name:        "some-server",
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
						Name:        "some-server",
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
		RequireOp(haOpCreateServer, "some-server"),
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
						Name:        "some-server",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:        "disabled_server_0",
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
						Name:        "some-server",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:        "different-server",
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
		RequireOp(haOpReplaceServer, "disabled_server_0"),
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
						Name:        "some-server",
						Address:     "1.2.3.4",
						Port:        int64p(8080),
						Maintenance: models.ServerMaintenanceDisabled,
					},
					models.Server{
						Name:        "different-server",
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
						Name:        "disabled_server_0",
						Address:     "127.0.0.1",
						Port:        int64p(1),
						Maintenance: models.ServerMaintenanceEnabled,
					},
					models.Server{
						Name:        "different-server",
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
		RequireOp(haOpReplaceServer, "some-server"),
	)
}

func TestDifferentCerts(t *testing.T) {
	old := State{
		Backends: []Backend{
			Backend{
				Backend: models.Backend{
					Name: "back",
				},
				Servers: []models.Server{
					models.Server{
						Name:           "some-server",
						Address:        "1.2.3.4",
						Port:           int64p(8080),
						Maintenance:    models.ServerMaintenanceDisabled,
						SslCafile:      "test",
						SslCertificate: "test1",
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
						Name:           "some-server",
						Address:        "1.2.3.4",
						Port:           int64p(8080),
						Maintenance:    models.ServerMaintenanceDisabled,
						SslCafile:      "test",
						SslCertificate: "test2",
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
		RequireOp(haOpCreateServer, "some-server"),
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
						Name:        "some-server",
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
						Name:        "some-server",
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
		RequireOp(haOpCreateServer, "some-server"),
	)
}
