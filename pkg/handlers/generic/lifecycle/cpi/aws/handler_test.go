// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var startAWSCPIConfigMap = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    helm.sh/chart: aws-cloud-controller-manager-0.0.8
  name: system:cloud-controller-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
- apiGroup: ""
  kind: ServiceAccount
  name: cloud-controller-manager
  namespace: kube-system
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    helm.sh/chart: aws-cloud-controller-manager-0.0.8
    k8s-app: aws-cloud-controller-manager
  name: aws-cloud-controller-manager
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: aws-cloud-controller-manager
  template:
    metadata:
      labels:
        k8s-app: aws-cloud-controller-manager
      name: aws-cloud-controller-manager
    spec:
      containers:
      - args:
        - --v=2
        - --cloud-provider=aws
        - --configure-cloud-routes=false
        env: []
        image: registry.k8s.io/provider-aws/cloud-controller-manager:v1.27.1
        name: aws-cloud-controller-manager
        resources:
          requests:
            cpu: 200m
        securityContext: {}
      dnsPolicy: Default
      nodeSelector:
        node-role.kubernetes.io/control-plane: ""
      priorityClassName: system-node-critical
      securityContext: {}
      serviceAccountName: cloud-controller-manager
      tolerations:
      - effect: NoSchedule
        key: node.cloudprovider.kubernetes.io/uninitialized
        value: "true"
      - effect: NoSchedule
        key: node-role.kubernetes.io/master
      - effect: NoSchedule
        key: node-role.kubernetes.io/control-plane
  updateStrategy:
    type: RollingUpdate
`

func Test_generateCPIConfigMapForCluster(t *testing.T) {
	tests := []struct {
		name           string
		startConfigMap *corev1.ConfigMap
		cluster        *clusterv1.Cluster
		key            string
	}{
		{
			name: "Can set cluster name in arguments",
			startConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-cpi-v1.27.1",
					Namespace: "default",
				},
				Data: map[string]string{
					"aws-cpi-v1.27.1.yaml": startAWSCPIConfigMap,
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cool-aws-cluster",
					Namespace: "default",
				},
			},
			key: "aws-cpi-v1.27.1.yaml",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cm, err := generateCPIConfigMapForCluster(
				context.TODO(),
				test.startConfigMap,
				test.cluster,
			)
			if err != nil {
				t.Error("failed to generateCPIConfigMapForCluster", err)
			}
			clusterArg := fmt.Sprintf("--cluster-name=%s", test.cluster.Name)
			if !strings.Contains(cm.Data[test.key], clusterArg) {
				t.Logf(
					"expected %s to contain \n %s. \n Got %s",
					test.startConfigMap.Data[test.key],
					clusterArg,
					cm.Data[test.key],
				)
				t.Fail()
			}
		})
	}
}
