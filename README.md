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

![architecture](https://github.com/criteo/haproxy-consul-connect/blob/master/docs/architecture.png)

## Requirements

* HAProxy >= v1.9 (http://www.haproxy.org/)
* DataplaneAPI >= v1.2 (https://www.haproxy.com/documentation/hapee/1-9r1/configuration/dataplaneapi/)

## How to use

```
./haproxy-consul-connect --help
Usage of ./haproxy-consul-connect:
  -dataplane string
    	Dataplane binary path (default "dataplane-api")
  -enable-intentions
    	Enable Connect intentions
  -haproxy string
    	Haproxy binary path (default "haproxy")
  -haproxy-cfg-base-path string
    	Haproxy binary path (default "/tmp")
  -http-addr string
    	Consul agent address (default "127.0.0.1:8500")
  -log-level string
    	Log level (default "INFO")
  -sidecar-for string
    	The consul service id to proxy
  -sidecar-for-tag string
    	The consul service id to proxy
  -stats-addr string
    	Listen addr for stats server
  -stats-service-register
    	Register a consul service for connect stats
  -token string
    	Consul ACL token./haproxy-consul-connect --help
```

## Contributing

For commit messages and general style please follow the haproxy project's [CONTRIBUTING guide](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING) and use that where applicable.
