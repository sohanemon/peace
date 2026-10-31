.PHONY: build clean install

build:
	go build -trimpath -ldflags="-s -w" -o bin/peace ./cmd/peace

clean:
	rm -f bin/peace

install:
	go install -trimpath -ldflags="-s -w" ./cmd/peace

GO_BIN := $(shell go env GOBIN)
ifeq ($(GO_BIN),)
GO_BIN := $(shell go env GOPATH)/bin
endif

uninstall:
	rm -f "$(GO_BIN)/peace"
