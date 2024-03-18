// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "users"
)

type usersPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *usersPatchHandler {
	return newUsersPatchHandler(
		clusterconfig.MetaVariableName,
		VariableName)
}

func newUsersPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *usersPatchHandler {
	return &usersPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *usersPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ ctrlclient.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx, "holderRef", holderRef)

	usersVariable, found, err := variables.Get[v1alpha1.Users](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("users variable not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		usersVariable,
	)

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("setting users in control plane kubeadm config template")
			bootstrapUsers := []bootstrapv1.User{}
			for _, userFromVariable := range usersVariable {
				bootstrapUsers = append(bootstrapUsers, generateBootstrapUser(userFromVariable))
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.Users = bootstrapUsers
			return nil
		}); err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.WorkersKubeadmConfigTemplateSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("setting users in worker node kubeadm config template")
			bootstrapUsers := []bootstrapv1.User{}
			for _, userFromVariable := range usersVariable {
				bootstrapUsers = append(bootstrapUsers, generateBootstrapUser(userFromVariable))
			}
			obj.Spec.Template.Spec.Users = bootstrapUsers
			return nil
		}); err != nil {
		return err
	}

	return nil
}

func generateBootstrapUser(userFromVariable v1alpha1.User) bootstrapv1.User {
	bootstrapUser := bootstrapv1.User{
		Name:              userFromVariable.Name,
		Passwd:            userFromVariable.Passwd,
		SSHAuthorizedKeys: userFromVariable.SSHAuthorizedKeys,
		Sudo:              userFromVariable.Sudo,
	}

	// LockPassword is not part of our API, because we can derive its value
	// for the use cases our API supports.
	//
	// We do not support the edge cases where a password is defined, but
	// password authentication is disabled, or where no password is defined, but
	// password authentication is enabled.
	//
	// We disable password authentication by default.
	bootstrapUser.LockPassword = ptr.To[bool](true)
	if userFromVariable.Passwd != nil {
		// We enable password authentication only if a password is defined.
		bootstrapUser.LockPassword = ptr.To[bool](true)
	}

	return bootstrapUser
}
