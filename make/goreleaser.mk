# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

GORELEASER_PARALLELISM ?= $(shell nproc --ignore=1)
GORELEASER_DEBUG ?= false

ifndef GORELEASER_CURRENT_TAG
export GORELEASER_CURRENT_TAG=$(GIT_TAG)
endif

.PHONY: build-snapshot
build-snapshot: ## Builds a snapshot with goreleaser
build-snapshot: go-generate ; $(info $(M) building snapshot $*)
	goreleaser --debug=$(GORELEASER_DEBUG) \
		build \
		--snapshot \
		--clean \
		--parallelism=$(GORELEASER_PARALLELISM) \
		$(if $(BUILD_ALL),,--single-target)

.PHONY: release
release: ## Builds a release with goreleaser
release: go-generate ; $(info $(M) building release $*)
	goreleaser --debug=$(GORELEASER_DEBUG) \
		release \
		--clean \
		--parallelism=$(GORELEASER_PARALLELISM) \
		--timeout=60m \
		$(GORELEASER_FLAGS)

.PHONY: release-snapshot
release-snapshot: ## Builds a snapshot release with goreleaser
release-snapshot: go-generate ; $(info $(M) building snapshot release $*)
	goreleaser --debug=$(GORELEASER_DEBUG) \
		release \
		--snapshot \
		--clean \
		--parallelism=$(GORELEASER_PARALLELISM) \
		--timeout=60m
