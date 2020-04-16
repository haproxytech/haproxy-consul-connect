BIN := haproxy-connect
SOURCES := $(shell find . -name '*.go')

all: test bin

$(BIN): $(SOURCES)
	go build -o haproxy-connect -ldflags "-X main.BuildTime=`date -u '+%Y-%m-%dT%H:%M:%SZ'` -X main.GitHash=`git rev-parse HEAD` -X main.Version=$${TRAVIS_TAG:-Dev}"

bin: $(BIN)

test:
	go test -timeout 30s ${gobuild_args} ./...
check:
	go fmt ./...
	go vet ./...
	git diff --exit-code
travis: check bin test
