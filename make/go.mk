# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# The GOPRIVATE environment variable controls which modules the go command considers
# to be private (not available publicly) and should therefore not use the proxy or checksum database
export GOPRIVATE ?=

# Always ensure that GOOS and GOARCH are unset in the evironment, otherwise this can cause issues
# with goreleaser and ko building images for the wrong platform.
override undefine GOOS
override undefine GOARCH

ALL_GO_SUBMODULES := $(shell find -mindepth 2 -maxdepth 2 -name go.mod -printf '%P\n' | sort)
GO_SUBMODULES_NO_DOCS := $(filter-out $(addsuffix /go.mod,docs),$(ALL_GO_SUBMODULES))
THIRD_PARTY_GO_SUBMODULES := $(shell find hack/third-party -mindepth 2 -name go.mod -printf 'hack/third-party/%P\n' | sort)

define go_test
	source <(setup-envtest use -p env $(ENVTEST_VERSION)) && \
	gotestsum \
		--jsonfile test.json \
		--junitfile junit-report.xml \
		--junitfile-testsuite-name=relative \
		--junitfile-testcase-classname=short \
		-- \
		-covermode=atomic \
		-coverprofile=coverage.out \
		-short \
		-race \
		-v \
		$(if $(GOTEST_RUN),-run "$(GOTEST_RUN)") \
		./... && \
	go tool cover \
		-html=coverage.out \
		-o coverage.html
endef

.PHONY: test
test: ## Runs go tests for all modules in repository
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
test: go-generate test.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
test: go-generate $(addprefix test.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif

.PHONY: test.%
test.%: ## Runs go tests for a specific module
test.%: go-generate ; $(info $(M) running tests$(if $(GOTEST_RUN), matching "$(GOTEST_RUN)") for $* module)
	$(if $(filter-out root,$*),cd $* && )$(call go_test)

.PHONY: integration-test
integration-test: ## Runs integration tests for all modules in repository
integration-test: GOTEST_RUN := Integration
integration-test: test

.PHONY: integration-test.%
integration-test.%: ## Runs integration tests for a specific module
integration-test.%: GOTEST_RUN := Integration
integration-test.%: test.$*

.PHONY: bench
bench: ## Runs go benchmarks for all modules in repository
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
bench: bench.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
bench: $(addprefix bench.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif

.PHONY: bench.%
bench.%: ## Runs go benchmarks for a specific module
bench.%: ; $(info $(M) running benchmarks$(if $(GOTEST_RUN), matching "$(GOTEST_RUN)") for $* module)
	$(if $(filter-out root,$*),cd $* && )go test $(if $(GOTEST_RUN),-run "$(GOTEST_RUN)") -race -cover -v ./...

E2E_DRYRUN ?= false
E2E_VERBOSE ?= $(filter $(E2E_DRYRUN),true) # If dry-run, enable verbosity
E2E_PARALLEL_NODES ?= $(if $(filter $(E2E_DRYRUN),true),1,$(shell nproc --ignore=1)) # Ginkgo cannot dry-run in parallel
E2E_FLAKE_ATTEMPTS ?= 1
E2E_CONF_FILE ?= $(REPO_ROOT)/test/e2e/config/caren.yaml
E2E_CONF_FILE_ENVSUBST ?= $(basename $(E2E_CONF_FILE))-envsubst.yaml
export E2E_KUBERNETES_VERSION ?= $(KINDEST_IMAGE_TAG)
ARTIFACTS ?= ${REPO_ROOT}/_artifacts

.PHONY: e2e-test
e2e-test: ## Runs e2e tests
ifneq ($(wildcard test/e2e/*),)
e2e-test: examples.sync
ifneq ($(SKIP_BUILD),true)
	$(MAKE) GORELEASER_FLAGS=$$'--config=<(env GOOS=$(shell go env GOOS) GOARCH=$(shell go env GOARCH) gojq --yaml-input --yaml-output \'del(.builds[0].goarch) | del(.builds[0].goos) | .builds[0].targets|=(["linux_amd64","linux_arm64",env.GOOS+"_"+env.GOARCH] | unique | map(. | sub("_amd64";"_amd64_v1")))\' .goreleaser.yml)' release-snapshot
endif
	$(info $(M) $(if $(filter $(E2E_DRYRUN), true),dry-,)running e2e tests$(if $(E2E_LABEL), labelled "$(E2E_LABEL)")$(if $(E2E_FOCUS), matching "$(E2E_FOCUS)"))
	env E2E_IMAGE_TAG="$$(gojq --raw-output '.version+"-"+.runtime.goarch' $(REPO_ROOT)/dist/metadata.json)" \
	  envsubst -no-unset -no-empty -i '$(E2E_CONF_FILE)' -o '$(E2E_CONF_FILE_ENVSUBST)'
	env AWS_B64ENCODED_CREDENTIALS="$$(clusterctl-aws bootstrap credentials encode-as-profile 2>/dev/null)" \
	  ginkgo run \
	    --r \
	    --show-node-events \
	    --trace \
	    --randomize-all \
	    --randomize-suites \
	    --fail-on-pending \
	    --keep-going \
	    $(if $(filter $(E2E_VERBOSE),true),--vv) \
	    --covermode=atomic \
	    --coverprofile coverage-e2e.out \
	    $(if $(filter $(E2E_DRYRUN), true),--dry-run) \
	    --procs=$(E2E_PARALLEL_NODES) \
	    --compilers=$(E2E_PARALLEL_NODES) \
	    --flake-attempts=$(E2E_FLAKE_ATTEMPTS) \
	    $(if $(E2E_FOCUS),--focus="$(E2E_FOCUS)") \
	    $(if $(E2E_SKIP),--skip="$(E2E_SKIP)") \
	    $(if $(E2E_LABEL),--label-filter="$(E2E_LABEL)") \
	    $(E2E_GINKGO_FLAGS) \
	    --junit-report=junit-e2e.xml \
	    --json-report=report-e2e.json \
	    --tags e2e \
	    test/e2e/... -- \
	      -e2e.artifacts-folder="$(ARTIFACTS)" \
	      -e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
	      $(if $(filter $(E2E_SKIP_CLEANUP),true),-e2e.skip-resource-cleanup) \
	      -e2e.bootstrap-kind-version="$(KINDEST_IMAGE_TAG)"
	go tool cover \
	  -html=coverage-e2e.out \
	  -o coverage-e2e.html
endif

GOLANGCI_CONFIG_FILE ?= $(wildcard $(REPO_ROOT)/.golangci.y*ml)

.PHONY: lint
lint: ## Runs golangci-lint for all modules in repository
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
lint: lint.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
lint: $(addprefix lint.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif

.PHONY: lint.%
lint.%: ## Runs golangci-lint for a specific module
lint.%: ; $(info $(M) linting $* module)
	$(if $(filter-out root,$*),cd $* && )golines -w $$(GOWORK=off go list -tags e2e ./... | grep -v external | sed "s|^$$(GOWORK=off go list -m)|.|")
	$(if $(filter-out root,$*),cd $* && )golangci-lint run --fix --config=$(GOLANGCI_CONFIG_FILE)
	$(if $(filter-out root,$*),cd $* && )golines -w $$(GOWORK=off go list -tags e2e ./... | grep -v external | sed "s|^$$(GOWORK=off go list -m)|.|")

.PHONY: mod-tidy
mod-tidy: ## Run go mod tidy for all modules
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
mod-tidy: mod-tidy.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
mod-tidy: $(addprefix mod-tidy.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif
ifneq ($(words $(THIRD_PARTY_GO_SUBMODULES)),0)
mod-tidy: $(addprefix mod-tidy.,$(THIRD_PARTY_GO_SUBMODULES:/go.mod=))
endif

.PHONY: mod-tidy.%
.PHONY: mod-tidy.hack/third-party/%
mod-tidy.% mod-tidy.hack/third-party/%: ## Runs go mod tidy for a specific module
mod-tidy.% mod-tidy.hack/third-party/%: ; $(info $(M) running go mod tidy for $* module)
	$(if $(filter-out root,$*),cd $(@:mod-tidy.%=%) && )go mod tidy -v
	$(if $(filter-out root,$*),cd $(@:mod-tidy.%=%) && )go mod verify

.PHONY: go-clean
go-clean: ## Cleans go build, test and modules caches for all modules
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
go-clean: go-clean.root
endif
ifneq ($(words $(ALL_GO_SUBMODULES)),0)
go-clean: $(addprefix go-clean.,$(ALL_GO_SUBMODULES:/go.mod=))
endif

.PHONY: go-clean.%
go-clean.%: ## Cleans go build, test and modules caches for a specific module
go-clean.%: ; $(info $(M) running go clean for $* module)
	$(if $(filter-out root,$*),cd $* && )go clean -r -i -cache -testcache -modcache

.PHONY: go-fix
go-fix: ## Runs go fix for all modules in repository
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
go-fix: go-fix.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
go-fix: $(addprefix go-fix.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif

.PHONY: go-fix.%
go-fix.%: ## Runs golangci-lint for a specific module
go-fix.%: ; $(info $(M) go fixing $* module)
	$(if $(filter-out root,$*),cd $* && )go fix ./...

.PHONY: go-generate
go-generate: ## Runs go generate
go-generate: ; $(info $(M) running go generate)
	go generate -x ./...
	controller-gen paths="./..." \
		rbac:headerFile="hack/license-header.yaml.txt",roleName=cluster-api-runtime-extensions-nutanix-manager-role \
		output:rbac:artifacts:config=charts/cluster-api-runtime-extensions-nutanix/templates
	controller-gen paths="./api/v1alpha1" \
	  object:headerFile="hack/license-header.go.txt" output:object:artifacts:config=/dev/null \
	  crd:headerFile=hack/license-header.yaml.txt output:crd:artifacts:config=./api/v1alpha1/crds
	controller-gen paths="./..." \
	  webhook:headerFile="hack/license-header.yaml.txt" \
	  output:webhook:stdout >charts/cluster-api-runtime-extensions-nutanix/templates/webhooks.yaml
	# Update the webhook names and configure cert-manager CA injection.
	yq --inplace \
	  '.metadata.name = "{{ include \"chart.name\" . }}-manager-role"' \
	  charts/cluster-api-runtime-extensions-nutanix/templates/role.yaml
	# Update the CA injection annotation.
	yq --inplace \
	  --from-file=hack/update-webhook-configurations.yq \
	  charts/cluster-api-runtime-extensions-nutanix/templates/webhooks.yaml
	#$(MAKE) go-fix
	$(MAKE) configure-csi-providers
	# Update anyOf schemas for resource.Quantity fields to only accept strings
	# until CAPI ClusterClass variables support anyOf schemas.
	find api/v1alpha1/crds/ -name 'caren.nutanix.com_nutanix*configs.yaml' \
	  -exec yq --inplace \
	    '(.. | select(has("memorySize") or has("systemDiskSize")) | (.memorySize?, .systemDiskSize?) | del(.anyOf)) += {"type": "string"}' \
	    {} \;

.PHONY: go-mod-upgrade
go-mod-upgrade: ## Interactive check for direct module dependency upgrades
go-mod-upgrade: ; $(info $(M) checking for direct module dependency upgrades)
	go-mod-upgrade

.PHONY: govulncheck
govulncheck: ## Runs go fix for all modules in repository
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
govulncheck: govulncheck.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
govulncheck: $(addprefix govulncheck.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif

.PHONY: govulncheck.%
govulncheck.%: ## Runs golangci-lint for a specific module
govulncheck.%: ; $(info $(M) running govulncheck on $* module)
	$(if $(filter-out root .,$*),cd $* && )govulncheck ./...

.PHONY: go-mod-edit-toolchain
go-mod-edit-toolchain: ## Edits the go.mod file of all modules in repository to use the toolchain version
ifneq ($(wildcard $(REPO_ROOT)/go.mod),)
go-mod-edit-toolchain: go-mod-edit-toolchain.root
endif
ifneq ($(words $(GO_SUBMODULES_NO_DOCS)),0)
go-mod-edit-toolchain: $(addprefix go-mod-edit-toolchain.,$(GO_SUBMODULES_NO_DOCS:/go.mod=))
endif

.PHONY: go-mod-edit-toolchain.%
go-mod-edit-toolchain.%: ## Edits the go.mod file of a specifc module in repository to use the toolchain version
go-mod-edit-toolchain.%: ; $(info $(M) setting go toolchain for $* module)
ifndef GO_TOOLCHAIN_VERSION
	$(error GO_TOOLCHAIN_VERSION is not set: please set GO_TOOLCHAIN_VERSION to the desired version, e.g. go1.22.5)
endif
	$(if $(filter-out root .,$*),cd $* && )go mod edit -toolchain=$(GO_TOOLCHAIN_VERSION)
