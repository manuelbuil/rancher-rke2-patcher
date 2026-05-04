BINARY ?= rke2-patcher
COMPONENT ?= traefik
EXEC_MODE ?= binary
IMAGE_BUNDLES_DIR ?= $(CURDIR)/tests/docker/airgap/bundles

.PHONY: help build version image-cve image-list image-patch test-docker-default test-docker-calico-traefik docker-build test-docker-airgap

help:
	@echo "Targets:"
	@echo "  make build"
	@echo "  make version"
	@echo "  make image-cve COMPONENT=traefik"
	@echo "  make image-list COMPONENT=traefik"
	@echo "  make image-patch COMPONENT=traefik"
	@echo "  make test-docker-image-cve EXEC_MODE=binary|pod"
	@echo "  make test-docker-default"
	@echo "  make test-docker-calico-traefik"
	@echo "  make test-docker-airgap [IMAGE_BUNDLES_DIR=/path/to/bundles]  # default: tests/docker/airgap/bundles"

build:
	CGO_ENABLED=0 go build -o $(BINARY) .

version: build
	./$(BINARY) --version

image-cve: build
	./$(BINARY) image-cve $(COMPONENT)

image-list: build
	./$(BINARY) image-list $(COMPONENT)

image-patch: build
	./$(BINARY) image-patch $(COMPONENT)

test-docker-image-cve: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/image_cve/image_cve_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-image-list: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/image_list/image_list_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-image-patcher: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/patch_components/patch_components_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-image-patcher-traefik-flannel: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/flannel_traefik_patch_components/flannel_traefik_patch_components_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-reconcile: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/reconcile/reconcile_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-image-cve-local: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/image_cve_local/image_cve_local_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-merging-values: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/merging_values/merging_values_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-reconcile-upgrade: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=80m ./tests/docker/reconcile_upgrade/reconcile_upgrade_test.go -ginkgo.v -rke2Version v1.35.3+rke2r3 -patcherBin $(CURDIR)/$(BINARY)

test-docker-airgap: build
	EXEC_MODE=$(EXEC_MODE) go test -v -timeout=90m ./tests/docker/airgap/airgap_test.go -ginkgo.v -rke2Version v1.35.4+rke2r1 -patcherBin $(CURDIR)/$(BINARY) -imageBundlesDir $(IMAGE_BUNDLES_DIR)

VERSION ?= $(shell grep '^const version' internal/cmd/app.go | cut -d '"' -f2)

build-image:
	docker build -t mbuilsuse/rke2-patcher:$(VERSION) .
