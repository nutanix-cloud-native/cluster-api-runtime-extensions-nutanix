# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

.PHONY: dev.run-on-kind
dev.run-on-kind: kind.create clusterctl.init install-tool.helm install-tool.gojq
ifndef SKIP_BUILD
	$(MAKE) release-snapshot
endif
	kind load docker-image --name $(KIND_CLUSTER_NAME) \
		$$(gojq -r '.[] | select(.type=="Docker Image") | select(.goarch=="$(GOARCH)") | .name' dist/artifacts.json)
	helm upgrade --install --kubeconfig $(KIND_KUBECONFIG) capi-runtime-extensions ./charts/capi-runtime-extensions \
		--set-string image.tag=$$(gojq -r .version dist/metadata.json) \
		--wait --wait-for-jobs
	kubectl --kubeconfig $(KIND_KUBECONFIG) rollout restart deployment capi-runtime-extensions
	kubectl --kubeconfig $(KIND_KUBECONFIG) rollout status deployment capi-runtime-extensions
