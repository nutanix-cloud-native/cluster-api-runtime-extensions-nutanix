# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

KIND_DIR := $(REPO_ROOT)/.local/kind

KIND_CLUSTER_NAME ?= $(GITHUB_REPOSITORY)-dev
KIND_KUBECONFIG ?= $(KIND_DIR)/$(KIND_CLUSTER_NAME)/kubeconfig

KINDEST_NODE_IMAGE ?= ghcr.io/mesosphere/kind-node
KINDEST_NODE_VERSION_v1.30 ?= v1.30.10
KINDEST_NODE_VERSION_v1.31 ?= v1.31.6
KINDEST_NODE_VERSION_v1.32 ?= v1.32.2
# Allow easy override of Kubernetes version to use via `make KIND_KUBERNETES_VERSION=v1.23` to use in CI
KIND_KUBERNETES_VERSION ?= v1.32
ifndef KINDEST_NODE_VERSION_$(KIND_KUBERNETES_VERSION)
  $(error Unsupported Kind Kubernetes version: $(KIND_KUBERNETES_VERSION) (use on of: [$(patsubst KINDEST_NODE_VERSION_%,%,$(filter KINDEST_NODE_VERSION_%,$(.VARIABLES)))]))
endif

export KINDEST_IMAGE_TAG ?= $(KINDEST_NODE_VERSION_$(KIND_KUBERNETES_VERSION))
KINDEST_IMAGE = $(KINDEST_NODE_IMAGE):$(KINDEST_IMAGE_TAG)

.PHONY: kind.recreate
kind.recreate: ## Re-creates new KinD cluster if necessary
kind.recreate: kind.delete kind.create

.PHONY: kind.create
kind.create: ## Creates new KinD cluster
kind.create: ; $(info $(M) creating kind cluster - $(KIND_CLUSTER_NAME))
	(kind get clusters 2>/dev/null | grep -Eq '^$(KIND_CLUSTER_NAME)$$' && echo '$(KIND_CLUSTER_NAME) already exists') || \
		env KUBECONFIG=$(KIND_KUBECONFIG) $(REPO_ROOT)/hack/kind/create-cluster.sh \
		  --cluster-name $(KIND_CLUSTER_NAME) \
		  --kindest-image $(KINDEST_IMAGE) \
		  --output-dir $(KIND_DIR)/$(KIND_CLUSTER_NAME) \
		  --base-config $(REPO_ROOT)/hack/kind/kind-base-config.yaml

.PHONY: kind.delete
kind.delete: ## Deletes KinD cluster
kind.delete: ; $(info $(M) deleting kind cluster - $(KIND_CLUSTER_NAME))
	(kind get clusters 2>/dev/null | grep -Eq '^$(KIND_CLUSTER_NAME)$$' && kind delete cluster --name $(KIND_CLUSTER_NAME)) || \
	  echo '$(KIND_CLUSTER_NAME) does not exist'
	rm -rf $(KIND_DIR)/$(KIND_CLUSTER_NAME)

.PHONY: kind.kubeconfig
kind.kubeconfig: ## Prints export definition for kubeconfig
	echo "export KUBECONFIG=$(KIND_KUBECONFIG)"
