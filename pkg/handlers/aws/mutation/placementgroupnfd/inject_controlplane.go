package placementgroupnfd

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/placementgroup"
)

type controlPlanePatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewControlPlanePatch() *controlPlanePatchHandler {
	return &controlPlanePatchHandler{
		variableName: v1alpha1.ClusterConfigVariableName,
		variableFieldPath: []string{
			v1alpha1.ControlPlaneConfigVariableName,
			v1alpha1.AWSVariableName,
			placementgroup.VariableName,
		},
	}
}

func (h *controlPlanePatchHandler) Mutate(
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
			log.V(5).Info("placement group variable for AWS controlplane not defined.")
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
		selectors.ControlPlane(), log,
		func(obj *cabpkv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting placement group for local node feature discovery in AWS controlplane KubeadmConfigTemplate")
			obj.Spec.Template.Spec.Files = append(obj.Spec.Template.Spec.Files, cabpkv1.File{
				Path:        PlacementGroupDiscoveryScriptFileOnRemote,
				Content:     string(PlacementgroupDiscoveryScript),
				Permissions: "0700",
			})
			return nil
		},
	)
}
