// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package name

import "fmt"

type NameGenerator func(string) string

func Suffix(suffix string) NameGenerator {
	return func(s string) string {
		return fmt.Sprintf("%s-%s", s, suffix)
	}
}
