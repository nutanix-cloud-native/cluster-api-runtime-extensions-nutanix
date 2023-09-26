# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export KUBERNETES_VERSION := v1.27.5

export CLUSTERCTL_VERSION := $(shell clusterctl version -o short 2>/dev/null)

.PHONY: examples.sync
examples.sync: ## Syncs the examples by fetching upstream examples and applying kustomize patches
	hack/examples/sync.sh
