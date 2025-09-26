// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentialprovider

import (
	"regexp"
)

//nolint:gochecknoglobals // Used here to more easily test URL globs.
var (
	ecrURLGlobs = []string{
		"*.dkr.ecr.*.amazonaws.com",
		"*.dkr.ecr.*.amazonaws.cn",
		"*.dkr.ecr-fips.*.amazonaws.com",
		"*.dkr.ecr.us-iso-east-1.c2s.ic.gov",
		"*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov",
	}

	gcrURLGlobs = []string{
		"container.cloud.google.com",
		"gcr.io",
		"*.gcr.io",
		"*.pkg.dev",
	}

	acrURLGlobs = []string{
		"*.azurecr.io",
		"*.azurecr.cn",
		"*.azurecr.de",
		"*.azurecr.us",
		"*.azurecr.*",
	}
)

func URLMatchesKnownRegistryProvider(target string) (bool, error) {
	urlGlobs := make([]string, 0, len(ecrURLGlobs)+len(gcrURLGlobs)+len(acrURLGlobs))
	urlGlobs = append(urlGlobs, ecrURLGlobs...)
	urlGlobs = append(urlGlobs, gcrURLGlobs...)
	urlGlobs = append(urlGlobs, acrURLGlobs...)
	return URLMatchesOneOfGlobs(urlGlobs, target)
}

func URLMatchesECR(target string) (bool, error) {
	return URLMatchesOneOfGlobs(ecrURLGlobs, target)
}

func URLMatchesGCR(target string) (bool, error) {
	return URLMatchesOneOfGlobs(gcrURLGlobs, target)
}

func URLMatchesACR(target string) (bool, error) {
	return URLMatchesOneOfGlobs(acrURLGlobs, target)
}

func URLMatchesOneOfGlobs(globs []string, target string) (bool, error) {
	// Strip scheme from target if present.
	target = regexp.MustCompile("^https?://").ReplaceAllString(target, "")

	for _, g := range globs {
		matches, err := URLsMatchStr(g, target)
		if err != nil {
			return false, err
		}
		if matches {
			return matches, nil
		}
	}

	return false, nil
}
