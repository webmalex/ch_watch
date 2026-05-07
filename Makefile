BINARY := ch_watch
CMD := ./cmd/ch_watch
MAINT_CMD := ./cmd/ch_watch_maint
BIN_DIR := ./bin
OUTPUT := $(BIN_DIR)/$(BINARY)
GO_PACKAGES := ./...
GOFILES := $(shell find cmd internal -type f -name '*.go' -print)
VERSION := $(file < VERSION)
LDFLAGS := -X github.com/webmalex/ch_watch/internal/version.Version=$(VERSION)

.PHONY: build install clean fmt fmt-check test test-race test-cover vet lint vuln smoke-run smoke-watch deps_accept deps_accept_dry_run hooks-install hooks-update hooks-run hooks-run-push hooks-run-manual check check-full release-check pre-release

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(OUTPUT) $(CMD)

install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

clean:
	rm -rf $(BIN_DIR) coverage.out

fmt:
	gofmt -w $(GOFILES)

fmt-check:
	@out="$$(gofmt -l $(GOFILES))"; \
	if [ -n "$$out" ]; then \
		printf '%s\n' "$$out"; \
		exit 1; \
	fi

test:
	go test $(GO_PACKAGES)

test-race:
	go test -race $(GO_PACKAGES)

test-cover:
	go test -coverprofile=coverage.out -covermode=atomic $(GO_PACKAGES)
	go tool cover -func=coverage.out

vet:
	go vet $(GO_PACKAGES)

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo 'install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest'; exit 1; }
	golangci-lint run

vuln:
	@command -v govulncheck >/dev/null 2>&1 || { echo 'install govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest'; exit 1; }
	govulncheck $(GO_PACKAGES)

smoke-run:
	go run $(CMD) run ./demo/ch/dev/tmp.sql --dry-run

smoke-watch:
	go run $(CMD) watch --root ./demo/ch --dry-run

deps_accept:
	go run $(MAINT_CMD) $(if $(PR),--pr $(PR),)

deps_accept_dry_run:
	go run $(MAINT_CMD) --dry-run $(if $(PR),--pr $(PR),)

hooks-install:
	pre-commit install --install-hooks -t pre-commit -t pre-push

hooks-update:
	pre-commit autoupdate

hooks-run:
	pre-commit run --all-files

hooks-run-push:
	pre-commit run --all-files --hook-stage pre-push

hooks-run-manual:
	pre-commit run --all-files --hook-stage manual

check: fmt-check test vet build

check-full: check test-race test-cover lint vuln

release-check: check-full smoke-run

pre-release: release-check
