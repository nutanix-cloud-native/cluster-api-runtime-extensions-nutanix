// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	remotefake "sigs.k8s.io/cluster-api/controllers/remote/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestGetCiliumLoadBalancerMode(t *testing.T) {
	ciliumConfigMap := func(data map[string]string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CiliumConfigMapName,
				Namespace: CiliumConfigMapNamespace,
			},
			Data: data,
		}
	}

	tests := []struct {
		name          string
		objects       []ctrlclient.Object
		interceptors  interceptor.Funcs
		expected      string
		expectedError string
	}{
		{
			name: "mode is dsr",
			objects: []ctrlclient.Object{
				ciliumConfigMap(map[string]string{CiliumLoadBalancerModeKey: CiliumLoadBalancerModeValueDSR}),
			},
			expected: CiliumLoadBalancerModeValueDSR,
		},
		{
			name: "mode is snat",
			objects: []ctrlclient.Object{
				ciliumConfigMap(map[string]string{CiliumLoadBalancerModeKey: "snat"}),
			},
			expected: "snat",
		},
		{
			name: "mode key is not set",
			objects: []ctrlclient.Object{
				ciliumConfigMap(map[string]string{"some-other-key": "value"}),
			},
			expectedError: CiliumLoadBalancerModeKey + " is not set in " + CiliumConfigMapName + " config map",
		},
		{
			name: "config map has no data",
			objects: []ctrlclient.Object{
				ciliumConfigMap(nil),
			},
			expectedError: CiliumLoadBalancerModeKey + " is not set in " + CiliumConfigMapName + " config map",
		},
		{
			name: "config map is in a different namespace",
			objects: []ctrlclient.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      CiliumConfigMapName,
						Namespace: "default",
					},
					Data: map[string]string{CiliumLoadBalancerModeKey: CiliumLoadBalancerModeValueDSR},
				},
			},
			expectedError: "error getting cilium config map",
		},
		{
			name:          "config map does not exist",
			expectedError: "error getting cilium config map",
		},
		{
			name: "error getting config map",
			interceptors: interceptor.Funcs{
				Get: func(
					_ context.Context,
					_ ctrlclient.WithWatch,
					_ ctrlclient.ObjectKey,
					_ ctrlclient.Object,
					_ ...ctrlclient.GetOption,
				) error {
					return fmt.Errorf("fake error")
				},
			},
			expectedError: "error getting cilium config map: fake error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remoteClient := fake.NewClientBuilder().
				WithObjects(tt.objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()

			mode, err := getCiliumLoadBalancerMode(context.Background(), remoteClient)
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, mode)
		})
	}
}

func TestGetKeysOfLoadBalancerServicesInFNSEntityGroups(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		objects      []ctrlclient.Object
		interceptors interceptor.Funcs
		want         []ctrlclient.ObjectKey
		wantErr      string
	}{
		{
			name: "no entity groups",
		},
		{
			name: "entity group without status",
			objects: []ctrlclient.Object{
				testEntityGroup("no-status", nil),
			},
		},
		{
			name: "entity group with empty status",
			objects: []ctrlclient.Object{
				testEntityGroup("empty-status", map[string]any{}),
			},
		},
		{
			name: "status without service details",
			objects: []ctrlclient.Object{
				testEntityGroup("no-services", map[string]any{
					"lastUpdatedTimestamp": int64(1),
				}),
			},
		},
		{
			name: "empty service details",
			objects: []ctrlclient.Object{
				testEntityGroup("empty-services", map[string]any{
					"serviceDetails": []any{},
				}),
			},
		},
		{
			name: "ignores non-loadbalancer services",
			objects: []ctrlclient.Object{
				testEntityGroup("other-types", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "clusterip", "default"),
						serviceDetail("NodePort", "nodeport", "default"),
					},
				}),
			},
		},
		{
			name: "skips loadbalancer services missing name or namespace",
			objects: []ctrlclient.Object{
				testEntityGroup("incomplete", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "", "default"),
						serviceDetail("LoadBalancer", "missing-namespace", ""),
					},
				}),
			},
		},
		{
			name: "returns loadbalancer services from every entity group",
			objects: []ctrlclient.Object{
				testEntityGroup("group-a", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "ignored", "default"),
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
				testEntityGroup("group-b", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "gateway", "istio-system"),
					},
				}),
			},
			want: []ctrlclient.ObjectKey{
				{Namespace: "kube-system", Name: "ingress"},
				{Namespace: "istio-system", Name: "gateway"},
			},
		},
		{
			name: "list error",
			interceptors: interceptor.Funcs{
				List: func(
					_ context.Context,
					_ ctrlclient.WithWatch,
					_ ctrlclient.ObjectList,
					_ ...ctrlclient.ListOption,
				) error {
					return fmt.Errorf("injected list failure")
				},
			},
			wantErr: "listing entity groups",
		},
		{
			name: "status is not an object",
			objects: []ctrlclient.Object{
				testEntityGroup("bad-status", "not-an-object"),
			},
			wantErr: `getting status from entity group "bad-status"`,
		},
		{
			name: "service details is not a list",
			objects: []ctrlclient.Object{
				testEntityGroup("bad-services", map[string]any{
					"serviceDetails": "not-a-list",
				}),
			},
			wantErr: `getting serviceDetails from entity group "bad-services"`,
		},
		{
			name: "service details entry is not an object",
			objects: []ctrlclient.Object{
				testEntityGroup("bad-entry", map[string]any{
					"serviceDetails": []any{"not-an-object"},
				}),
			},
			wantErr: `serviceDetails entry in entity group "bad-entry" is string, expected object`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A private scheme keeps List from registering the EntityGroup GVK on the
			// process-wide scheme while other subtests read it.
			builder := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithObjects(tt.objects...)
			if tt.interceptors.List != nil {
				builder = builder.WithInterceptorFuncs(tt.interceptors)
			}
			remoteClient := builder.Build()

			got, err := getKeysOfLoadBalancerServicesInFNSEntityGroups(t.Context(), remoteClient)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestFlowNetworkSecurityDSRToSNATCheckerRun(t *testing.T) {
	t.Parallel()

	dsrAnnotations := map[string]string{
		"service.cilium.io/type": "LoadBalancer",
		"service.cilium.io/dsr":  "true",
	}

	testCases := []struct {
		name                string
		objects             []ctrlclient.Object
		clusterClientGetter remote.ClusterClientGetter
		want                preflight.CheckResult
		messageContains     []string
	}{
		{
			name: "error creating remote cluster client",
			clusterClientGetter: func(
				_ context.Context,
				_ string,
				_ ctrlclient.Client,
				_ ctrlclient.ObjectKey,
			) (ctrlclient.Client, error) {
				return nil, fmt.Errorf("injected client failure")
			},
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "error creating remote cluster client: injected client failure",
					},
				},
			},
		},
		{
			name: "cilium loadBalancer mode is already snat",
			objects: []ctrlclient.Object{
				testCiliumConfigMap("snat"),
				testEntityGroup("missing-annotations", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
				testService("kube-system", "ingress", nil),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "no entity groups",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "entity groups do not reference load balancer services",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("other-types", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "clusterip", "default"),
						serviceDetail("NodePort", "nodeport", "default"),
					},
				}),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "entity groups reference load balancer services with cilium dsr annotations",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("annotated", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
						serviceDetail("LoadBalancer", "gateway", "istio-system"),
					},
				}),
				testService("kube-system", "ingress", dsrAnnotations),
				testService("istio-system", "gateway", dsrAnnotations),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "entity groups reference load balancer services missing cilium dsr annotations",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("missing-annotations", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "ignored", "default"),
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
						serviceDetail("LoadBalancer", "gateway", "istio-system"),
					},
				}),
				testEntityGroup("same-ingress", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
				testService("kube-system", "ingress", nil),
				testService("istio-system", "gateway", map[string]string{
					"service.cilium.io/type": "LoadBalancer",
				}),
				testService("default", "ignored", nil),
			},
			messageContains: []string{
				"kube-system/ingress",
				"istio-system/gateway",
				"(service.cilium.io/type: LoadBalancer)",
				"(service.cilium.io/dsr: true)",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A private scheme keeps List from registering the EntityGroup GVK on the
			// process-wide scheme while other subtests read it.
			scheme := runtime.NewScheme()
			utilruntime.Must(corev1.AddToScheme(scheme))
			remoteClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			clusterClientGetter := tt.clusterClientGetter
			if clusterClientGetter == nil {
				clusterClientGetter = remotefake.NewClusterClient
			}

			checker := &FlowNetworkSecurityDSRToSNATChecker{
				kclient:             remoteClient,
				cluster:             workloadCluster(),
				clusterClientGetter: clusterClientGetter,
			}

			got := checker.Run(t.Context())
			if len(tt.messageContains) == 0 {
				assert.Equal(t, tt.want, got)
				return
			}

			assert.False(t, got.Allowed)
			assert.False(t, got.InternalError)
			require.Len(t, got.Causes, 1)
			for _, fragment := range tt.messageContains {
				assert.Contains(t, got.Causes[0].Message, fragment)
			}
			assert.NotContains(t, got.Causes[0].Message, "default/ignored")
			// Two EntityGroups reference kube-system/ingress. The check reports it once.
			assert.Equal(t, 1, strings.Count(got.Causes[0].Message, "kube-system/ingress"))
		})
	}
}

func TestNewFlowNetworkSecurityDSRToSNATChecker(t *testing.T) {
	t.Parallel()

	kclient := fake.NewClientBuilder().Build()

	clusterWithControlPlaneInitialized := func(initialized *bool) *clusterv1.Cluster {
		cluster := workloadCluster()
		cluster.Status.Initialization.ControlPlaneInitialized = initialized
		return cluster
	}

	testCases := []struct {
		name      string
		cd        *checkDependencies
		wantCheck bool
	}{
		{
			name: "nil dependencies",
		},
		{
			name: "nil kubernetes client",
			cd: &checkDependencies{
				cluster: clusterWithControlPlaneInitialized(ptr.To(true)),
			},
		},
		{
			name: "nil cluster",
			cd: &checkDependencies{
				kclient: kclient,
			},
		},
		{
			name: "control plane initialized is not set",
			cd: &checkDependencies{
				kclient: kclient,
				cluster: clusterWithControlPlaneInitialized(nil),
			},
		},
		{
			name: "control plane is not initialized",
			cd: &checkDependencies{
				kclient: kclient,
				cluster: clusterWithControlPlaneInitialized(ptr.To(false)),
			},
		},
		{
			name: "control plane is initialized",
			cd: &checkDependencies{
				kclient: kclient,
				cluster: clusterWithControlPlaneInitialized(ptr.To(true)),
			},
			wantCheck: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checks := newFlowNetworkSecurityDSRToSNATChecker(tt.cd)
			if !tt.wantCheck {
				assert.Empty(t, checks)
				return
			}

			require.Len(t, checks, 1)
			checker, ok := checks[0].(*FlowNetworkSecurityDSRToSNATChecker)
			require.True(t, ok, "check is %T, expected *FlowNetworkSecurityDSRToSNATChecker", checks[0])
			assert.Equal(t, "NutanixFlowNetworkSecurityDSRToSNAT", checker.Name())
			assert.Same(t, tt.cd.kclient, checker.kclient)
			assert.Same(t, tt.cd.cluster, checker.cluster)
			assert.NotNil(t, checker.clusterClientGetter)
		})
	}
}

func workloadCluster() *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workload",
			Namespace: "default",
		},
	}
}

func testCiliumConfigMap(mode string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CiliumConfigMapName,
			Namespace: CiliumConfigMapNamespace,
		},
		Data: map[string]string{
			CiliumLoadBalancerModeKey: mode,
		},
	}
}

func testService(namespace, name string, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}

func testEntityGroup(name string, status any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   FlowNetworkSecurityGroupVersion.Group,
		Version: FlowNetworkSecurityGroupVersion.Version,
		Kind:    "EntityGroup",
	})
	obj.SetName(name)
	if status != nil {
		obj.Object["status"] = status
	}
	return obj
}

func serviceDetail(serviceType, name, namespace string) map[string]any {
	detail := map[string]any{
		"serviceType": serviceType,
	}
	if name != "" {
		detail["serviceName"] = name
	}
	if namespace != "" {
		detail["serviceNamespace"] = namespace
	}
	return detail
}
