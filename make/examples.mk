# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

KUBERNETES_VERSION ?= v1.27.5

# gojq mutations:
# - ClusterClass: 	Add cluster-config external patch
#
# - Cluster: 		Add CNI label
# - Cluster: 		Add an empty clusterConfig variable
.PHONY: examples.sync
examples.sync: ## Syncs the examples by fetching upstream examples using clusterclt and applying gojq mutations
	mkdir -p examples/capi-quickstart
	# Sync ClusterClass and all Templates
	kustomize build ./hack/examples | \
	  gojq --yaml-input --yaml-output '. | select(.kind != "Cluster")' > examples/capi-quickstart/capd-cluster-class.yaml
	# Sync Cluster
	kustomize build ./hack/examples | \
	  gojq --yaml-input --yaml-output '. | select(.kind == "Cluster")' > examples/capi-quickstart/capd-cluster.yaml
