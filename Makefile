BIN ?= $(HOME)/.local/bin/mdv
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install

build:
	go build -ldflags "$(LDFLAGS)" -o mdv .

# The docs folder and its welcome doc are created by mdv itself on first run.
install:
	mkdir -p $(dir $(BIN))
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .
	@echo "installed $(BIN) ($(VERSION))"
