.PHONY: all
all: lint test build

# ==============================================================================
# Build Options
VERSION_PACKAGE=github.com/yaoice/meliodas/pkg/version

# ==============================================================================
# Includes

include build/lib/common.mk
include build/lib/help.mk
include build/lib/golang.mk
include build/lib/image.mk
include build/lib/deploy.mk

# ==============================================================================
# Targets

.PHONY: build
build:
	@$(MAKE) go.build

.PHONY: build.all
build.all:
	@$(MAKE) go.build.all

.PHONY: image
image:
	@$(MAKE) image.push

.PHONY: deploy
deploy:
	@$(MAKE) deploy.run

.PHONY: deploy.all
deploy.all:
	@$(MAKE) deploy.run.all

.PHONY: doc
doc:
	@$(MAKE) doc.run

.PHONY: clean
clean:
	@$(MAKE) go.clean

.PHONY: lint
lint:
	@$(MAKE) go.lint

.PHONY: test
test:
	@$(MAKE) go.test

.PHONY: help
help:
	@echo "$$HELPTEXT"

.PHONY: cni.docker
cni.docker: go.build.linux_amd64.host-device go.build.linux_amd64.ipvlan go.build.linux_amd64.tcnp-ipam go.build.linux_amd64.veth-host go.build.linux_amd64.veth-route
	docker build --tag meliodas:v1 -f ./build/docker/cni.Dockerfile ./

.PHONY: update-vendor
update-vendor:
	hack/update-vendor.sh

.PHONY: unit-test
unit-test: update-vendor
	hack/unit-test.sh

.PHONY: install-etcd
install-etcd:
	hack/install-etcd.sh

.PHONY: autogen
autogen: update-vendor
	hack/update-generated-openapi.sh

.PHONY: integration-test
integration-test: install-etcd autogen
	hack/integration-test.sh

.PHONY: verify-gofmt
verify-gofmt:
	hack/verify-gofmt.sh

.PHONY: scheduler.docker
scheduler.docker: autogen
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-w' -o ./build/kube-scheduler cmd/meliodas-scheduler/main.go
	chmod +x ./build/kube-scheduler
	docker build --tag scheduler-plugins:latest -f ./build/docker/kube-scheduler.Dockerfile ./build
	rm ./build/kube-scheduler