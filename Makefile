MAKEFLAGS := -j2
SHELL := /bin/bash

PACK             := clerk
PACKDIR          := sdk
PROJECT          := github.com/pulumi/pulumi-clerk
NODE_MODULE_NAME := @pulumi/clerk
TF_NAME          := clerk
PROVIDER_PATH    := provider
PROVIDER_VERSION ?= $(shell pulumictl get version --language generic 2>/dev/null || echo "0.0.1-alpha.0")

TESTPARALLELISM := 4
WORKING_DIR     := $(shell pwd)

generate: generate_schema generate_sdks

generate_schema:
	cd $(PROVIDER_PATH) && go run ./cmd/pulumi-tfgen-$(PACK) --out ../schema.json

generate_sdks: generate_schema
	pulumi package gen-sdk --language nodejs --out $(PACKDIR)/nodejs schema.json
	pulumi package gen-sdk --language python --out $(PACKDIR)/python schema.json
	pulumi package gen-sdk --language go --out $(PACKDIR)/go schema.json
	pulumi package gen-sdk --language dotnet --out $(PACKDIR)/dotnet schema.json
	pulumi package gen-sdk --language java --out $(PACKDIR)/java schema.json

build:: build_provider build_tfgen

build_provider:
	cd $(PROVIDER_PATH) && go build -o $(WORKING_DIR)/bin/pulumi-resource-$(PACK) ./cmd/pulumi-resource-$(PACK)

build_tfgen:
	cd $(PROVIDER_PATH) && go build -o $(WORKING_DIR)/bin/pulumi-tfgen-$(PACK) ./cmd/pulumi-tfgen-$(PACK)

install_provider: build_provider
	mkdir -p $$HOME/.pulumi/plugins/resource-$(PACK)-$(PROVIDER_VERSION)
	cp $(WORKING_DIR)/bin/pulumi-resource-$(PACK) $$HOME/.pulumi/plugins/resource-$(PACK)-$(PROVIDER_VERSION)/pulumi-resource-$(PACK)

tidy:
	cd $(PROVIDER_PATH)/shim && go mod tidy
	cd $(PROVIDER_PATH) && go mod tidy

test:
	cd $(PROVIDER_PATH) && go test ./... -v -count=1 -timeout 2h -parallel $(TESTPARALLELISM)

.PHONY: generate generate_schema generate_sdks build build_provider build_tfgen install_provider tidy test
