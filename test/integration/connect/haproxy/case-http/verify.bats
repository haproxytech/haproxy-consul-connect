#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :8080" {
  retry_default curl -f -s localhost:8080/stats -o /dev/null
}

@test "s2 proxy admin is up on :8181" {
  retry_default curl -f -s localhost:8181/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1
}

# @test "s1 upstream should have healthy endpoints for s2" {
#   # protocol is configured in an upstream override so the cluster name is customized here
#   assert_upstream_has_endpoints_in_status 127.0.0.1:8080 1a47f6e1~s2.default.primary HEALTHY 1
# }

@test "s1 upstream should be able to connect to s2 with http/1.1" {
  run retry_default curl --http1.1 -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

# @test "s1 proxy should have been configured with http connection managers" {
#   LISTEN_FILTERS=$(get_envoy_listener_filters localhost:19000)
#   PUB=$(echo "$LISTEN_FILTERS" | grep -E "^public_listener:" | cut -f 2 -d ' ' )
#   UPS=$(echo "$LISTEN_FILTERS" | grep -E "^s2:" | cut -f 2 -d ' ' )

#   echo "LISTEN_FILTERS = $LISTEN_FILTERS"
#   echo "PUB = $PUB"
#   echo "UPS = $UPS"

#   [ "$PUB" = "envoy.http_connection_manager" ]
#   [ "$UPS" = "envoy.http_connection_manager" ]
# }

# @test "s2 proxy should have been configured with an http connection manager" {
#   LISTEN_FILTERS=$(get_envoy_listener_filters localhost:19001)
#   PUB=$(echo "$LISTEN_FILTERS" | grep -E "^public_listener:" | cut -f 2 -d ' ' )

#   echo "LISTEN_FILTERS = $LISTEN_FILTERS"
#   echo "PUB = $PUB"

#   [ "$PUB" = "envoy.http_connection_manager" ]
# }

# @test "s1 proxy should have been configured with http rbac filters" {
#   HTTP_FILTERS=$(get_envoy_http_filters localhost:19000)
#   PUB=$(echo "$HTTP_FILTERS" | grep -E "^public_listener:" | cut -f 2 -d ' ' )
#   UPS=$(echo "$HTTP_FILTERS" | grep -E "^s2:" | cut -f 2 -d ' ' )

#   echo "HTTP_FILTERS = $HTTP_FILTERS"
#   echo "PUB = $PUB"
#   echo "UPS = $UPS"

#   [ "$PUB" = "envoy.filters.http.rbac,envoy.router" ]
#   [ "$UPS" = "envoy.router" ]
# }

# @test "s2 proxy should have been configured with http rbac filters" {
#   HTTP_FILTERS=$(get_envoy_http_filters localhost:19001)
#   PUB=$(echo "$HTTP_FILTERS" | grep -E "^public_listener:" | cut -f 2 -d ' ' )

#   echo "HTTP_FILTERS = $HTTP_FILTERS"
#   echo "PUB = $PUB"

#   [ "$PUB" = "envoy.filters.http.rbac,envoy.router" ]
# }
