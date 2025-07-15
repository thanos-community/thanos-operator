include .bingo/Variables.mk

# Image URL to use all building/pushing image targets
DOCKER_IMAGE_REPO ?= quay.io/thanos/thanos-operator
DOCKER_IMAGE_TAG  ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))-$(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD)

# This needs to be updated to the version of this operator.
# This is used to build the bundle image and the catalog image.
# Needs to a semantic version, can't be a main branch tag
# TODO(saswatamcode): Release process
VERSION  ?= 0.1.0
IMG ?= ${DOCKER_IMAGE_REPO}:${DOCKER_IMAGE_TAG}
IMG_MAIN ?= ${DOCKER_IMAGE_REPO}:main

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.40.0
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

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# thanos/thanos-operator-bundle:$VERSION and thanos/thanos-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= quay.io/thanos/thanos-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

FILES_TO_FMT      	 ?= $(shell find . -path ./vendor -prune -o -name '*.go' -print)
MD_FILES_TO_FORMAT = $(filter-out docs/api.md, $(shell find docs -name "*.md")) $(shell ls *.md)
MDOX_VALIDATE_CONFIG ?= .mdox.validate.yaml
CRD_REF_DOCS_CONFIG ?= .crd_ref.yaml

.PHONY: all
all: build bundle docs

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
lint: all
lint: $(FAILLINT) $(GOLANGCI_LINT) deps
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
	@VERSION=$$(cat VERSION) && \
	REVISION=$$(git rev-parse HEAD) && \
	BRANCH=$$(git rev-parse --abbrev-ref HEAD) && \
	BUILDUSER=$$(whoami)@$$HOSTNAME && \
	BUILDDATE=$$(date +%Y%m%d-%H:%M:%S) && \
	go build -o bin/manager \
		-ldflags="-s -w \
		-X github.com/prometheus/common/version.Version=$$VERSION \
		-X github.com/prometheus/common/version.Revision=$$REVISION \
		-X github.com/prometheus/common/version.Branch=$$BRANCH \
		-X github.com/prometheus/common/version.BuildUser=$$BUILDUSER \
		-X github.com/prometheus/common/version.BuildDate=$$BUILDDATE" \
		cmd/main.go

.PHONY: run
run: manifests generate format vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# Used for e2e testing.
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	@VERSION=$$(cat VERSION) && \
	REVISION=$$(git rev-parse HEAD) && \
	BRANCH=$$(git rev-parse --abbrev-ref HEAD) && \
	BUILDUSER=$$(whoami)@$$HOSTNAME && \
	BUILDDATE=$$(date +%Y%m%d-%H:%M:%S) && \
	$(CONTAINER_TOOL) build --load \
		--build-arg VERSION=$$VERSION \
		--build-arg REVISION=$$REVISION \
		--build-arg BRANCH=$$BRANCH \
		--build-arg BUILDUSER=$$BUILDUSER \
		--build-arg BUILDDATE=$$BUILDDATE \
		-t ${IMG} .

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
	VERSION=$$(cat VERSION) && \
	REVISION=$$(git rev-parse HEAD) && \
	BRANCH=$$(git rev-parse --abbrev-ref HEAD) && \
	BUILDUSER=$$(whoami)@$$HOSTNAME && \
	BUILDDATE=$$(date +%Y%m%d-%H:%M:%S) && \
	$(CONTAINER_TOOL) buildx build --push \
		--platform=$(PLATFORMS) \
		--build-arg VERSION=$$VERSION \
		--build-arg REVISION=$$REVISION \
		--build-arg BRANCH=$$BRANCH \
		--build-arg BUILDUSER=$$BUILDUSER \
		--build-arg BUILDDATE=$$BUILDDATE \
		--tag ${IMG} --tag ${IMG_MAIN} \
		-f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

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

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

# Add a bundle.yaml file with CRDs and deployment, with kustomize config.
## Generate a consolidated YAML with CRDs and deployment.
## Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests generate format kustomize operator-sdk
	@echo ">> operator-sdk version"
	$(OPERATOR_SDK) version
	@echo ">> generating bundle.yaml (override image using IMG_MAIN)"
	mkdir -p dist
	@if [ -d "config/crd" ]; then \
		$(KUSTOMIZE) build config/crd > bundle.yaml; \
	fi
	echo "---" >> bundle.yaml  # Add a document separator before appending
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_MAIN}
	$(KUSTOMIZE) build config/default >> bundle.yaml
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-check
bundle-check: bundle
	git diff --quiet --exit-code bundle config

.PHONY: bundle-build
bundle-build: generate bundle ## Build the bundle image.
	$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) image-push IMG=$(BUNDLE_IMG)

.PHONY: image-push
image-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool $(CONTAINER_TOOL)  --generate --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)
	docker buildx build --platform linux/amd64 \
      -t $(CATALOG_IMG) -f index.Dockerfile .
	rm -f index.Dockerfile

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) image-push IMG=$(CATALOG_IMG)

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
