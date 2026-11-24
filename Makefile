# ggufget — build and release helpers.
#
# The version lives in internal/netx (Version const); keep VERSION in sync.
VERSION ?= 1.2.2
BIN     := ggufget
BIN_EXT :=
ifeq ($(OS),Windows_NT)
BIN_EXT := .exe
endif
DIST    := dist
GOFLAGS := -trimpath
LDFLAGS := -s -w
PKGS    := ./...

PLATFORMS := \
	windows/amd64 windows/arm64 \
	linux/amd64   linux/arm64 \
	darwin/amd64  darwin/arm64

.PHONY: all build test vet fmt check dist checksums clean install help

all: check build

help:
	@echo "make build     - build ./dist/$(BIN) for this machine"
	@echo "make test      - run the test suite"
	@echo "make check     - gofmt check + go vet + tests"
	@echo "make dist      - cross-compile $(words $(PLATFORMS)) targets into $(DIST)/"
	@echo "make clean     - remove $(DIST) and test leftovers"

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(BIN)$(BIN_EXT) ./cmd/ggufget

test:
	go test $(PKGS)

vet:
	go vet $(PKGS)

fmt:
	gofmt -w $(shell go list -f '{{.Dir}}' $(PKGS))

check:
	@out=$$(gofmt -l $(shell go list -f '{{.Dir}}' $(PKGS))); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet $(PKGS)
	go test $(PKGS)

# Cross-compile every supported target and write dist/SHA256SUMS.
# CGO_ENABLED=0 keeps the binaries static and dependency free.
dist:
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%%/*}; arch=$${p##*/}; \
		out=$(DIST)/$(BIN)_$(VERSION)_$${os}_$${arch}; \
		[ "$$os" = "windows" ] && out=$$out.exe; \
		echo "  build $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $$out ./cmd/ggufget || exit 1; \
	done
	@$(MAKE) --no-print-directory checksums

checksums:
	@cd $(DIST) && \
	if command -v sha256sum >/dev/null 2>&1; then sha256sum $(BIN)_* > SHA256SUMS; \
	else shasum -a 256 $(BIN)_* > SHA256SUMS; fi
	@echo "wrote $(DIST)/SHA256SUMS"

install: build
	install -m 0755 $(DIST)/$(BIN) /usr/local/bin/$(BIN)

clean:
	rm -rf $(DIST) tmp-test
