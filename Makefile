# Makefile for kubevirt-vm-to-pod

BINARY_NAME ?= kubevirt-vm-to-pod
PODMAN_REPO ?= quay.io/vladikr/kubevirt-vm-to-pod-tool
PODMAN_TAG ?= latest
PODMAN_IMG ?= $(PODMAN_REPO):$(PODMAN_TAG)

GO_BUILD_ENV ?= CGO_ENABLED=0 GOOS=linux GOARCH=amd64
GO_TEST_FLAGS ?= -v -race

.PHONY: all build test podman-build podman-push clean

all: build test

build:
	$(GO_BUILD_ENV) go build -o $(BINARY_NAME) ./cmd

test:
	go test $(GO_TEST_FLAGS) ./...

podman-build:
	podman build -t $(PODMAN_IMG) .

podman-push: podman-build
	podman push $(PODMAN_IMG)

clean:
	rm -f $(BINARY_NAME)
