# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CAPI_VERSION := $(shell GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api)
export CAPD_VERSION := $(shell GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api/test)
export CAPA_VERSION := $(shell cd hack/third-party/capa && GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api-provider-aws/v2)
export CAPX_VERSION := $(shell cd hack/third-party/capx && GOWORK=off go list -m -f '{{ .Version }}' github.com/nutanix-cloud-native/cluster-api-provider-nutanix)
export CAAPH_VERSION := $(shell cd hack/third-party/caaph && GOWORK=off go list -m -f '{{ .Version }}' sigs.k8s.io/cluster-api-addon-provider-helm)

# Leave Nutanix credentials empty here and set it when creating the clusters
.PHONY: clusterctl.init
clusterctl.init:
	env CLUSTER_TOPOLOGY=true \
	    EXP_RUNTIME_SDK=true \
	    EXP_CLUSTER_RESOURCE_SET=true \
	    EXP_MACHINE_POOL=true \
	    AWS_B64ENCODED_CREDENTIALS=$$(clusterctl-aws bootstrap credentials encode-as-profile) \
	    NUTANIX_ENDPOINT="" NUTANIX_PASSWORD="" NUTANIX_USER="" \
	    clusterctl init \
	      --kubeconfig=$(KIND_KUBECONFIG) \
	      --core cluster-api:$(CAPI_VERSION) \
	      --bootstrap kubeadm:$(CAPI_VERSION) \
	      --control-plane kubeadm:$(CAPI_VERSION) \
	      --infrastructure docker:$(CAPD_VERSION),aws:$(CAPA_VERSION),nutanix:$(CAPX_VERSION) \
	      --addon helm:$(CAAPH_VERSION) \
	      --wait-providers

.PHONY: clusterctl.delete
clusterctl.delete:
	clusterctl delete --kubeconfig=$(KIND_KUBECONFIG) --all
