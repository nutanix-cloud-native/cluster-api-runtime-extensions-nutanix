// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patterns

func Anchored(pattern string) string {
	return "^" + pattern + "$"
}

func HTTPSURL() string {
	return `^https://`
}
