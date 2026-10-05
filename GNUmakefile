# Pass all environment variables through. Optional: without it a bare
# `make testacc` fails on a missing file rather than on missing credentials.
-include .env
export

# Drive the acceptance tests with tofu rather than terraform. The test harness
# builds a provider reattach address under registry.terraform.io, which tofu
# rejects; the host and namespace overrides make it acceptable.
TOFU ?= tofu
TF_ACC_TERRAFORM_PATH ?= $(shell command -v $(TOFU))
TF_ACC_PROVIDER_HOST ?= registry.opentofu.org
TF_ACC_PROVIDER_NAMESPACE ?= hashicorp

default: testacc

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

# Regenerate docs/ from the provider schema and the configs in examples/
.PHONY: generate
generate:
	go generate ./...
