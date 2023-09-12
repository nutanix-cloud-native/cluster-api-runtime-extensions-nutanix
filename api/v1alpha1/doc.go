// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 contains API Schema definitions for the CAPI extensions v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=capiext.labs.d2iq.io
//
//go:generate -command CTRLGEN controller-gen  paths="./..."
//go:generate CTRLGEN rbac:headerFile="../../hack/license-header.yaml.txt",roleName=capi-runtime-extensions-manager-role output:rbac:artifacts:config=../../charts/capi-runtime-extensions/templates
//go:generate CTRLGEN object:headerFile="../../hack/license-header.go.txt" output:object:artifacts:config=/dev/null
package v1alpha1
