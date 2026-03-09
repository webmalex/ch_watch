BINARY := ch_watch
CMD := ./cmd/ch_watch
BIN_DIR := ./bin
OUTPUT := $(BIN_DIR)/$(BINARY)
GO_PACKAGES := ./...
GOFILES := $(shell find cmd internal -type f -name '*.go' -print)

.PHONY: build install clean fmt fmt-check test test-race test-cover vet lint vuln smoke-run smoke-watch check check-full

build:
	mkdir -p $(BIN_DIR)
	go build -o $(OUTPUT) $(CMD)

install:
	go install $(CMD)

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

check: fmt-check test vet build

check-full: check test-race test-cover lint vuln
