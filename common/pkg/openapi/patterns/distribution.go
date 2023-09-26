// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patterns

const (
	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L53
	NameSeparator = `(?:[._]|__|[-]+)`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L123
	PathComponent = Alphanumeric + `(` + NameSeparator + Alphanumeric + `)*`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L125-L130
	ImageRepository = `(` + HostAndOptionalPort + `/)?` + PathComponent + `(/` + PathComponent + `)*`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L68
	ImageTag = `[\w][\w.-]{0,127}`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L81
	ImageDigest = `[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][0-9A-Fa-f]{32,}`

	// See: https://github.com/distribution/reference/blob/v0.5.0/regexp.go#L136C2-L136C14
	ImageReference = ImageRepository + `(:` + ImageTag + `)?` + `(@` + ImageDigest + `)?`
)
