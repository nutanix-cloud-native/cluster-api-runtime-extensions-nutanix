// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patterns

const (
	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L53
	NameSeparator = `(?:[._]|__|[-]+)`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L123C18-L123C65
	PathComponent = Alphanumeric + `(` + NameSeparator + Alphanumeric + `)*`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L130
	ImageRegistry = `(` + DNS1123Subdomain + `|` + IPv6 + `)` + OptionalPort + `(/` + PathComponent + `)*`
)
