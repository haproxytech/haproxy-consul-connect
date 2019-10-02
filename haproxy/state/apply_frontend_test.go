package state

import (
	"testing"

	"github.com/haproxytech/models"
	"github.com/stretchr/testify/require"
)

func TestAddFrontend(t *testing.T) {
	old := State{}
	new := State{
		Frontends: []Frontend{
			Frontend{
				Frontend: models.Frontend{
					Name: "front",
				},
				Bind: models.Bind{
					Name: "front_bind",
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpCreateFrontend, "front"),
		RequireOp(haOpCreateBind, "front_bind"),
	)
}

func TestNoChangeFrontend(t *testing.T) {
	old := State{
		Frontends: []Frontend{
			Frontend{
				Frontend: models.Frontend{
					Name:          "front",
					ClientTimeout: int64p(1),
				},
				Bind: models.Bind{
					Name: "front_bind",
				},
			},
		},
	}
	new := State{
		Frontends: []Frontend{
			Frontend{
				Frontend: models.Frontend{
					Name:          "front",
					ClientTimeout: int64p(1),
				},
				Bind: models.Bind{
					Name: "front_bind",
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t)
}

func TestRemoveFrontend(t *testing.T) {
	old := State{
		Frontends: []Frontend{
			Frontend{
				Frontend: models.Frontend{
					Name: "front",
				},
				Bind: models.Bind{
					Name: "front_bind",
				},
			},
		},
	}
	new := State{}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpDeleteFrontend, "front"),
	)
}

func TestChangeFrontend(t *testing.T) {
	old := State{
		Frontends: []Frontend{
			Frontend{
				Frontend: models.Frontend{
					Name: "front",
				},
				Bind: models.Bind{
					Name: "front_bind",
				},
			},
		},
	}
	new := State{
		Frontends: []Frontend{
			Frontend{
				Frontend: models.Frontend{
					Name:          "front",
					ClientTimeout: int64p(1),
				},
				Bind: models.Bind{
					Name: "front_bind",
				},
			},
		},
	}

	ha := &fakeHA{}

	err := Apply(ha, old, new)
	require.Nil(t, err)

	ha.RequireOps(t,
		RequireOp(haOpDeleteFrontend, "front"),
		RequireOp(haOpCreateFrontend, "front"),
		RequireOp(haOpCreateBind, "front_bind"),
	)
}
