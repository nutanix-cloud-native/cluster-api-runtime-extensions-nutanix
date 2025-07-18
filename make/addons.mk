# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.29.4
export CILIUM_VERSION := 1.17.4
export NODE_FEATURE_DISCOVERY_VERSION := 0.17.3
export CLUSTER_AUTOSCALER_CHART_VERSION := 9.46.6
export AWS_EBS_CSI_CHART_VERSION := 2.44.0
export NUTANIX_STORAGE_CSI_CHART_VERSION := 3.3.4
export LOCAL_PATH_CSI_CHART_VERSION := 0.0.31
export SNAPSHOT_CONTROLLER_CHART_VERSION := 4.0.2
# AWS CCM uses the same chart version for all kubernetes versions. The image used in the deployment will
# be updated by the addon kustomization for CRS deployments and via Helm values for HelmAddon deployments.
export AWS_CCM_CHART_VERSION := 0.0.8
# A map of AWS CCM versions.
export AWS_CCM_VERSION_130 := v1.30.8
export AWS_CCM_VERSION_131 := v1.31.5
export AWS_CCM_VERSION_132 := v1.32.1
export AWS_CCM_VERSION_133 := v1.33.0

export NUTANIX_CCM_CHART_VERSION := 0.5.2

export KUBE_VIP_VERSION := v0.9.1

export METALLB_CHART_VERSION := 0.15.2

export COSI_CONTROLLER_VERSION := 0.0.1-alpha.5

.PHONY: addons.sync
addons.sync: $(addprefix update-addon.,calico cilium nfd cluster-autoscaler snapshot-controller local-path-provisioner-csi aws-ebs-csi kube-vip)
addons.sync: $(addprefix update-addon.aws-ccm.,130 131 132 133)
addons.sync: template-helm-repository

.PHONY: update-addon.calico
update-addon.calico: ; $(info $(M) updating calico manifests)
	./hack/addons/update-calico-manifests.sh

.PHONY: update-addon.cilium
update-addon.cilium: ; $(info $(M) updating cilium manifests)
	./hack/addons/update-cilium-manifests.sh

.PHONY: update-addon.nfd
update-addon.nfd: ; $(info $(M) updating node feature discovery manifests)
	./hack/addons/update-node-feature-discovery-manifests.sh

.PHONY: update-addon.cluster-autoscaler
update-addon.cluster-autoscaler: ; $(info $(M) updating cluster-autoscaler manifests)
	./hack/addons/update-cluster-autoscaler.sh

.PHONY: update-addon.aws-ebs-csi
update-addon.aws-ebs-csi: ; $(info $(M) updating aws ebs csi manifests)
	./hack/addons/update-aws-ebs-csi.sh

.PHONY: update-addon.local-path-provisioner-csi
update-addon.local-path-provisioner-csi: ; $(info $(M) updating local-path-provisioner csi manifests)
	./hack/addons/update-local-path-provisioner-csi.sh

.PHONY: update-addon.snapshot-controller
update-addon.snapshot-controller: ; $(info $(M) updating snapshot-controller manifests)
	./hack/addons/update-snapshot-controller.sh

.PHONY: update-addon.aws-ccm.%
update-addon.aws-ccm.%: ; $(info $(M) updating aws ccm $* manifests)
	./hack/addons/update-aws-ccm.sh $(AWS_CCM_VERSION_$*) $(AWS_CCM_CHART_VERSION)

.PHONY: update-addon.kube-vip
update-addon.kube-vip: ; $(info $(M) updating kube-vip manifests)
	./hack/addons/update-kube-vip-manifests.sh

.PHONY: generate-helm-configmap
generate-helm-configmap:
	go run hack/tools/helm-cm/main.go -kustomize-directory="./hack/addons/kustomize" \
	  -output-file="./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml"
	./hack/addons/add-warning-helm-configmap.sh

# Set only the supported CSI providers for each provider.
.PHONY: configure-csi-providers
configure-csi-providers: ; $(info $(M) configuring supported csi providers)
	./hack/addons/configure-supported-csi-providers.sh

.PHONY: generate-mindthegap-repofile
generate-mindthegap-repofile: generate-helm-configmap ; $(info $(M) generating helm repofile for mindthgap)
	./hack/addons/generate-mindthegap-repofile.sh

.PHONY: template-helm-repository
template-helm-repository: generate-mindthegap-repofile ## this is used by gorealeaser to set the helm value to this.
	yq -i '.data |= (to_entries | map(.value |= (. | fromjson | .RepositoryURL |= "{{ if .Values.helmRepository.enabled }}oci://helm-repository.{{ .Release.Namespace }}.svc/charts{{ else }}" + . + "{{ end }}" | to_yaml)) | from_entries)' ./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml

.PHONY: list-images
list-images:
	cd hack/tools/fetch-images && go run . \
	  -chart-directory=$(PWD)/charts/cluster-api-runtime-extensions-nutanix/ \
	  -helm-chart-configmap=$(PWD)/charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml \
	  -caren-version=$(CAREN_VERSION) \
	  -additional-yaml-files=$(PWD)/charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses/nutanix-cluster-class.yaml
