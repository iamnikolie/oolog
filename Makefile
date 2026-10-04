# oolog — build & dev tasks. Run `make help` to list targets.
BIN    := oolog
PREFIX ?= $(HOME)/.local/bin
PKGS   := ./...
PKG    := github.com/iamnikolie/oolog/cmd

# Version stamped into the binary. Falls back to the short commit when the tree
# has no tag yet, so a local build is still identifiable.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG).version=$(VERSION)

.DEFAULT_GOAL := build
.PHONY: all build install uninstall run test e2e vet fmt fmt-check tidy check clean help

## build: compile the oolog binary into ./$(BIN)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

## install: build, then symlink $(PREFIX)/$(BIN) -> ./$(BIN) (rebuild is enough to update)
install: build
	mkdir -p $(PREFIX)
	ln -sf $(CURDIR)/$(BIN) $(PREFIX)/$(BIN)
	@echo "installed: $(PREFIX)/$(BIN) -> $(CURDIR)/$(BIN)"

## uninstall: remove the installed symlink
uninstall:
	rm -f $(PREFIX)/$(BIN)
	@echo "removed: $(PREFIX)/$(BIN)"

## run: build and run (pass args via ARGS=, e.g. make run ARGS="tail -n 5")
run: build
	./$(BIN) $(ARGS)

## test: run the unit test suite
test:
	go test -race $(PKGS)

## e2e: run end-to-end tests against OpenObserve in Docker (skips if docker is unavailable)
e2e:
	go test -tags e2e -count=1 -timeout 10m -v ./e2e/...

## vet: run go vet
vet:
	go vet $(PKGS)

## fmt: gofmt all Go sources in place
fmt:
	gofmt -w cmd internal main.go

## fmt-check: fail if any Go source is not gofmt-clean
fmt-check:
	@out=$$(gofmt -l cmd internal main.go); \
	if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi
	@echo "gofmt clean"

## tidy: sync go.mod/go.sum with the source
tidy:
	go mod tidy

## check: fmt-check + vet + test (the pre-commit / CI gate)
check: fmt-check vet test

## all: full gate (check) then build
all: check build

## clean: remove the built binary
clean:
	rm -f $(BIN)
	rm -rf dist

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
