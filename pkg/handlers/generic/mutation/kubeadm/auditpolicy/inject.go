// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"context"
	"crypto/tls"
	_ "embed"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

type auditPolicyPatchHandler struct{}

//go:embed embedded/apiserver-audit-policy.yaml
var auditPolicy string

const auditPolicyPath = "/etc/kubernetes/audit-policy.yaml"

func NewPatch() *auditPolicyPatchHandler {
	return &auditPolicyPatchHandler{}
}

func (h *auditPolicyPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding files and updating API server extra args in kubeadm config spec")

			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				bootstrapv1.File{
					Path:        auditPolicyPath,
					Permissions: "0600",
					Content:     auditPolicy,
				},
			)

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
			if apiServer.ExtraArgs == nil {
				apiServer.ExtraArgs = make(map[string]string, 5)
			}

			// Originally, we had 1 log of 100MB, and 10 rotated logs of 100MB each, for a total of 1100MB.
			// We wanted to increase retention, but keep the total disk usage about the same.
			// Now, we have 1 log of 100MB, and 90 compressed, rotated logs of approximately 10MB each,
			// for a total of approximately 1000MB.
			apiServer.ExtraArgs["audit-log-path"] = "/var/log/audit/kube-apiserver-audit.log"
			apiServer.ExtraArgs["audit-log-maxage"] = "30"     // Maximum number of days to retain audit log files.
			apiServer.ExtraArgs["audit-log-maxbackup"] = "90"  // Maximum number of audit log files to retain.
			apiServer.ExtraArgs["audit-log-maxsize"] = "100"   // Maximum size of log file in MB before it is rotated.
			apiServer.ExtraArgs["audit-log-compress"] = "true" // Compress (gzip) audit log file when it is rotated.
			apiServer.ExtraArgs["audit-policy-file"] = auditPolicyPath
			apiServer.ExtraArgs["tls-cipher-suites"] = strings.Join(
				[]string{
					tls.CipherSuiteName(tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256),
					tls.CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256),
					tls.CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384),
					tls.CipherSuiteName(tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384),
				},
				",",
			)

			if apiServer.ExtraVolumes == nil {
				apiServer.ExtraVolumes = make([]bootstrapv1.HostPathMount, 0, 2)
			}

			apiServer.ExtraVolumes = append(
				apiServer.ExtraVolumes,
				bootstrapv1.HostPathMount{
					Name:      "audit-policy",
					HostPath:  auditPolicyPath,
					MountPath: auditPolicyPath,
					ReadOnly:  true,
					PathType:  corev1.HostPathFile,
				},
				bootstrapv1.HostPathMount{
					Name:      "audit-logs",
					HostPath:  "/var/log/kubernetes/audit",
					MountPath: "/var/log/audit/",
					PathType:  corev1.HostPathDirectoryOrCreate,
				},
			)

			return nil
		},
	)
}
