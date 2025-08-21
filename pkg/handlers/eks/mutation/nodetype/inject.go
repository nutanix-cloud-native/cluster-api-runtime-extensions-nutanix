// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nodetype

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eksbootstrapv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/ami"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "nodeType"
)

var versionLessThan133Range = semver.MustParseRange("<1.33.0-0")

type eksNodeTypePatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewWorkerPatch() *eksNodeTypePatchHandler {
	return newEKSNodeTypePatchHandler(
		v1alpha1.WorkerConfigVariableName,
		v1alpha1.EKSVariableName,
		VariableName,
	)
}

func newEKSNodeTypePatchHandler(
	variableName string,
	variableFieldPath ...string,
) *eksNodeTypePatchHandler {
	return &eksNodeTypePatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *eksNodeTypePatchHandler) Mutate(
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

	// If there is no MD version then we can skip patching this resource.
	mdVersion, err := variables.Get[string](
		vars,
		runtimehooksv1.BuiltinsName,
		"machineDeployment",
		"version",
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	nodeTypeVar, err := variables.Get[string](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil && !variables.IsNotFoundError(err) {
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
	)

	amiSpecVar, err := variables.Get[v1alpha1.AMISpec](
		vars,
		v1alpha1.WorkerConfigVariableName,
		v1alpha1.EKSVariableName,
		ami.VariableName,
	)
	if err != nil && !variables.IsNotFoundError(err) {
		return err
	}

	// Set the default node type based on Kubernetes version if AMI variable is not set.
	// Kubernetes switched to AL2023 in 1.33.0 which requires a node type of al2023 in
	// order for bootstrap configuration to be valid for nodeadm as opposed to cloud-init.
	if nodeTypeVar == "" {
		// If AMI ID or AMI lookup is set, then we don't need to set the node type.
		if amiSpecVar.ID != "" ||
			(amiSpecVar.Lookup != nil &&
				amiSpecVar.Lookup.Format != "" &&
				amiSpecVar.Lookup.Org != "" &&
				amiSpecVar.Lookup.BaseOS != "") {
			return nil
		}

		mdKubernetesVersion, err := semver.ParseTolerant(mdVersion)
		if err != nil {
			return fmt.Errorf("failed to parse nodepool Kubernetes version %q: %w", mdVersion, err)
		}

		if versionLessThan133Range(mdKubernetesVersion) {
			return nil
		}

		nodeTypeVar = string(eksbootstrapv1.NodeTypeAL2023)
	}

	if nodeTypeVar != string(eksbootstrapv1.NodeTypeAL2023) {
		return fmt.Errorf("invalid node type specified: %s", nodeTypeVar)
	}

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.WorkersConfigTemplateSelector(eksbootstrapv1.GroupVersion.String(), "EKSConfigTemplate"),
		log,
		func(obj *eksbootstrapv1.EKSConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
				"nodeType", nodeTypeVar,
			).Info("setting NodeType in EKSConfigTemplate spec")

			obj.Spec.Template.Spec.NodeType = eksbootstrapv1.NodeType(nodeTypeVar)

			return nil
		},
	); err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureWorkerMachineTemplates(
			capav1.GroupVersion.Version,
			"AWSMachineTemplate",
		),
		log,
		func(obj *capav1.AWSMachineTemplate) error {
			log = log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
				"nodeType", nodeTypeVar,
			)

			log.Info("setting insecureSkipSecretsManager in AWSMachineTemplate spec")

			obj.Spec.Template.Spec.CloudInit.InsecureSkipSecretsManager = true

			if amiSpecVar.ID != "" ||
				(amiSpecVar.Lookup != nil &&
					amiSpecVar.Lookup.Format != "" &&
					amiSpecVar.Lookup.Org != "" &&
					amiSpecVar.Lookup.BaseOS != "") {
				return nil
			}

			log.Info("setting EKS optimized AMI lookup type in AWSMachineTemplate spec")

			obj.Spec.Template.Spec.AMI.EKSOptimizedLookupType = ptr.To(capav1.AmazonLinux2023)

			return nil
		},
	); err != nil {
		return err
	}

	return nil
}
