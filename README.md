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

```
haproxy-connect                                                                                      \
    -sidecar-for <your-service-id>                                                                   \
                                                                                                     \
    -enable-intentions                      # wether or not to enbale intentions                     \

    -haproxy-cfg-base-path <path>           # base path to store haproxy config in,                  \
                                            # will generate a unique directory per run under this,   \
                                            # defaults to /tmp                                       \


    -haproxy <haproxy binary path>          # no required if in your path                            \
    -dataplane <dataplane-api binary path>  # no required if in your path                            \
```