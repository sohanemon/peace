.PHONY: build clean install test update

PLUGIN_DIR ?= $(HOME)/.config/omarchy/plugins/io.github.sohanemon.omapeace

build:
	go build -trimpath -ldflags="-s -w" -o bin/peace ./cmd/peace

clean:
	rm -f bin/peace

install:
	go install -trimpath -ldflags="-s -w" ./cmd/peace

test:
	go test ./...

update: test build
	command -v rsync >/dev/null
	command -v omarchy-restart-shell >/dev/null
	mkdir -p "$(PLUGIN_DIR)/bin"
	rsync -a --exclude='/.git/' --exclude='/.agents/' --exclude='/.codex/' --exclude='/.aws/' --exclude='/bin/peace' ./ "$(PLUGIN_DIR)/"
	install -m 755 bin/peace "$(PLUGIN_DIR)/bin/peace"
	omarchy-restart-shell

GO_BIN := $(shell go env GOBIN)
ifeq ($(GO_BIN),)
GO_BIN := $(shell go env GOPATH)/bin
endif

uninstall:
	rm -f "$(GO_BIN)/peace"
