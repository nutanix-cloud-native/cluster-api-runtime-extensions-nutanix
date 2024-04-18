# Copyright 2023 D2iQ, Inc. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

ADDONS_PROVIDER := ClusterResourceSet

.PHONY: dev.run-on-kind
dev.run-on-kind: export KUBECONFIG := $(KIND_KUBECONFIG)
dev.run-on-kind: kind.create clusterctl.init
ifndef SKIP_BUILD
	$(MAKE) release-snapshot
endif
	kind load docker-image --name $(KIND_CLUSTER_NAME) \
		ko.local/cluster-api-runtime-extensions-nutanix:$$(gojq -r .version dist/metadata.json)
	helm upgrade --install cluster-api-runtime-extensions-nutanix ./charts/cluster-api-runtime-extensions-nutanix \
		--set-string image.repository=ko.local/cluster-api-runtime-extensions-nutanix \
		--set-string image.tag=$$(gojq -r .version dist/metadata.json) \
		--wait --wait-for-jobs
	kubectl rollout restart deployment cluster-api-runtime-extensions-nutanix
	kubectl rollout status deployment cluster-api-runtime-extensions-nutanix

.PHONY: dev.update-webhook-image-on-kind
dev.update-webhook-image-on-kind: export KUBECONFIG := $(KIND_KUBECONFIG)
dev.update-webhook-image-on-kind:
ifndef SKIP_BUILD
	$(MAKE) release-snapshot
endif
	kind load docker-image --name $(KIND_CLUSTER_NAME) \
		ko.local/cluster-api-runtime-extensions-nutanix:$$(gojq -r .version dist/metadata.json)
	kubectl set image deployment cluster-api-runtime-extensions-nutanix webhook=ko.local/cluster-api-runtime-extensions-nutanix:$$(gojq -r .version dist/metadata.json)
	kubectl rollout restart deployment cluster-api-runtime-extensions-nutanix
	kubectl rollout status deployment cluster-api-runtime-extensions-nutanix


.PHONY: dev.update-bootstrap-credentials-aws
dev.update-bootstrap-credentials-aws: export KUBECONFIG := $(KIND_KUBECONFIG)
dev.update-bootstrap-credentials-aws:
	kubectl patch secret capa-manager-bootstrap-credentials -n capa-system -p="{\"data\":{\"credentials\": \"$$(clusterawsadm bootstrap credentials encode-as-profile)\"}}"
	kubectl rollout restart deployment capa-controller-manager -n capa-system
	kubectl rollout status deployment capa-controller-manager -n capa-system

.PHONY: release-please
release-please:
ifneq ($(GIT_CURRENT_BRANCH),main)
	$(error "release-please should only be run on the main branch")
else
	release-please release-pr \
	  --repo-url $(GITHUB_ORG)/$(GITHUB_REPOSITORY) --token "$$(gh auth token)"
endif

.PHONY: .envrc.e2e
.envrc.e2e:
	gojq --yaml-input --raw-output '.variables | to_entries | map("\(.key)=\(.value|tostring)")|.[]' < test/e2e/config/caren.yaml | envsubst > .envrc.e2e
	direnv reload
