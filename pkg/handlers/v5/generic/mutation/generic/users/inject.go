// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	eksbootstrapv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
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
		v1alpha1.ClusterConfigVariableName,
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
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx, "holderRef", holderRef)

	usersVariable, err := variables.Get[[]v1alpha1.User](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("users variable not defined")
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
		usersVariable,
	)

	bootstrapUsers := []bootstrapv1.User{}
	for _, userFromVariable := range usersVariable {
		bootstrapUsers = append(bootstrapUsers, generateBootstrapUser(userFromVariable))
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("setting users in control plane kubeadm config template")

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
			obj.Spec.Template.Spec.Users = bootstrapUsers
			return nil
		}); err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef,
		selectors.WorkersConfigTemplateSelector(eksbootstrapv1.GroupVersion.String(), "NodeadmConfigTemplate"), log,
		func(obj *eksbootstrapv1.NodeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("setting users in worker node NodeadmConfig template")
			eksBootstrapUsers := make([]eksbootstrapv1.User, 0, len(bootstrapUsers))
			for i := range bootstrapUsers {
				user := &bootstrapUsers[i]
				var passwdFrom *eksbootstrapv1.PasswdSource
				if user.PasswdFrom.Secret.Name != "" {
					passwdFrom = &eksbootstrapv1.PasswdSource{
						Secret: eksbootstrapv1.SecretPasswdSource{
							Name: user.PasswdFrom.Secret.Name,
							Key:  user.PasswdFrom.Secret.Key,
						},
					}
				}
				eksBootstrapUsers = append(eksBootstrapUsers, eksbootstrapv1.User{
					Name:              user.Name,
					Gecos:             ptr.To(user.Gecos),
					Groups:            ptr.To(user.Groups),
					HomeDir:           ptr.To(user.HomeDir),
					Inactive:          user.Inactive,
					Shell:             ptr.To(user.Shell),
					Passwd:            ptr.To(user.Passwd),
					PasswdFrom:        passwdFrom,
					PrimaryGroup:      ptr.To(user.PrimaryGroup),
					LockPassword:      user.LockPassword,
					Sudo:              ptr.To(user.Sudo),
					SSHAuthorizedKeys: user.SSHAuthorizedKeys,
				})
			}

			obj.Spec.Template.Spec.Users = eksBootstrapUsers
			return nil
		}); err != nil {
		return err
	}

	return nil
}

func generateBootstrapUser(userFromVariable v1alpha1.User) bootstrapv1.User {
	bootstrapUser := bootstrapv1.User{
		Name:              userFromVariable.Name,
		SSHAuthorizedKeys: userFromVariable.SSHAuthorizedKeys,
	}

	// LockPassword is not part of our API, because we can derive its value
	// for the use cases our API supports.
	//
	// We do not support these edge cases:
	// (a) Hashed password is defined, password authentication is not enabled.
	// (b) Hashed password is not defined, password authentication is enabled.
	//
	// We disable password authentication by default.
	bootstrapUser.LockPassword = ptr.To(true)

	if userFromVariable.HashedPassword != "" {
		// We enable password authentication only if a hashed password is defined.
		bootstrapUser.LockPassword = ptr.To(false)
		bootstrapUser.Passwd = userFromVariable.HashedPassword
	}

	if userFromVariable.Sudo != "" {
		bootstrapUser.Sudo = userFromVariable.Sudo
	}

	return bootstrapUser
}
