// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patterns

const (
	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L91
	IPv6 = `\[(?:[a-fA-F0-9:]+)\]`

	Port = `:[0-9]+`

	// See https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L65
	OptionalPort = `(` + Port + `)?`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L106
	Host = `(?:` + DNS1123Subdomain + `|` + IPv6 + `)`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L110
	HostAndOptionalPort = Host + OptionalPort
)
