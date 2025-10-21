package placementgroupnfd

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eksbootstrapv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/placementgroup"
	awsplacementgroupnfd "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/placementgroupnfd"
)

type workerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewWorkerPatch() *workerPatchHandler {
	return &workerPatchHandler{
		variableName: v1alpha1.WorkerConfigVariableName,
		variableFieldPath: []string{
			v1alpha1.EKSVariableName,
			placementgroup.VariableName,
		},
	}
}

func (h *workerPatchHandler) Mutate(
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

	placementGroupVar, err := variables.Get[v1alpha1.PlacementGroup](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("placement group variable for EKS worker not defined.")
			return nil
		}
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		placementGroupVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.WorkersConfigTemplateSelector(eksbootstrapv1.GroupVersion.String(), "NodeadmConfigTemplate"), log,
		func(obj *eksbootstrapv1.NodeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting placement group for local node feature discovery in EKS workers NodeadmConfigTemplate")
			obj.Spec.Template.Spec.Files = append(obj.Spec.Template.Spec.Files, eksbootstrapv1.File{
				Path:        awsplacementgroupnfd.PlacementGroupDiscoveryScriptFileOnRemote,
				Content:     string(awsplacementgroupnfd.PlacementgroupDiscoveryScript),
				Permissions: "0700",
			})
			return nil
		},
	)
}
