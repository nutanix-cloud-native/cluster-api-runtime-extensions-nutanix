# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: clusterctl.init
clusterctl.init: install-tool.clusterctl
	env CLUSTER_TOPOLOGY=true \
			EXP_RUNTIME_SDK=true \
			EXP_CLUSTER_RESOURCE_SET=true \
			clusterctl init \
		--kubeconfig=$(KIND_KUBECONFIG) \
		--infrastructure docker \
		--wait-providers

.PHONY: clusterctl.delete
clusterctl.delete: install-tool.clusterctl
	clusterctl delete --kubeconfig=$(KIND_KUBECONFIG) --all
