// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "cilium.io"
	Version   = "v2alpha1"
)

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&CiliumL2AnnouncementPolicy{},
		&CiliumL2AnnouncementPolicyList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
