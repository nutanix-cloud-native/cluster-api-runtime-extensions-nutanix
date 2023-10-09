// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var startAWSConfigMap = `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ebs-sc
parameters:
  csi.storage.k8s.io/fstype: ext4
  type: gp3
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
`

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
				Data: map[string]string{
					"aws-ebs-csi.yaml": startAWSConfigMap,
				},
			},
			key: "aws-ebs-csi.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setDefaultStorageClass(context.TODO(), ctrl.LoggerFrom(context.TODO()), tt.startConfigMap)
			if err != nil {
				t.Fatal("failed to set default storage class", err)
			}
			if !strings.Contains(tt.startConfigMap.Data[tt.key], defualtStorageClassKey) {
				t.Logf(
					"expected %s to containe %s",
					tt.startConfigMap.Data[tt.key],
					defualtStorageClassKey,
				)
				t.Fail()
			}
		})
	}
}
