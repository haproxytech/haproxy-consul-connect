BIN := haproxy-connect
SOURCES := $(shell find . -name '*.go')
test-haproxy-integ:
	@go test -v -timeout=30m -tags integration ./test/integration/connect/haproxy


all: test bin

$(BIN): $(SOURCES)
	go build -o haproxy-connect -ldflags "-X main.BuildTime=`date -u '+%Y-%m-%dT%H:%M:%SZ'` -X main.GitHash=`git rev-parse HEAD` -X main.Version=$${TRAVIS_TAG:-Dev}"

bin: $(BIN)

test:
	HAPROXY=/usr/sbin/haproxy DATAPLANEAPI=/usr/local/bin/dataplaneapi go test -timeout 600s ${gobuild_args} ./...
check:
	go fmt ./...
	go vet ./...
	git diff --exit-code
travis: check bin test
