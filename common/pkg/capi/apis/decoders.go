// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"

	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	capdv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
)

var capiCoreGroups = []schema.GroupVersion{
	controlplanev1.GroupVersion,
	bootstrapv1.GroupVersion,
	capiv1.GroupVersion,
}

func CAPIDecoder() runtime.Decoder {
	return serializer.NewCodecFactory(CAPIScheme()).UniversalDecoder(capiCoreGroups...)
}

func CAPDDecoder() runtime.Decoder {
	return serializer.NewCodecFactory(CAPDScheme()).UniversalDecoder(
		append(
			[]schema.GroupVersion{capdv1.GroupVersion},
			capiCoreGroups...,
		)...,
	)
}

func CAPADecoder() runtime.Decoder {
	return serializer.NewCodecFactory(CAPAScheme()).UniversalDecoder(
		append(
			[]schema.GroupVersion{capav1.GroupVersion},
			capiCoreGroups...,
		)...,
	)
}
