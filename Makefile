.PHONY: gifs

all: gifs

VERSION=v0.0.7

TAPES=$(shell ls doc/vhs/*tape)
gifs: $(TAPES)
	for i in $(TAPES); do vhs < $$i; done

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v

lint:
	golangci-lint run -v

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
	go mod tidy

exhaustive:
	golangci-lint run -v --enable=exhaustive

ESCUSE_ME_BINARY=$(shell which escuse-me)

install:
	go build -o ./dist/escuse-me ./cmd/escuse-me && \
		cp ./dist/escuse-me $(ESCUSE_ME_BINARY)
