// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"

const (
	CNIProviderLabelKey = handlers.MetadataDomain + "/cni"

	PodSubnetAnnotationKey = handlers.MetadataDomain + "/pod-subnet"
)
