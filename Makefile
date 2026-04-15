.PHONY: build test lint helm-lint docker-build

BINARY := capp-status-server
IMAGE  ?= ghcr.io/dana-team/capp-status-server:latest

build:
	go build -o $(BINARY) ./cmd/status-server

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

helm-lint:
	helm lint charts/capp-monitoring

docker-build:
	docker build -f docker/Dockerfile -t $(IMAGE) .
