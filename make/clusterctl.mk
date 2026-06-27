# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CAPI_VERSION := $(shell GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api)
export CAPD_VERSION := $(shell GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api/test)
export CAPA_VERSION := $(shell cd hack/third-party/capa && GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api-provider-aws/v2)-ncn.1
export CAPX_VERSION := $(shell cd hack/third-party/capx && GOWORK=off go list -m -f '{{ .Version }}' github.com/nutanix-cloud-native/cluster-api-provider-nutanix)
export CAAPH_VERSION := $(shell cd hack/third-party/caaph && GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api-addon-provider-helm)

# Copy local CAPX metadata.yaml override so clusterctl accepts v1.10+ series,
# which is missing from the public CAPX release's metadata.yaml.
.PHONY: clusterctl.setup-capx-override
clusterctl.setup-capx-override:
	mkdir -p $(HOME)/.config/cluster-api/overrides/infrastructure-nutanix/$(CAPX_VERSION)
	cp test/e2e/data/shared/capx/metadata.yaml \
	   $(HOME)/.config/cluster-api/overrides/infrastructure-nutanix/$(CAPX_VERSION)/metadata.yaml

# Leave Nutanix credentials empty here and set it when creating the clusters
.PHONY: clusterctl.init
clusterctl.init: clusterctl.setup-capx-override
	env CLUSTER_TOPOLOGY=true \
	    EXP_RUNTIME_SDK=true \
	    EXP_MACHINE_POOL=true \
	    CAPA_EKS=true \
	    AWS_B64ENCODED_CREDENTIALS=$$(clusterctl-aws bootstrap credentials encode-as-profile) \
	    NUTANIX_ENDPOINT="" NUTANIX_PASSWORD="" NUTANIX_USER="" \
	    clusterctl init -v=10 \
	      --kubeconfig=$(KIND_KUBECONFIG) \
	      --core cluster-api:$(CAPI_VERSION) \
	      --bootstrap kubeadm:$(CAPI_VERSION) \
	      --control-plane kubeadm:$(CAPI_VERSION) \
	      --infrastructure docker:$(CAPD_VERSION),aws:$(CAPA_VERSION),nutanix:$(CAPX_VERSION) \
	      --addon helm:$(CAAPH_VERSION) \
	      --config clusterctl.yaml \
	      --wait-providers

.PHONY: clusterctl.delete
clusterctl.delete:
	clusterctl delete --kubeconfig=$(KIND_KUBECONFIG) --all

.PHONY: capa.update-credentials-secret
capa.update-credentials-secret:
	kubectl patch secret capa-manager-bootstrap-credentials -n capa-system -p="{\"data\":{\"credentials\": \"$$(clusterctl-aws bootstrap credentials encode-as-profile)\"}}"
	kubectl rollout restart deployment capa-controller-manager -n capa-system
	kubectl rollout status deployment capa-controller-manager -n capa-system
