// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_setDefaultStorageClass(t *testing.T) {
	tests := []struct {
		name           string
		startConfigMap *corev1.ConfigMap
		key            string
	}{
		{
			name: "aws config map",
			startConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test1",
					Namespace: "default",
				},
				Data: map[string]string{},
			},
			key: "aws-ebs-csi.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
