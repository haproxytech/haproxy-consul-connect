package haproxy

type Stats struct {
	dpapi *dataplaneClient
}

func (s *Stats) Run() error {
	return nil
}
