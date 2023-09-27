# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export KUBERNETES_VERSION := v1.27.5

export CLUSTERCTL_VERSION := $(shell clusterctl version -o short 2>/dev/null)
export CAPA_VERSION := $(shell grep -E -e "sigs.k8s.io/cluster-api-provider-aws/v2" external/cluster-api-provider-aws/v2/go.mod | cut -d' ' -f3)

.PHONY: examples.sync
examples.sync: ## Syncs the examples by fetching upstream examples and applying kustomize patches
	hack/examples/sync.sh
