# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export KUBERNETES_VERSION := v1.27.5

export CLUSTERCTL_VERSION := $(shell clusterctl version -o short 2>/dev/null)
export CAPA_VERSION := $(shell cd hack/third-party/capa && go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api-provider-aws/v2)
export CAPX_VERSION := $(shell cd hack/third-party/capx && go list -m -f '{{ .Version }}' github.com/nutanix-cloud-native/cluster-api-provider-nutanix)

.PHONY: examples.sync
examples.sync: ## Syncs the examples by fetching upstream examples and applying kustomize patches
	hack/examples/sync.sh
