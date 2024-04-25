// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func DecoderForScheme(scheme *runtime.Scheme) runtime.Decoder {
	return serializer.NewCodecFactory(scheme).UniversalDecoder(
		scheme.PrioritizedVersionsAllGroups()...,
	)
}

func CAPIDecoder() runtime.Decoder {
	return DecoderForScheme(CAPIScheme())
}

func CAPDDecoder() runtime.Decoder {
	return DecoderForScheme(CAPDScheme())
}

func CAPADecoder() runtime.Decoder {
	return DecoderForScheme(CAPAScheme())
}
