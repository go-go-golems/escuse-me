.PHONY: all test build lint lintmax docker-lint gosec govulncheck goreleaser tag-major tag-minor tag-patch release bump-glazed install gifs codeql-local

VERSION ?= $(shell svu)
COMMIT ?= $(shell git rev-parse --short HEAD)
DIRTY ?= $(shell git diff --quiet || echo "dirty")
LDFLAGS=-ldflags "-X main.version=$(VERSION)-$(COMMIT)-$(DIRTY)"

all: test build

VERSION=v0.0.7

TAPES=$(shell ls doc/vhs/*tape)
gifs: $(TAPES)
	for i in $(TAPES); do vhs < $$i; done

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v2.0.2 golangci-lint run -v

lint:
	golangci-lint run -v

lintmax:
	golangci-lint run -v --max-same-issues=100

gosec:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude=G101,G304,G301,G306,G204 -exclude-dir=.history ./...

govulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

test:
	go test ./...

build:
	go generate ./...
	go build ./...

goreleaser:
	goreleaser release --skip=sign --snapshot --clean

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

release:
	git push --tags
	GOPROXY=proxy.golang.org go list -m github.com/go-go-golems/escuse-me@$(shell svu current)

bump-glazed:
	go get -u -t -x github.com/go-go-golems/glazed@latest
	go get -u -t -x github.com/go-go-golems/clay@latest
	go get -u -t -x github.com/go-go-golems/parka@latest
	go get -u -t -x github.com/go-go-golems/go-emrichen@latest
	go get -u -t -x github.com/go-go-golems/geppetto@latest
	go mod tidy

ESCUSE_ME_BINARY=$(shell which escuse-me)

install:
	go build -o ./dist/escuse-me ./cmd/escuse-me && \
		cp ./dist/escuse-me $(ESCUSE_ME_BINARY)

# Path to CodeQL CLI - adjust based on installation location
CODEQL_PATH ?= $(shell which codeql)
# Path to CodeQL queries - adjust based on where you cloned the repository
CODEQL_QUERIES ?= $(HOME)/codeql-go/ql/src/go

# Create CodeQL database and run analysis
codeql-local:
	@if [ -z "$(CODEQL_PATH)" ]; then echo "CodeQL CLI not found. Install from https://github.com/github/codeql-cli-binaries/releases"; exit 1; fi
	@if [ ! -d "$(CODEQL_QUERIES)" ]; then echo "CodeQL queries not found. Clone from https://github.com/github/codeql-go"; exit 1; fi
	$(CODEQL_PATH) database create --language=go --source-root=. ./codeql-db
	$(CODEQL_PATH) database analyze ./codeql-db $(CODEQL_QUERIES)/Security --format=sarif-latest --output=codeql-results.sarif
	@echo "Results saved to codeql-results.sarif"
