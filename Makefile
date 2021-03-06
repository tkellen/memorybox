export CGO_ENABLED = 0

all: build

fmt:
	go fmt ./...

lint:
	golint ./...

vet:
	go vet ./...

test:
	go test -v ./... -coverprofile /dev/null

build: fmt lint vet test
	go build ./...

run:
	go run ./...

.PHONY: all fmt lint vet test build run
