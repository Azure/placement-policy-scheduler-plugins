# Directories
ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)

# Binaries
CONTROLLER_GEN_VER := v0.7.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)

GOLANGCI_LINT_VER := v1.41.1
GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)

# Scripts
GO_INSTALL := ./hack/go-install.sh
UPDATE_GENERATED_OPENAPI := ./hack/update-generated-openapi.sh
INSTALL_ETCD := ./hack/install-etcd.sh
RUN_TEST := ./hack/integration-test.sh

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CONTROLLER_GEN):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) sigs.k8s.io/controller-tools/cmd/controller-gen $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) github.com/golangci/golangci-lint/cmd/golangci-lint $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

## --------------------------------------
## Testing
## --------------------------------------

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Run go mod vendor against go.mod
.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

# Install etcd
.PHONY: install-etcd
install-etcd:
	$(INSTALL_ETCD)


# Run unit tests
.PHONY: unit-test
unit-test: autogen manager manifests
	go test ./pkg/... -mod=vendor -coverprofile cover.out

## --------------------------------------
## Linting
## --------------------------------------

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run -v

.PHONY: lint-full
lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false

## --------------------------------------
## Code Generation
## --------------------------------------

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1"

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate code
generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

## --------------------------------------
## Binaries
## --------------------------------------

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager cmd/scheduler/main.go

.PHONY: autogen
autogen: vendor
	$(UPDATE_GENERATED_OPENAPI)

## --------------------------------------
## Integration Testing
## --------------------------------------

# Run integration tests
.PHONY: integration-test
integration-test: install-etcd autogen manager manifests
	$(RUN_TEST)

## --------------------------------------
## E2E Testing
## --------------------------------------

# Run all tests
.PHONY: e2e-test
e2e-test: 
	go test -tags=e2e -v ./test/e2e
