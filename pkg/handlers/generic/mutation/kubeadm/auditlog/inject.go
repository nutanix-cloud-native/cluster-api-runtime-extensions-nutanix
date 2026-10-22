// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the field name under clusterConfig for AuditLog.
	VariableName = "auditLog"

	// WebhookKubeconfigSecretKey is the Secret data key the handler reads for audit-webhook-config-file.
	WebhookKubeconfigSecretKey = "kubeconfig"

	// AuditPolicyDataKey is the only ConfigMap.Data key read for the audit policy (YAML or JSON string).
	// BinaryData is not supported.
	AuditPolicyDataKey = "policy.yaml"
)

// Implementation details that are not configurable.
const (
	auditLogFilePath      = "/var/log/audit/kube-apiserver-audit.log"
	auditLogDir           = "/var/log/kubernetes/audit"
	auditLogMountPath     = "/var/log/audit/"
	webhookKubeconfigPath = "/etc/kubernetes/audit-webhook-kubeconfig.yaml"
	auditPolicyPath       = "/etc/kubernetes/audit-policy.yaml"
)

//go:embed embedded/default-audit-policy.yaml
var defaultAuditPolicy string

type auditLogPatchHandler struct {
	client            client.Client
	variableName      string
	variableFieldPath []string
}

// NewPatch returns a mutator that configures API server audit logging from the clusterConfig.auditLog variable.
func NewPatch(c client.Client) *auditLogPatchHandler {
	return &auditLogPatchHandler{
		client:            c,
		variableName:      v1alpha1.ClusterConfigVariableName,
		variableFieldPath: []string{VariableName},
	}
}

func (h *auditLogPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues("holderRef", holderRef)

	auditCfg, err := variables.Get[v1alpha1.AuditLog](vars, h.variableName, h.variableFieldPath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("audit log variable not defined")
			return nil
		}
		return err
	}

	if auditCfg.Webhook == nil && auditCfg.Log == nil {
		log.V(5).Info("audit log backends not configured")
		return nil
	}

	policy := defaultAuditPolicy

	if auditCfg.Policy != nil {
		if auditCfg.Policy.ConfigMap == nil {
			// This case should be rejected by validation, but we handle it to be safe.
			return fmt.Errorf(
				"audit log policy ConfigMap is required when webhook or log backend is configured",
			)
		}

		cm := &corev1.ConfigMap{}
		cmKey := client.ObjectKey{Namespace: clusterKey.Namespace, Name: auditCfg.Policy.ConfigMap.Name}
		if err := h.client.Get(ctx, cmKey, cm); err != nil {
			return fmt.Errorf("failed to get audit policy ConfigMap %q: %w", cmKey, err)
		}

		policy, err = policyFromConfigMap(cm)
		if err != nil {
			return err
		}
	}

	files := []bootstrapv1.File{
		{
			Path:        auditPolicyPath,
			Permissions: "0600",
			Content:     policy,
		},
	}

	var extraArgs []bootstrapv1.Arg
	var extraVolumes []bootstrapv1.HostPathMount

	switch {
	case auditCfg.Webhook != nil:
		wh := auditCfg.Webhook
		if wh.Secret == nil {
			return fmt.Errorf("audit webhook secret is required")
		}
		files = append(files, bootstrapv1.File{
			Path:        webhookKubeconfigPath,
			Permissions: "0600",
			ContentFrom: bootstrapv1.FileSource{
				Secret: bootstrapv1.SecretFileSource{
					Name: wh.Secret.Name,
					Key:  WebhookKubeconfigSecretKey,
				},
			},
		})

		mode := wh.Mode
		if mode == "" {
			// Mode is marked optional, but has a default value, so it should never be empty.
			return fmt.Errorf("audit webhook mode is required")
		}

		extraArgs = append(extraArgs,
			bootstrapv1.Arg{Name: "audit-policy-file", Value: ptr.To(auditPolicyPath)},
			bootstrapv1.Arg{Name: "audit-webhook-config-file", Value: ptr.To(webhookKubeconfigPath)},
			bootstrapv1.Arg{Name: "audit-webhook-mode", Value: ptr.To(mode)},
		)
		if d := wh.InitialBackoff.Duration; d > 0 {
			extraArgs = append(extraArgs, bootstrapv1.Arg{
				Name:  "audit-webhook-initial-backoff",
				Value: ptr.To(d.String()),
			})
		}
		if mode == "batch" && wh.EventBatching != nil {
			extraArgs = append(extraArgs, webhookBatchArgs(wh.EventBatching)...)
		}

		extraVolumes = append(
			extraVolumes,
			hostPathVolume("audit-policy", auditPolicyPath, auditPolicyPath, true, corev1.HostPathFile),
			hostPathVolume(
				"audit-webhook-kubeconfig",
				webhookKubeconfigPath,
				webhookKubeconfigPath,
				true,
				corev1.HostPathFile,
			),
		)

	case auditCfg.Log != nil:
		lg := auditCfg.Log
		logPath := lg.Path
		if logPath == "" {
			logPath = auditLogFilePath
		}
		mode := lg.Mode
		if mode == "" {
			// Mode is marked optional, but has a default value, so it should never be empty.
			return fmt.Errorf("audit log mode is required")
		}
		extraArgs = append(extraArgs,
			bootstrapv1.Arg{Name: "audit-policy-file", Value: ptr.To(auditPolicyPath)},
			bootstrapv1.Arg{Name: "audit-log-path", Value: ptr.To(logPath)},
			bootstrapv1.Arg{Name: "audit-log-mode", Value: ptr.To(mode)},
		)
		if lg.MaxAge > 0 {
			extraArgs = append(extraArgs, bootstrapv1.Arg{
				Name:  "audit-log-maxage",
				Value: ptr.To(strconv.FormatInt(int64(lg.MaxAge), 10)),
			})
		}
		if lg.MaxBackup > 0 {
			extraArgs = append(extraArgs, bootstrapv1.Arg{
				Name:  "audit-log-maxbackup",
				Value: ptr.To(strconv.FormatInt(int64(lg.MaxBackup), 10)),
			})
		}
		if lg.MaxSize > 0 {
			extraArgs = append(extraArgs, bootstrapv1.Arg{
				Name:  "audit-log-maxsize",
				Value: ptr.To(strconv.FormatInt(int64(lg.MaxSize), 10)),
			})
		}
		if lg.Compress {
			extraArgs = append(extraArgs, bootstrapv1.Arg{Name: "audit-log-compress", Value: ptr.To("true")})
		}
		if mode == "batch" && lg.EventBatching != nil {
			extraArgs = append(extraArgs, logBatchArgs(lg.EventBatching)...)
		}

		extraVolumes = append(extraVolumes,
			hostPathVolume("audit-policy", auditPolicyPath, auditPolicyPath, true, corev1.HostPathFile),
			hostPathVolume("audit-logs", auditLogDir, auditLogMountPath, false, corev1.HostPathDirectoryOrCreate),
		)
	}

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("configuring API server audit logging from cluster config")

			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				files...,
			)

			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
			mergeExtraArgs(apiServer, extraArgs)
			mergeExtraVolumes(apiServer, extraVolumes)

			return nil
		},
	)
}

func webhookBatchArgs(b *v1alpha1.AuditLogEventBatching) []bootstrapv1.Arg {
	var args []bootstrapv1.Arg
	if b.BufferSize > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-webhook-batch-buffer-size",
			Value: ptr.To(strconv.FormatInt(int64(b.BufferSize), 10)),
		})
	}
	if b.MaxSize > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-webhook-batch-max-size",
			Value: ptr.To(strconv.FormatInt(int64(b.MaxSize), 10)),
		})
	}
	if b.MaxWait > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-webhook-batch-max-wait",
			Value: ptr.To(fmt.Sprintf("%ds", b.MaxWait)),
		})
	}
	if b.ThrottleEnable {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-webhook-batch-throttle-enable",
			Value: ptr.To("true"),
		})
	}
	if b.ThrottleQPS > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-webhook-batch-throttle-qps",
			Value: ptr.To(strconv.FormatFloat(float64(b.ThrottleQPS), 'f', -1, 64)),
		})
	}
	if b.ThrottleBurst > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-webhook-batch-throttle-burst",
			Value: ptr.To(strconv.FormatInt(int64(b.ThrottleBurst), 10)),
		})
	}
	return args
}

func logBatchArgs(b *v1alpha1.AuditLogEventBatching) []bootstrapv1.Arg {
	var args []bootstrapv1.Arg
	if b.BufferSize > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-log-batch-buffer-size",
			Value: ptr.To(strconv.FormatInt(int64(b.BufferSize), 10)),
		})
	}
	if b.MaxSize > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-log-batch-max-size",
			Value: ptr.To(strconv.FormatInt(int64(b.MaxSize), 10)),
		})
	}
	if b.MaxWait > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-log-batch-max-wait",
			Value: ptr.To(fmt.Sprintf("%ds", b.MaxWait)),
		})
	}
	if b.ThrottleEnable {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-log-batch-throttle-enable",
			Value: ptr.To("true"),
		})
	}
	if b.ThrottleQPS > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-log-batch-throttle-qps",
			Value: ptr.To(strconv.FormatFloat(float64(b.ThrottleQPS), 'f', -1, 64)),
		})
	}
	if b.ThrottleBurst > 0 {
		args = append(args, bootstrapv1.Arg{
			Name:  "audit-log-batch-throttle-burst",
			Value: ptr.To(strconv.FormatInt(int64(b.ThrottleBurst), 10)),
		})
	}
	return args
}

func hostPathVolume(
	name, hostPath, mountPath string,
	readOnly bool,
	pathType corev1.HostPathType,
) bootstrapv1.HostPathMount {
	return bootstrapv1.HostPathMount{
		Name:      name,
		HostPath:  hostPath,
		MountPath: mountPath,
		ReadOnly:  ptr.To(readOnly),
		PathType:  pathType,
	}
}

func mergeExtraArgs(apiServer *bootstrapv1.APIServer, args []bootstrapv1.Arg) {
	seen := make(map[string]bool, len(apiServer.ExtraArgs)+len(args))
	for _, arg := range apiServer.ExtraArgs {
		seen[arg.Name] = true
	}
	for _, arg := range args {
		if !seen[arg.Name] {
			apiServer.ExtraArgs = append(apiServer.ExtraArgs, arg)
			seen[arg.Name] = true
		}
	}
}

func mergeExtraVolumes(apiServer *bootstrapv1.APIServer, mounts []bootstrapv1.HostPathMount) {
	if apiServer.ExtraVolumes == nil {
		apiServer.ExtraVolumes = make([]bootstrapv1.HostPathMount, 0, len(mounts))
	}
	seen := make(map[string]bool)
	for _, v := range apiServer.ExtraVolumes {
		seen[v.Name] = true
	}
	for _, m := range mounts {
		if !seen[m.Name] {
			apiServer.ExtraVolumes = append(apiServer.ExtraVolumes, m)
			seen[m.Name] = true
		}
	}
}
