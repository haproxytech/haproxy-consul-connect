package haproxy

type Secret string

func (s Secret) String() string {
	return "<hidden>"
}

type BackendServer struct {
	Name string
	Host string
	Port int

	TLS           bool
	ServerCA      []Secret
	ServerCAPath  string
	ClientCRT     Secret
	ClientKey     Secret
	ClientCRTPath string
}

type Backend struct {
	Name    string
	Servers []BackendServer
}

type Frontend struct {
	Name           string
	BindAddr       string
	BindPort       int
	DefaultBackend string

	SPOE bool

	TLS           bool
	ClientCA      []Secret
	ClientCAPath  string
	ServerCRT     Secret
	ServerKey     Secret
	ServerCRTPath string
}

type Configuration struct {
	Frontends    []Frontend
	Backends     []Backend
	SocketPath   string
	SPOEConfPath string
}
