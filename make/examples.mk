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
examples.sync: export KUBECONFIG := $(KIND_KUBECONFIG)
examples.sync: kind.create clusterctl.init
	mkdir -p examples/capi-quickstart
	# Sync ClusterClass and all Templates
	clusterctl generate cluster capi-quickstart \
      --flavor development \
      --kubernetes-version $(KUBERNETES_VERSION) \
      --control-plane-machine-count=1 \
      --worker-machine-count=1 | \
    gojq --yaml-input --yaml-output \
      '. | (select(.kind=="ClusterClass").spec.patches|= .+ [{"name": "cluster-config", "external": {"generateExtension": "clusterconfigpatch.capi-runtime-extensions", "discoverVariablesExtension": "clusterconfigvars.capi-runtime-extensions"}}])' | \
    gojq --yaml-input --yaml-output \
      '. | select(.kind != "Cluster")' | \
    gojq --yaml-input --yaml-output '.' > examples/capi-quickstart/cluster-class.yaml
	# Sync Cluster
	clusterctl generate cluster capi-quickstart \
      --flavor development \
      --kubernetes-version $(KUBERNETES_VERSION) \
      --control-plane-machine-count=1 \
      --worker-machine-count=1 | \
    gojq --yaml-input --yaml-output \
      '. | (select(.kind=="Cluster").metadata.labels["capiext.labs.d2iq.io/cni"]|="calico")' | \
    gojq --yaml-input --yaml-output \
    	'. | (select(.kind=="Cluster").spec.topology.variables|= .+ [{"name": "clusterConfig", "value": {}}])' | \
    gojq --yaml-input --yaml-output \
      '. | select(.kind == "Cluster")' | \
    gojq --yaml-input --yaml-output '.' > examples/capi-quickstart/cluster.yaml
