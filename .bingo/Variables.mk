# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.9. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
BINGO_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Below generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for crd-ref-docs variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(CRD_REF_DOCS)
#	@echo "Running crd-ref-docs"
#	@$(CRD_REF_DOCS) <flags/args..>
#
CRD_REF_DOCS := $(GOBIN)/crd-ref-docs-v0.1.0
$(CRD_REF_DOCS): $(BINGO_DIR)/crd-ref-docs.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/crd-ref-docs-v0.1.0"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=crd-ref-docs.mod -o=$(GOBIN)/crd-ref-docs-v0.1.0 "github.com/elastic/crd-ref-docs"

FAILLINT := $(GOBIN)/faillint-v1.13.0
$(FAILLINT): $(BINGO_DIR)/faillint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/faillint-v1.13.0"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=faillint.mod -o=$(GOBIN)/faillint-v1.13.0 "github.com/fatih/faillint"

GOIMPORTS := $(GOBIN)/goimports-v0.20.0
$(GOIMPORTS): $(BINGO_DIR)/goimports.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/goimports-v0.20.0"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=goimports.mod -o=$(GOBIN)/goimports-v0.20.0 "golang.org/x/tools/cmd/goimports"

GOLANGCI_LINT := $(GOBIN)/golangci-lint-v1.63.4
$(GOLANGCI_LINT): $(BINGO_DIR)/golangci-lint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/golangci-lint-v1.63.4"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=golangci-lint.mod -o=$(GOBIN)/golangci-lint-v1.63.4 "github.com/golangci/golangci-lint/cmd/golangci-lint"

MDOX := $(GOBIN)/mdox-v0.9.1-0.20220713110358-25b9abcf90a0
$(MDOX): $(BINGO_DIR)/mdox.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/mdox-v0.9.1-0.20220713110358-25b9abcf90a0"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=mdox.mod -o=$(GOBIN)/mdox-v0.9.1-0.20220713110358-25b9abcf90a0 "github.com/bwplotka/mdox"

