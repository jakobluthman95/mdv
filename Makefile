BIN ?= $(HOME)/.local/bin/mdv
DOCS ?= $(HOME)/dev/md

.PHONY: build install

build:
	go build -o mdv .

install:
	mkdir -p $(dir $(BIN)) $(DOCS)
	go build -o $(BIN) .
	[ -e $(DOCS)/welcome.md ] || cp welcome.md $(DOCS)/welcome.md
	@echo "installed $(BIN)"
