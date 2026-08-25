.PHONY: helm-lint docker-build

IMAGE ?= ghcr.io/dana-team/capp-benchmark-runner:latest

helm-lint:
	helm lint charts/capp-monitoring

docker-build:
	docker build -f docker/Dockerfile -t $(IMAGE) .
