// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentialprovider_test

import (
	"testing"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries/credentials/credentialprovider"
)

func TestURLsMatch(t *testing.T) {
	tests := []struct {
		globURL       string
		targetURL     string
		matchExpected bool
	}{
		// match when there is no path component
		{
			globURL:       "*.kubernetes.io",
			targetURL:     "prefix.kubernetes.io",
			matchExpected: true,
		},
		{
			globURL:       "prefix.*.io",
			targetURL:     "prefix.kubernetes.io",
			matchExpected: true,
		},
		{
			globURL:       "prefix.kubernetes.*",
			targetURL:     "prefix.kubernetes.io",
			matchExpected: true,
		},
		{
			globURL:       "*-good.kubernetes.io",
			targetURL:     "prefix-good.kubernetes.io",
			matchExpected: true,
		},
		// match with path components
		{
			globURL:       "*.kubernetes.io/blah",
			targetURL:     "prefix.kubernetes.io/blah",
			matchExpected: true,
		},
		{
			globURL:       "prefix.*.io/foo",
			targetURL:     "prefix.kubernetes.io/foo/bar",
			matchExpected: true,
		},
		// match with path components and ports
		{
			globURL:       "*.kubernetes.io:1111/blah",
			targetURL:     "prefix.kubernetes.io:1111/blah",
			matchExpected: true,
		},
		{
			globURL:       "prefix.*.io:1111/foo",
			targetURL:     "prefix.kubernetes.io:1111/foo/bar",
			matchExpected: true,
		},
		// no match when number of parts mismatch
		{
			globURL:       "*.kubernetes.io",
			targetURL:     "kubernetes.io",
			matchExpected: false,
		},
		{
			globURL:       "*.*.kubernetes.io",
			targetURL:     "prefix.kubernetes.io",
			matchExpected: false,
		},
		{
			globURL:       "*.*.kubernetes.io",
			targetURL:     "kubernetes.io",
			matchExpected: false,
		},
		// no match when some parts mismatch
		{
			globURL:       "kubernetes.io",
			targetURL:     "kubernetes.com",
			matchExpected: false,
		},
		{
			globURL:       "k*.io",
			targetURL:     "quay.io",
			matchExpected: false,
		},
		// no match when ports mismatch
		{
			globURL:       "*.kubernetes.io:1234/blah",
			targetURL:     "prefix.kubernetes.io:1111/blah",
			matchExpected: false,
		},
		{
			globURL:       "prefix.*.io/foo",
			targetURL:     "prefix.kubernetes.io:1111/foo/bar",
			matchExpected: false,
		},
	}
	for _, test := range tests {
		matched, _ := credentialprovider.URLsMatchStr(test.globURL, test.targetURL)
		if matched != test.matchExpected {
			t.Errorf("Expected match result of %s and %s to be %t, but was %t",
				test.globURL, test.targetURL, test.matchExpected, matched)
		}
	}
}
