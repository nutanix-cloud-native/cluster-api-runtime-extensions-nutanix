# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

ifeq ($(strip $(REPO_ROOT)),)
REPO_ROOT := $(shell git rev-parse --show-toplevel)
endif

GIT_COMMIT := $(shell git rev-parse "HEAD^{commit}")
export GIT_TAG ?= $(shell git describe --tags "$(GIT_COMMIT)^{commit}" --match v* --abbrev=0 2>/dev/null)
export GIT_CURRENT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

export GITHUB_ORG := $(shell gh repo view --jq '.owner.login' --json owner)
export GITHUB_REPOSITORY := $(shell gh repo view --jq '.name' --json name)

ifneq ($(shell git status --porcelain 2>/dev/null; echo $$?), 0)
	export GIT_TREE_STATE := dirty
else
	export GIT_TREE_STATE :=
endif
