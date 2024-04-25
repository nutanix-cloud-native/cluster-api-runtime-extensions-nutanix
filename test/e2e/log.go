//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
)

// This code was inspired from kubernetes/kubernetes, specifically
// https://github.com/oomichi/kubernetes/blob/master/test/e2e/framework/log.go

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func logf(level, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf prints info logs with a timestamp and formatting.
func Logf(format string, args ...interface{}) {
	logf("INFO", format, args...)
}

// LogWarningf prints warning logs with a timestamp and formatting.
func LogWarningf(format string, args ...interface{}) {
	logf("WARNING", format, args...)
}

// Log prints info logs with a timestamp.
func Log(message string) {
	logf("INFO", message)
}

// LogWarning prints warning logs with a timestamp.
func LogWarning(message string) {
	logf("WARNING", message)
}
