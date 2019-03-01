package haproxy

type Haproxy interface {
	AddFrontend(Frontend) error
	AddBackend(Backend) error
	AddServer(backend, host string, port int) error
}
