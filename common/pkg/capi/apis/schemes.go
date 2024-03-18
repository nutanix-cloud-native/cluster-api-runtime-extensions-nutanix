// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capdv1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"

	capav1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
)

func NewScheme(registerFuncs ...func(*runtime.Scheme) error) *runtime.Scheme {
	sb := runtime.NewSchemeBuilder(registerFuncs...)
	scheme := runtime.NewScheme()
	utilruntime.Must(sb.AddToScheme(scheme))
	return scheme
}

func CAPIRegisterFuncs() []func(*runtime.Scheme) error {
	return []func(*runtime.Scheme) error{
		bootstrapv1.AddToScheme,
		controlplanev1.AddToScheme,
		capiv1.AddToScheme,
	}
}

func CAPARegisterFuncs() []func(*runtime.Scheme) error {
	return []func(*runtime.Scheme) error{
		capav1.AddToScheme,
	}
}

func CAPDRegisterFuncs() []func(*runtime.Scheme) error {
	return []func(*runtime.Scheme) error{
		capdv1.AddToScheme,
	}
}

func CAPIScheme() *runtime.Scheme {
	return NewScheme(CAPIRegisterFuncs()...)
}

func CAPAScheme() *runtime.Scheme {
	return NewScheme(append(CAPIRegisterFuncs(), CAPARegisterFuncs()...)...)
}

func CAPDScheme() *runtime.Scheme {
	return NewScheme(append(CAPIRegisterFuncs(), CAPDRegisterFuncs()...)...)
}
