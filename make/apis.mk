# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

# We import sigs.k8s.io/cluster-api, which itself imports sigs.k8s.io/controller-runtime,
# as well as other modules.
#
# Third-party CAPI providers, e.g. CAPA, CAPZ, etc, also depend on these modules, but
# usually at different versions. For this reason, importing both sigs.k8s.io/cluster-api
# and a third-party provider usually causes dependency conflicts.
#
# To avoid conflicts, we do not import third-party provider types. Instead, we "sync,"
# i.e. copy, or vendor, the third-party provider types into the api/external directory.
#
# However, we do not sync the Kubeadm Control Plane or Kubeadm Bootstrap provider types,
# because they are developed in the sigs.k8s.io/cluster-api module, which we define as
# a dependency.
#
# We also do not sync the Docker infrastructure provider types, because they in the
# sigs.k8s.io/cluster-api/test module, which is developed in parallel with
# sigs.k8s.io/cluster-api. For that reason, we expect no dependency conflicts.

# Add third-party CAPI provider types below

PROVIDER_MODULE_capa := sigs.k8s.io/cluster-api-provider-aws/v2
PROVIDER_API_PATHS_capa := api/v1beta2 controlplane/eks/api/v1beta2 bootstrap/eks/api/v1beta2 iam/api/v1beta1

PROVIDER_MODULE_caaph := sigs.k8s.io/cluster-api-addon-provider-helm
PROVIDER_API_PATHS_caaph := api/v1alpha1

PROVIDER_MODULE_capx := github.com/nutanix-cloud-native/cluster-api-provider-nutanix
PROVIDER_API_PATHS_capx := api/v1beta1

# Add third-party CAPI provider types above

.PHONY: apis.sync
apis.sync: ## Syncs third-party CAPI providers' types
apis.sync: $(addprefix api.sync.,capa caaph capx) mod-tidy.api go-fix.api

.PHONY: api.sync.%
api.sync.%: ## Syncs a third-party CAPI provider's API types
api.sync.%: PROVIDER_MODULE_DIR=$(REPO_ROOT)/hack/third-party/$*
api.sync.%:
	cd $(PROVIDER_MODULE_DIR) && go mod tidy
	$(foreach PROVIDER_API_PATH,$(PROVIDER_API_PATHS_$*), \
	echo 'syncing external API: $(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH)'; \
	mkdir -p api/external/$(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH)/; \
	rsync \
	  --recursive --delete --times --links --verbose --prune-empty-dirs \
	  --exclude='*webhook*.go' \
	  --exclude='*test.go'     \
	  --exclude='s3bucket.go'  \
	  $$(cd $(PROVIDER_MODULE_DIR) && GOWORK=off go list -m -f '{{ .Dir }}' $(PROVIDER_MODULE_$*))/$(PROVIDER_API_PATH)/*.go \
	  api/external/$(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH)/; \
		find api/external/$(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH)/ -type d -exec chmod 0755 {} \; ; \
		find api/external/$(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH)/ -type f -exec chmod 0644 {} \; ; \
		sed -i 's|"$(PROVIDER_MODULE_$*)/|"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/$(PROVIDER_MODULE_$*)/|' api/external/$(PROVIDER_MODULE_$*)/$(PROVIDER_API_PATH)/*.go; \
	)

.PHONY: coredns.sync
coredns.sync: ## Syncs the Kubernetes version to CoreDNS version mapping used in the cluster upgrade
coredns.sync: ; $(info $(M) syncing CoreDNS version mapping)
	cd hack/tools/coredns-versions && go run . --output="$(REPO_ROOT)/api/versions/coredns.go"
