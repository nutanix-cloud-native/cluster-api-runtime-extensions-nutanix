// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patterns

const (
	// See: https://github.com/kubernetes/apimachinery/blob/v0.28.1/pkg/util/validation/validation.go#L178
	DNS1123Label = `[a-z0-9]([-a-z0-9]*[a-z0-9])?`

	// See: https://github.com/kubernetes/apimachinery/blob/v0.28.1/pkg/util/validation/validation.go#L205
	DNS1123Subdomain = DNS1123Label + `(\.` + DNS1123Label + `)*`
)
