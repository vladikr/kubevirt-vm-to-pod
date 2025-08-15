# Makefile for kubevirt-vm-to-pod

BINARY_NAME ?= kubevirt-vm-to-pod
PODMAN_REPO ?= quay.io/vladikr/kubevirt-vm-to-pod-tool
PODMAN_TAG ?= latest
PODMAN_IMG ?= $(PODMAN_REPO):$(PODMAN_TAG)
PROXY_BINARY_NAME ?= console-proxy

GO_BUILD_ENV ?= CGO_ENABLED=0 GOOS=linux GOARCH=amd64
GO_TEST_FLAGS ?= -v -race

DEV_MODE ?= false  # Set to true for dev builds (dynamically replaces KubeVirt dep with main branch)

.PHONY: all build test podman-build podman-build-dev podman-push clean build-proxy podman-build-proxy podman-push-proxy

all: build test build-proxy

build:
	$(GO_BUILD_ENV) go build -o $(BINARY_NAME) ./cmd

test:
	go test $(GO_TEST_FLAGS) ./...

clean:
	rm -f $(BINARY_NAME)

podman-build: podman-build-proxy
	podman build --build-arg dev_mode=$(DEV_MODE) -t $(PODMAN_IMG) .

podman-build-dev: DEV_MODE = true
podman-build-dev: podman-build-proxy
	podman build --build-arg dev_mode=true -t $(PODMAN_IMG)-dev .

podman-push: podman-push-proxy
	podman push $(PODMAN_IMG)

build-proxy:
	$(GO_BUILD_ENV) go build -o $(PROXY_BINARY_NAME) ./pkg/transformer/proxy.go

podman-build-proxy:
	podman build -f Dockerfile.proxy --build-arg dev_mode=$(DEV_MODE) -t $(PODMAN_REPO)-proxy:$(PODMAN_TAG) .

podman-push-proxy: podman-build-proxy
	podman push $(PODMAN_REPO)-proxy:$(PODMAN_TAG)
