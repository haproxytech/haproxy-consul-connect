# HAProxy Connect

[Consul Connect](https://www.consul.io/docs/connect/index.html) provides a simple way to setup service mesh between your services by offloading the load balancing logic to a sidecar process running alongside your application. It exposes a local port per service and takes care of forwarding the traffic to alives instances of the services your application wants to target. Additionnaly, the traffic is automatically encrypted using TLS, and can be restricted by using [intentions](https://www.consul.io/docs/connect/intentions.html) by selecting what services can or cannot call your application.
[HAProxy](https://www.haproxy.org) is a proven load balancer widely used in the industry for its high performance and reliability.
HAProxy Connect allows to use HAProxy as a load balancer for Consul Connect.

## Architecture

Three components are used :
* HAProxy, the load balancer
* Dataplane API, which provides a high level configuration interface for HAProxy
* HAProxy Connect, that configures HAProxy through the Dataplane API with information pulled from Consul.

To handle intentions, HAProxy Connect, sets up a SPOE filter on the application public frontend. On each connection HAProxy checks with HAProxy Connect that the incomming connection is authorized. HAProxy Connect parses the request certificates and in turn calls the Consul agent to know wether it should tell HAProxy to allow or deny the connection.

![architecture](https://github.com/haproxytech/haproxy-consul-connect/blob/master/docs/architecture.png)

## Requirements

* HAProxy >= v1.9 (http://www.haproxy.org/)
* DataplaneAPI >= v1.2 (https://www.haproxy.com/documentation/hapee/1-9r1/configuration/dataplaneapi/)

## How to use

Please see this for the current app parameters:
```
./haproxy-consul-connect --help
```

## Minimal working example

You will need 2 SEPARATE servers within the same network, one for the server and another for the client.
On both you need all 3 binaries - consul, dataplaneapi and haproxy-consul-connect.

### The services

#### Server

Create this config file for consul:
```
{
  "service": {
    "name": "server",
    "port": 8181,
    "connect": { "sidecar_service": {} }
  }
}
```
Run consul:
```
consul agent -dev -config-file client.cfg
```
Run the test server:
```
python -m SimpleHTTPServer 8181
```
Run haproxy-connect (assuming that `haproxy` and `dataplaneapi` are $PATH):
```
haproxy-consul-connect -sidecar-for server
```

#### Client

Create this config file for consul:
```
{
  "service": {
    "name": "client",
    "port": 8080,
    "connect": {
      "sidecar_service": {
        "proxy": {
          "upstreams": [
            {
              "destination_name": "server",
              "local_bind_port": 9191
            }
          ]
        }
      }
    }
  }
}
```
Run consul:
```
consul agent -dev -config-file server.cfg
```
Run haproxy-connect (assuming that `haproxy` and `dataplaneapi` in $PATH) :
```
haproxy-consul-connect -sidecar-for client -log-level debug
```

### Testing

On the server:
```
curl -v 127.0.0.1:9191/
```

## Contributing

For commit messages and general style please follow the haproxy project's [CONTRIBUTING guide](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING) and use that where applicable.
