package haproxyconfig

const haproxyConfTmpl = `
{{$out := .}}

global
    stats socket {{.SocketPath}} mode 600 level admin expose-fd listeners
    stats timeout 2m

{{range $fe := .Frontends}}
frontend {{$fe.Name}}
    mode tcp
    bind {{$fe.BindAddr}}:{{$fe.BindPort}}{{if $fe.TLS}} ssl crt {{$fe.ServerCRTPath}} ca-file {{$fe.ClientCAPath}} verify required{{end}}
    option tcplog
    timeout client 1m
    default_backend {{$fe.DefaultBackend}}
    {{if $fe.SPOE}}
    filter spoe engine intentions config {{$out.SPOEConfPath}}
    tcp-request content reject if { var(sess.connect.auth) -m int eq 0 }
    {{end}}
{{end}}

{{range $be := .Backends}}
backend {{$be.Name}}
    mode tcp
    option redispatch
    balance roundrobin
    timeout connect 10s
	timeout server 1m
	{{range $s := $be.Servers}}
	server {{$s.Name}} {{$s.Host}}:{{$s.Port}}{{if $s.TLS}} ssl crt {{$s.ClientCRTPath}} ca-file {{$s.ServerCAPath}} verify required{{end}}
	{{end}}
{{end}}
`

const spoeConfTmpl = `
[intentions]

spoe-agent intentions-agent
    messages check-intentions

    option var-prefix connect

    timeout hello      100ms
    timeout idle       30s
    timeout processing 15ms

    use-backend spoe_back

spoe-message check-intentions
    args ip=src cert=ssl_c_der
    event on-frontend-tcp-request
`
