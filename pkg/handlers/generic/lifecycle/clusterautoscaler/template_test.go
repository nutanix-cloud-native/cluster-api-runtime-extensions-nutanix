// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_templateData(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1.Cluster
		data    map[string]string
		want    map[string]string
	}{
		{
			name: "template data",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			data: map[string]string{
				mapKey: testDeployment,
			},
			want: map[string]string{
				mapKey: templatedDeployment,
			},
		},
		{
			name: "no data to template",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			data: map[string]string{
				mapKey: templatedDeployment,
			},
			want: map[string]string{
				mapKey: templatedDeployment,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := templateData(tt.cluster, tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_templateValues(t *testing.T) {
	tests := []struct {
		name    string
		cluster *clusterv1.Cluster
		text    string
		want    string
	}{
		{
			name: "template values",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			text: testValues,
			want: templatedValues,
		},
		{
			name: "no values to template",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			text: templatedValues,
			want: templatedValues,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := templateValues(tt.cluster, tt.text)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

const (
	mapKey = "deployment.yaml"

	testDeployment = `---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: cluster-autoscaler-tmpl-clustername-tmpl
      namespace: tmpl-clusternamespace-tmpl
    spec:
      replicas: 1
      revisionHistoryLimit: 10
      selector:
        matchLabels:
          app.kubernetes.io/instance: cluster-autoscaler-tmpl-clustername-tmpl
          app.kubernetes.io/name: clusterapi-cluster-autoscaler
      template:
        metadata:
          labels:
            app.kubernetes.io/instance: cluster-autoscaler-tmpl-clustername-tmpl
            app.kubernetes.io/name: clusterapi-cluster-autoscaler
        spec:
          containers:
          - command:
            - ./cluster-autoscaler
            - --cloud-provider=clusterapi
            - --namespace=tmpl-clusternamespace-tmpl
            - --node-group-auto-discovery=clusterapi:clusterName=tmpl-clustername-tmpl
            - --kubeconfig=/cluster/kubeconfig
            - --clusterapi-cloud-config-authoritative
            - --enforce-node-group-min-size=true
            - --logtostderr=true
            - --stderrthreshold=info
            - --v=4
            name: clusterapi-cluster-autoscaler
            volumeMounts:
            - mountPath: /cluster
              name: kubeconfig
              readOnly: true
          serviceAccountName: cluster-autoscaler-tmpl-clustername-tmpl
          volumes:
          - name: kubeconfig
            secret:
              items:
              - key: value
                path: kubeconfig
              secretName: tmpl-clustername-tmpl-kubeconfig`

	templatedDeployment = `---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: cluster-autoscaler-test-cluster
      namespace: test-namespace
    spec:
      replicas: 1
      revisionHistoryLimit: 10
      selector:
        matchLabels:
          app.kubernetes.io/instance: cluster-autoscaler-test-cluster
          app.kubernetes.io/name: clusterapi-cluster-autoscaler
      template:
        metadata:
          labels:
            app.kubernetes.io/instance: cluster-autoscaler-test-cluster
            app.kubernetes.io/name: clusterapi-cluster-autoscaler
        spec:
          containers:
          - command:
            - ./cluster-autoscaler
            - --cloud-provider=clusterapi
            - --namespace=test-namespace
            - --node-group-auto-discovery=clusterapi:clusterName=test-cluster
            - --kubeconfig=/cluster/kubeconfig
            - --clusterapi-cloud-config-authoritative
            - --enforce-node-group-min-size=true
            - --logtostderr=true
            - --stderrthreshold=info
            - --v=4
            name: clusterapi-cluster-autoscaler
            volumeMounts:
            - mountPath: /cluster
              name: kubeconfig
              readOnly: true
          serviceAccountName: cluster-autoscaler-test-cluster
          volumes:
          - name: kubeconfig
            secret:
              items:
              - key: value
                path: kubeconfig
              secretName: test-cluster-kubeconfig`

	testValues = `    ---
    fullnameOverride: "cluster-autoscaler-{{ .Cluster.Name }}"

    cloudProvider: clusterapi

    # Always trigger a scale-out if replicas are less than the min.
    extraArgs:
      enforce-node-group-min-size: true

    # Enable it to run in a 1 Node cluster.
    tolerations:
      - effect: NoSchedule
        key: node-role.kubernetes.io/control-plane

    # Limit a single cluster-autoscaler Deployment to a single Cluster.
    autoDiscovery:
      clusterName: "{{ .Cluster.Name }}"
      # The controller failed with an RBAC error trying to watch CAPI objects at the cluster scope without this.
      labels:
        - namespace: "{{ .Cluster.Namespace }}"

    clusterAPIConfigMapsNamespace: "{{ .Cluster.Namespace }}"
    # For workload clusters it is not possible to use the in-cluster client.
    # To simplify the configuration, use the admin kubeconfig generated by CAPI for all clusters.
    clusterAPIMode: kubeconfig-incluster
    clusterAPIWorkloadKubeconfigPath: /cluster/kubeconfig
    extraVolumeSecrets:
      kubeconfig:
        name: "{{ .Cluster.Name }}-kubeconfig"
        mountPath: /cluster
        readOnly: true
        items:
          - key: value
            path: kubeconfig
    rbac:
      # Create a Role instead of a ClusterRoles to update cluster-api objects
      clusterScoped: false`

	templatedValues = `    ---
    fullnameOverride: "cluster-autoscaler-test-cluster"

    cloudProvider: clusterapi

    # Always trigger a scale-out if replicas are less than the min.
    extraArgs:
      enforce-node-group-min-size: true

    # Enable it to run in a 1 Node cluster.
    tolerations:
      - effect: NoSchedule
        key: node-role.kubernetes.io/control-plane

    # Limit a single cluster-autoscaler Deployment to a single Cluster.
    autoDiscovery:
      clusterName: "test-cluster"
      # The controller failed with an RBAC error trying to watch CAPI objects at the cluster scope without this.
      labels:
        - namespace: "test-namespace"

    clusterAPIConfigMapsNamespace: "test-namespace"
    # For workload clusters it is not possible to use the in-cluster client.
    # To simplify the configuration, use the admin kubeconfig generated by CAPI for all clusters.
    clusterAPIMode: kubeconfig-incluster
    clusterAPIWorkloadKubeconfigPath: /cluster/kubeconfig
    extraVolumeSecrets:
      kubeconfig:
        name: "test-cluster-kubeconfig"
        mountPath: /cluster
        readOnly: true
        items:
          - key: value
            path: kubeconfig
    rbac:
      # Create a Role instead of a ClusterRoles to update cluster-api objects
      clusterScoped: false`
)
