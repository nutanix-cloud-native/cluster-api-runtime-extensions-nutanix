// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Adapted from the internal utility authored by raghu.rapole@nutanix.com

package utils

import (
	"regexp"
	"strconv"
	"strings"
)

// Regex for PC version format 202x.xx.xx.xx.
var pcVersion202xRe = regexp.MustCompile(`^202(\d(\.\d+)+)$`)

// Regex for PC version format 7.xx.xx.
var pcVersion7xRe = regexp.MustCompile(`^7((\.\d+)+)$`)

// CleanPCVersion normalizes a Prism Central version string by trimming whitespace,
// lower-casing it, and removing the optional "pc." prefix.
func CleanPCVersion(version string) string {
	lowerVersion := strings.ToLower(strings.TrimSpace(version))
	return strings.TrimPrefix(lowerVersion, "pc.")
}

// convertStringToIntList converts input '.' separated string to list of integers.
// If any part of the string is not convertable to integer or if input is empty
// string, it will return [9999].
func convertStringToIntList(str string) []int {
	strList := strings.Split(str, ".")
	var intList []int
	for _, x := range strList {
		if val, err := strconv.Atoi(x); err != nil {
			return []int{9999}
		} else {
			intList = append(intList, val)
		}
	}
	return intList
}

// CompareVersions compares version numbers of the format '3.5.2.1'.
// Returns 0 : if v1 == v2
// Returns 1 : if v1 > v2
// Returns -1: if v1 < v2
//
// If either version is not in the correct format, they will be the greater,
// unless neither can be parsed in which case they are equal. The case where a
// branch can't be parsed is if the cluster is running master, or some other
// non-release branch, or empty string. This is only expected in a test/debug
// situation and is the motivation for making an unparseable format greater.
func CompareVersions(v1, v2 string) int {
	if strings.EqualFold(v1, "master") {
		v1 = "9999"
	}
	if strings.EqualFold(v2, "master") {
		v2 = "9999"
	}

	v1IntList := convertStringToIntList(v1)
	v2IntList := convertStringToIntList(v2)

	var maxLen int
	if len(v1IntList) < len(v2IntList) {
		maxLen = len(v2IntList)
	} else {
		maxLen = len(v1IntList)
	}

	v1NormIntList := make([]int, maxLen)
	v2NormIntList := make([]int, maxLen)
	copy(v1NormIntList, v1IntList)
	copy(v2NormIntList, v2IntList)

	for i, e := range v1NormIntList {
		if e > v2NormIntList[i] {
			return 1
		} else if e < v2NormIntList[i] {
			return -1
		}
	}
	return 0
}

// is202XPcVersion checks if the version is of the format 202x.xx.xx..
func is202XPcVersion(version string) bool {
	return pcVersion202xRe.MatchString(version)
}

// is7XPcVersion checks if the version is of the format 7.xx.xx..
func is7XPcVersion(version string) bool {
	return pcVersion7xRe.MatchString(version)
}

// ComparePCVersions compares PC version numbers of the format '2024.2.0.1', '7.3', etc.
// Returns 0 : if ver1 == ver2
// Returns 1 : if ver1 > ver2
// Returns -1: if ver1 < ver2
//
// If either version is not in the correct format, they will be the greater,
// unless neither can be parsed in which case they are equal. The case where a
// branch can't be parsed is if the cluster is running master, or some other
// non-release branch. This is only expected in a test/debug situation and is
// the motivation for making an unparseable format greater.
func ComparePCVersions(v1, v2 string) int {
	cleanV1 := CleanPCVersion(v1)
	cleanV2 := CleanPCVersion(v2)

	// Special case for comparing PC versions of format 7.xx and 202x.xx.xx
	if is7XPcVersion(cleanV1) && is202XPcVersion(cleanV2) {
		return 1
	}
	if is7XPcVersion(cleanV2) && is202XPcVersion(cleanV1) {
		return -1
	}

	return CompareVersions(cleanV1, cleanV2)
}
