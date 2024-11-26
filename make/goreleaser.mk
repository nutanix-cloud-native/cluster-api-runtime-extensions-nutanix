# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

GORELEASER_PARALLELISM ?= $(shell nproc --ignore=1)
GORELEASER_VERBOSE ?= false

ifndef GORELEASER_CURRENT_TAG
export GORELEASER_CURRENT_TAG=$(GIT_TAG)
endif

.PHONY: build-snapshot
build-snapshot: ## Builds a snapshot with goreleaser
build-snapshot: go-generate ; $(info $(M) building snapshot $*)
	goreleaser --verbose=$(GORELEASER_VERBOSE) \
		build \
		--snapshot \
		--clean \
		--parallelism=$(GORELEASER_PARALLELISM) \
		--config=<(env GOOS=$(shell go env GOOS) gojq --yaml-input --yaml-output '.builds[0].goos |= (. + [env.GOOS] | unique)' .goreleaser.yml) \
		$(if $(BUILD_ALL),,--single-target)

.PHONY: release
release: ## Builds a release with goreleaser
release: go-generate ; $(info $(M) building release $*)
	goreleaser --verbose=$(GORELEASER_VERBOSE) \
		release \
		--clean \
		--parallelism=$(GORELEASER_PARALLELISM) \
		--timeout=60m \
		$(GORELEASER_FLAGS)

.PHONY: release-snapshot
release-snapshot: ## Builds a snapshot release with goreleaser
release-snapshot: go-generate ; $(info $(M) building snapshot release $*)
	goreleaser --verbose=$(GORELEASER_VERBOSE) \
		release \
		--snapshot \
		--clean \
		--parallelism=$(GORELEASER_PARALLELISM) \
		--timeout=60m

.PHONY: list-releases
list-releases: ## List releases from GitHub
	gh release list --json tagName | gojq -r .[].tagName

.PHONY: add-version-to-clusterclasses
add-version-to-clusterclasses:
	./hack/examples/release/add-version-to-clusterclasses.sh $(CAREN_VERSION)
