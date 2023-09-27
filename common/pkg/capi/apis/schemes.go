// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"

	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capdv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
)

func CAPIScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))
	utilruntime.Must(capiv1.AddToScheme(scheme))

	return scheme
}

func CAPAScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))
	utilruntime.Must(capiv1.AddToScheme(scheme))
	utilruntime.Must(capav1.AddToScheme(scheme))

	return scheme
}

func CAPDScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1.AddToScheme(scheme))
	utilruntime.Must(capiv1.AddToScheme(scheme))
	utilruntime.Must(capdv1.AddToScheme(scheme))

	return scheme
}
