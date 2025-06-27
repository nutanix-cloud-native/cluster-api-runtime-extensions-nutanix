// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package syncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
)

func Test_shouldApplyRegistrySyncer(t *testing.T) {
	// Pre-create a cluster so it can be used in a test case that requires the same name and namespace.
	clusterWithSameNameAndNamespace := clusterWithRegistry(t)
	tests := []struct {
		name              string
		cluster           *clusterv1.Cluster
		managementCluster *clusterv1.Cluster
		enableFeatureGate bool
		shouldApply       bool
	}{
		{
			name:              "should apply",
			cluster:           clusterWithRegistry(t),
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: true,
			shouldApply:       true,
		},
		{
			name:              "should not apply when management cluster is nil",
			cluster:           clusterWithRegistry(t),
			managementCluster: nil,
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when management cluster name and namespace matches cluster",
			cluster:           clusterWithSameNameAndNamespace,
			managementCluster: clusterWithSameNameAndNamespace,
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when feature gate is disabled",
			cluster:           clusterWithRegistry(t),
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: false,
			shouldApply:       false,
		},
		{
			name: "should not apply when cluster has skip annotation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						carenv1.SkipSynchronizingWorkloadClusterRegistry: "true",
					},
				},
			},
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when cluster does not have registry enabled",
			cluster:           clusterWithoutRegistry(t),
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when management cluster does not have registry enabled",
			cluster:           clusterWithRegistry(t),
			managementCluster: clusterWithoutRegistry(t),
			enableFeatureGate: true,
			shouldApply:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enableFeatureGate(t, tt.enableFeatureGate)
			shouldApply, err := shouldApplyRegistrySyncer(tt.cluster, tt.managementCluster)
			require.NoError(t, err)
			assert.Equal(t, tt.shouldApply, shouldApply)
		})
	}
}

func Test_templateValues(t *testing.T) {
	result, err := templateValues(namedClusterWithRegistry(t, "test-cluster"), testRegistrySyncerTemplate)
	require.NoError(t, err)
	assert.Equal(t, expectedRegistrySyncerTemplate, result)
}

func clusterWithRegistry(t *testing.T) *clusterv1.Cluster {
	t.Helper()

	return namedClusterWithRegistry(t, names.SimpleNameGenerator.GenerateName("with-registry-"))
}

func namedClusterWithRegistry(t *testing.T, name string) *clusterv1.Cluster {
	t.Helper()

	clusterConfigSpec := &carenv1.DockerClusterConfigSpec{
		Addons: &carenv1.DockerAddons{
			GenericAddons: carenv1.GenericAddons{
				CNI:      &carenv1.CNI{},
				Registry: &carenv1.RegistryAddon{},
			},
		},
	}
	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	require.NoError(t, err)
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			Topology: &clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{
					*variable,
				},
			},
		},
	}
}

func clusterWithoutRegistry(t *testing.T) *clusterv1.Cluster {
	t.Helper()

	clusterConfigSpec := &carenv1.DockerClusterConfigSpec{
		Addons: &carenv1.DockerAddons{
			GenericAddons: carenv1.GenericAddons{
				CNI: &carenv1.CNI{},
			},
		},
	}
	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	require.NoError(t, err)
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.SimpleNameGenerator.GenerateName("without-registry-"),
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			Topology: &clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{
					*variable,
				},
			},
		},
	}
}

func enableFeatureGate(t *testing.T, value bool) {
	t.Helper()

	featuregatetesting.SetFeatureGateDuringTest(
		t,
		feature.Gates,
		feature.SynchronizeWorkloadClusterRegistry,
		value,
	)
}

const (
	testRegistrySyncerTemplate = `initContainers:
  # The regsync container does not fail when it cannot connect to the destination registry.
  # In the case when it runs as a Job, it will prematurely exit.
  # This init container will wait for the destination registry to be ready.
  - name: wait-for-registry
    image: ghcr.io/d2iq-labs/kubectl-betterwait:{{ .KubernetesVersion }}
    args:
      - --for=condition=Ready
      - --timeout=-1s # a negative number here means wait forever
      - --interval=5s # poll every 5 seconds to the resources to be created
      - --namespace={{ .DestinationRegistryHeadlessServiceNamespace }}
      - --kubeconfig=/kubeconfig/admin.conf
      # Ideally we would wait for the Service to be ready, but Kubernetes does not have a condition for that.
      - pod/{{ .DestinationRegistryAnyPodName }}
    volumeMounts:
      - mountPath: /kubeconfig
        name: kubeconfig
        readOnly: true
  - name: port-forward-registry
    image: ghcr.io/d2iq-labs/kubectl-betterwait:{{ .KubernetesVersion }}
    command:
      - /bin/kubectl
    args:
      - port-forward
      - --address=127.0.0.1
      - --namespace={{ .DestinationRegistryHeadlessServiceNamespace }}
      - --kubeconfig=/kubeconfig/admin.conf
      # This will port-forward to a single Pod in the Service.
      - service/{{ .DestinationRegistryHeadlessServiceName }}
      - 5000:{{ .DestinationRegistryHeadlessServicePort }}
    resources:
      requests:
        cpu: 25m
        memory: 32Mi
      limits:
        cpu: 100m
        memory: 50Mi
    volumeMounts:
      - mountPath: /kubeconfig
        name: kubeconfig
        readOnly: true
    # Kubernetes will treat this as a Sidecar container
    # https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/
    restartPolicy: Always

extraVolumes:
  - name: kubeconfig
    secret:
      items:
        - key: value
          path: admin.conf
      secretName: {{ .CusterName }}-kubeconfig
  - name: ca-cert
    secret:
      secretName: {{ .RegistryCASecretName }}

extraVolumeMounts:
  # Assume both the source and the target registries have the same CA.
  # Source registry running in the cluster.
  - mountPath: /etc/docker/certs.d/{{ .SourceRegistryAddress }}/
    name: ca-cert
    readOnly: true
  # Destination registry running in the remote cluster being port-forwarded.
  - mountPath: /etc/docker/certs.d/127.0.0.1:5000/
    name: ca-cert
    readOnly: true

deployment:
  config:
    creds:
      - registry: {{ .SourceRegistryAddress }}
        reqPerSec: 1
    sync:
      - source: {{ .SourceRegistryAddress }}
        target: 127.0.0.1:5000
        type: registry
        interval: 1m

job:
  enabled: true
  config:
    sync:
      - source: {{ .SourceRegistryAddress }}
        target: 127.0.0.1:5000
        type: registry
        interval: 1m`

	expectedRegistrySyncerTemplate = `initContainers:
  # The regsync container does not fail when it cannot connect to the destination registry.
  # In the case when it runs as a Job, it will prematurely exit.
  # This init container will wait for the destination registry to be ready.
  - name: wait-for-registry
    image: ghcr.io/d2iq-labs/kubectl-betterwait:
    args:
      - --for=condition=Ready
      - --timeout=-1s # a negative number here means wait forever
      - --interval=5s # poll every 5 seconds to the resources to be created
      - --namespace=registry-system
      - --kubeconfig=/kubeconfig/admin.conf
      # Ideally we would wait for the Service to be ready, but Kubernetes does not have a condition for that.
      - pod/cncf-distribution-registry-docker-registry-0
    volumeMounts:
      - mountPath: /kubeconfig
        name: kubeconfig
        readOnly: true
  - name: port-forward-registry
    image: ghcr.io/d2iq-labs/kubectl-betterwait:
    command:
      - /bin/kubectl
    args:
      - port-forward
      - --address=127.0.0.1
      - --namespace=registry-system
      - --kubeconfig=/kubeconfig/admin.conf
      # This will port-forward to a single Pod in the Service.
      - service/cncf-distribution-registry-docker-registry-headless
      - 5000:5000
    resources:
      requests:
        cpu: 25m
        memory: 32Mi
      limits:
        cpu: 100m
        memory: 50Mi
    volumeMounts:
      - mountPath: /kubeconfig
        name: kubeconfig
        readOnly: true
    # Kubernetes will treat this as a Sidecar container
    # https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/
    restartPolicy: Always

extraVolumes:
  - name: kubeconfig
    secret:
      items:
        - key: value
          path: admin.conf
      secretName: test-cluster-kubeconfig
  - name: ca-cert
    secret:
      secretName: test-cluster-registry-addon-ca

extraVolumeMounts:
  # Assume both the source and the target registries have the same CA.
  # Source registry running in the cluster.
  - mountPath: /etc/docker/certs.d/cncf-distribution-registry-docker-registry.registry-system.svc.cluster.local:443/
    name: ca-cert
    readOnly: true
  # Destination registry running in the remote cluster being port-forwarded.
  - mountPath: /etc/docker/certs.d/127.0.0.1:5000/
    name: ca-cert
    readOnly: true

deployment:
  config:
    creds:
      - registry: cncf-distribution-registry-docker-registry.registry-system.svc.cluster.local:443
        reqPerSec: 1
    sync:
      - source: cncf-distribution-registry-docker-registry.registry-system.svc.cluster.local:443
        target: 127.0.0.1:5000
        type: registry
        interval: 1m

job:
  enabled: true
  config:
    sync:
      - source: cncf-distribution-registry-docker-registry.registry-system.svc.cluster.local:443
        target: 127.0.0.1:5000
        type: registry
        interval: 1m`
)
