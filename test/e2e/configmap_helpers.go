//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WaitForConfigMapDataInput struct {
	Getter        framework.Getter
	ConfigMap     *corev1.ConfigMap
	DataValidator func(data map[string]string) bool
}

func WaitForConfigMapData(
	ctx context.Context,
	input WaitForConfigMapDataInput,
	intervals ...interface{},
) {
	start := time.Now()
	key := client.ObjectKeyFromObject(input.ConfigMap)
	capie2e.Byf("waiting for ConfigMap %s to have expected data", key)
	Log("starting to wait for ConfigMap to have expected data")
	Eventually(func() bool {
		if err := input.Getter.Get(ctx, key, input.ConfigMap); err == nil {
			return input.DataValidator(input.ConfigMap.Data)
		}
		return false
	}, intervals...).Should(BeTrue(), func() string {
		return DescribeIncorrectConfigMapData(input, input.ConfigMap)
	})
	Logf("ConfigMap %s has expected data, took %v", key, time.Since(start))
}

// DescribeIncorrectConfigMapData returns detailed output to help debug a ConfigMap data validation failure in e2e.
func DescribeIncorrectConfigMapData(
	input WaitForConfigMapDataInput,
	configMap *corev1.ConfigMap,
) string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("ConfigMap %s failed to get expected data:\n",
		klog.KObj(input.ConfigMap)))
	if configMap == nil {
		b.WriteString("\nConfigMap: nil\n")
	} else {
		b.WriteString(fmt.Sprintf("\nConfigMap:\n%s\n", framework.PrettyPrint(configMap)))
	}
	return b.String()
}
