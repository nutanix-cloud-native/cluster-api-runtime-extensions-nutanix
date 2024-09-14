# Copyright 2023 Nutanix. All rights reserved.
# SPDX-License-Identifier: Apache-2.0

export CALICO_VERSION := v3.28.1
export CILIUM_VERSION := 1.15.6
export NODE_FEATURE_DISCOVERY_VERSION := 0.16.1
export CLUSTER_AUTOSCALER_CHART_VERSION := 9.37.0
export AWS_EBS_CSI_CHART_VERSION := 2.32.0
export NUTANIX_STORAGE_CSI_CHART_VERSION := 3.0.0
export LOCAL_PATH_CSI_CHART_VERSION := 0.0.29
export SNAPSHOT_CONTROLLER_CHART_VERSION := 3.0.5
# AWS CCM uses the same chart version for all kubernetes versions. The image used in the deployment will
# be updated by the addon kustomization for CRS deployments and via Helm values for HelmAddon deployments.
export AWS_CCM_CHART_VERSION := 0.0.8
# A map of AWS CCM versions.
export AWS_CCM_VERSION_127 := v1.27.7
export AWS_CCM_VERSION_128 := v1.28.6
export AWS_CCM_VERSION_129 := v1.29.3
export AWS_CCM_VERSION_130 := v1.30.1

export NUTANIX_CCM_CHART_VERSION := 0.4.1

export KUBE_VIP_VERSION := v0.8.1

export METALLB_CHART_VERSION := 0.14.8

.PHONY: addons.sync
addons.sync: $(addprefix update-addon.,calico cilium nfd cluster-autoscaler snapshot-controller local-path-provisioner-csi aws-ebs-csi aws-ccm.127 aws-ccm.128 aws-ccm.129 aws-ccm.130 kube-vip)

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
	sed -i '/RepositoryURL:/s#\(RepositoryURL: *\)\(.*\)#\1{{ if .Values.selfHostedRegistry }}oci://helm-repository.{{ .Release.Namespace }}.svc/charts{{ else }}\2{{ end }}#'  "./charts/cluster-api-runtime-extensions-nutanix/templates/helm-config.yaml"
