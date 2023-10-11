package ami

import (
	"context"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	ControlPlaneHandlerNamePatch = "AWSAMISpecControlPlanePatch"
	VariableName                 = "ami"
)

var (
	_ commonhandlers.Named = &awsAMISpecPatchHandler{}
	_ mutation.MetaMutator = &awsAMISpecPatchHandler{}
)

type awsAMISpecPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
	patchSelector     clusterv1.PatchSelector
}

func newAWSAMISpecPatchHandler(
	metaVariableName string,
	variableFieldPath []string,
	patchSelector clusterv1.PatchSelector,
) *awsAMISpecPatchHandler {
	return &awsAMISpecPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
		patchSelector:     patchSelector,
	}
}

func (h *awsAMISpecPatchHandler) Name() string {
	return ControlPlaneHandlerNamePatch
}

func (h *awsAMISpecPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	amiSpecVar, found, err := variables.Get[v1alpha1.AMISpec](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("AWS amiSpec variable not defined. Default AMI provided by CAPA will be used.")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.metaVariableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		amiSpecVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		h.patchSelector,
		log,
		func(obj *capav1.AWSMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting AMI in AWSMachineTemplate spec")

			obj.Spec.Template.Spec.AMI = capav1.AMIReference{ID: &amiSpecVar.ID}
			obj.Spec.Template.Spec.ImageLookupFormat = amiSpecVar.Format
			obj.Spec.Template.Spec.ImageLookupOrg = amiSpecVar.Org
			obj.Spec.Template.Spec.ImageLookupBaseOS = amiSpecVar.BaseOS

			return nil
		},
	)
}

func (h *awsAMISpecPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		unstructured.UnstructuredJSONScheme,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			return h.Mutate(
				ctx,
				obj.(*unstructured.Unstructured),
				vars,
				holderRef,
				client.ObjectKey{},
			)
		},
	)
}
