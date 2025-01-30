include .bingo/Variables.mk

# Image URL to use all building/pushing image targets
DOCKER_IMAGE_REPO ?= quay.io/thanos/thanos-operator
DOCKER_IMAGE_TAG  ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))-$(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD)
IMG ?= ${DOCKER_IMAGE_REPO}:${DOCKER_IMAGE_TAG}
IMG_MAIN ?= ${DOCKER_IMAGE_REPO}:main
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

FILES_TO_FMT      	 ?= $(shell find . -path ./vendor -prune -o -name '*.go' -print)
MD_FILES_TO_FORMAT = $(filter-out docs/api.md, $(shell find docs -name "*.md")) $(shell ls *.md)
MDOX_VALIDATE_CONFIG ?= .mdox.validate.yaml
CRD_REF_DOCS_CONFIG ?= .crd_ref.yaml

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: deps
deps: ## Ensures fresh go.mod and go.sum.
	@go mod tidy
	@go mod verify

.PHONY: format
format: ## Formats Go code.
format: $(GOIMPORTS)
	go fmt ./...
	@echo ">> formatting code"
	@$(GOIMPORTS) -local github.com/thanos-community/thanos-operator -w $(FILES_TO_FMT)

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate format vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -v $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test -v ./test/e2e/ -v -ginkgo.v

define require_clean_work_tree
	@git update-index -q --ignore-submodules --refresh

    @if ! git diff-files --quiet --ignore-submodules --; then \
        echo >&2 "$1: you have unstaged changes."; \
        git diff-files --name-status -r --ignore-submodules -- >&2; \
        echo >&2 "Please commit or stash them."; \
        exit 1; \
    fi

    @if ! git diff-index --cached --quiet HEAD --ignore-submodules --; then \
        echo >&2 "$1: your index contains uncommitted changes."; \
        git diff-index --cached --name-status -r --ignore-submodules HEAD -- >&2; \
        echo >&2 "Please commit or stash them."; \
        exit 1; \
    fi

endef

.PHONY: lint ## Runs various static analysis against our code.
lint: $(FAILLINT) $(GOLANGCI_LINT) format deps
	$(call require_clean_work_tree,"detected not clean main before running lint")
	@echo ">> verifying modules being imported"
	@$(FAILLINT) -paths "fmt.{Print,Printf,Println}" -ignore-tests ./...
	@echo ">> examining all of the Go files"
	@go vet -stdmethods=false ./...
	@echo ">> linting all of the Go files GOGC=${GOGC}"
	@$(GOLANGCI_LINT) run
	$(call require_clean_work_tree,"run make lint file and commit changes.")

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Docs

define require_clean_work_tree
	@git update-index -q --ignore-submodules --refresh

	@if ! git diff-files --quiet --ignore-submodules --; then \
		echo >&2 "cannot $1: you have unstaged changes."; \
		git diff -r --ignore-submodules -- >&2; \
		echo >&2 "Please commit or stash them."; \
		exit 1; \
	fi

	@if ! git diff-index --cached --quiet HEAD --ignore-submodules --; then \
		echo >&2 "cannot $1: your index contains uncommitted changes."; \
		git diff --cached -r --ignore-submodules HEAD -- >&2; \
		echo >&2 "Please commit or stash them."; \
		exit 1; \
	fi

endef

.PHONY: generate-api-docs
generate-api-docs: ## Generate documentation from CRD
generate-api-docs: $(CRD_REF_DOCS) generate
	$(CRD_REF_DOCS) --source-path=./api --renderer=markdown --output-path=./docs --output-mode=group --config=$(CRD_REF_DOCS_CONFIG)
	mv ./docs/monitoring.thanos.io.md ./docs/api.md

.PHONY: docs
docs: ## Generates docs for all commands, localise links, ensure GitHub format.
docs: build generate-api-docs $(MDOX)
	@echo ">> generating docs"
	PATH="${PATH}:$(GOBIN)" $(MDOX) fmt $(MD_FILES_TO_FORMAT)

.PHONY: check-docs
check-docs: ## Checks docs against discrepancy with flags, links, white noise.
check-docs: build generate-api-docs $(MDOX)
	@echo ">> checking docs"
	PATH="${PATH}:$(GOBIN)" $(MDOX) fmt -l --links.validate.config-file=$(MDOX_VALIDATE_CONFIG) $(MD_FILES_TO_FORMAT)
	$(call require_clean_work_tree,'run make docs and commit changes')

##@ Build

.PHONY: build
build: manifests generate format vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate format vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# Used for e2e testing.
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build --load -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
# Used for CI pushing.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} --tag ${IMG_MAIN} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

# Add a bundle.yaml file with CRDs and deployment, with kustomize config.
.PHONY: bundle
bundle: manifests generate format kustomize ## Generate a consolidated YAML with CRDs and deployment.
	@echo ">> generating bundle.yaml (override image using IMG_MAIN)"
	mkdir -p dist
	@if [ -d "config/crd" ]; then \
		$(KUSTOMIZE) build config/crd > bundle.yaml; \
	fi
	echo "---" >> bundle.yaml  # Add a document separator before appending
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_MAIN}
	$(KUSTOMIZE) build config/default >> bundle.yaml


ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply --server-side -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	@echo ">> deploying controller (override image using IMG_MAIN)"
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_MAIN}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply --server-side -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: install-example
install-example: manifests kustomize ## Install example definitions to K8s cluster specified in ~/.kube/config.
install-example: ## Installs minio and definitions of all components to be used with the operator.
install-example: ## Ensure you run make install and make deploy in ns of choice before this command
	$(KUBECTL) apply -f test/utils/testdata/
	$(KUSTOMIZE) build config/samples | $(KUBECTL) apply -f -


.PHONY: install-sample
install-sample: install deploy
	$(KUSTOMIZE) build config/samples | $(KUBECTL) apply -f -


.PHONY: uninstall-example
uninstall-example: manifests kustomize ## Uninstall example definitions from K8s cluster specified in ~/.kube/config.
	$(KUBECTL) delete -f test/utils/testdata/
	$(KUSTOMIZE) build config/samples | $(KUBECTL) delete -f -  

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.4.3
CONTROLLER_TOOLS_VERSION ?= v0.16.1
ENVTEST_VERSION ?= latest

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
