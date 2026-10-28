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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	remotefake "sigs.k8s.io/cluster-api/controllers/remote/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
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
			expected: CiliumLoadBalancerModeValueSNAT,
		},
		{
			name: "config map has no data",
			objects: []ctrlclient.Object{
				ciliumConfigMap(nil),
			},
			expected: CiliumLoadBalancerModeValueSNAT,
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

func TestGetKeysOfNodePortAndLoadBalancerServicesInFNSEntityGroups(t *testing.T) {
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
			name: "ignores services that are not NodePort or LoadBalancer",
			objects: []ctrlclient.Object{
				testEntityGroup("other-types", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "clusterip", "default"),
						serviceDetail("ExternalName", "externalname", "default"),
					},
				}),
			},
		},
		{
			name: "skips services missing name or namespace",
			objects: []ctrlclient.Object{
				testEntityGroup("incomplete", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "", "default"),
						serviceDetail("LoadBalancer", "missing-namespace", ""),
						serviceDetail("NodePort", "", "default"),
						serviceDetail("NodePort", "missing-namespace", ""),
					},
				}),
			},
		},
		{
			name: "returns nodeport and loadbalancer services from every entity group",
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
						serviceDetail("NodePort", "metrics", "monitoring"),
					},
				}),
			},
			want: []ctrlclient.ObjectKey{
				{Namespace: "istio-system", Name: "gateway"},
				{Namespace: "kube-system", Name: "ingress"},
				{Namespace: "monitoring", Name: "metrics"},
			},
		},
		{
			name: "entity group CRD is not installed",
			interceptors: interceptor.Funcs{
				List: func(
					_ context.Context,
					_ ctrlclient.WithWatch,
					_ ctrlclient.ObjectList,
					_ ...ctrlclient.ListOption,
				) error {
					return entityGroupNoKindMatchError()
				},
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
		{
			name: "service type is not a string",
			objects: []ctrlclient.Object{
				testEntityGroup("bad-type", map[string]any{
					"serviceDetails": []any{
						map[string]any{
							"serviceType":      int64(1),
							"serviceName":      "ingress",
							"serviceNamespace": "kube-system",
						},
					},
				}),
			},
			wantErr: `getting serviceType from entity group "bad-type"`,
		},
		{
			name: "service name is not a string",
			objects: []ctrlclient.Object{
				testEntityGroup("bad-name", map[string]any{
					"serviceDetails": []any{
						map[string]any{
							"serviceType":      "LoadBalancer",
							"serviceName":      int64(1),
							"serviceNamespace": "kube-system",
						},
					},
				}),
			},
			wantErr: `getting serviceName from entity group "bad-name"`,
		},
		{
			name: "service namespace is not a string",
			objects: []ctrlclient.Object{
				testEntityGroup("bad-namespace", map[string]any{
					"serviceDetails": []any{
						map[string]any{
							"serviceType":      "LoadBalancer",
							"serviceName":      "ingress",
							"serviceNamespace": int64(1),
						},
					},
				}),
			},
			wantErr: `getting serviceNamespace from entity group "bad-namespace"`,
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

			got, err := getKeysOfNodePortAndLoadBalancerServicesInFNSEntityGroups(t.Context(), remoteClient)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			if len(tt.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFlowNetworkSecurityDSRToSNATCheckerRun(t *testing.T) {
	t.Parallel()

	dsrAnnotations := map[string]string{
		CiliumForwardingModeAnnotationKey: CiliumForwardingModeAnnotationValueDSR,
	}

	failGetWhen := func(shouldFail func(ctrlclient.Object) bool) interceptor.Funcs {
		return interceptor.Funcs{
			Get: func(
				ctx context.Context,
				c ctrlclient.WithWatch,
				key ctrlclient.ObjectKey,
				obj ctrlclient.Object,
				opts ...ctrlclient.GetOption,
			) error {
				if shouldFail(obj) {
					return fmt.Errorf("injected get failure")
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}
	}
	isConfigMap := func(obj ctrlclient.Object) bool {
		_, ok := obj.(*corev1.ConfigMap)
		return ok
	}
	isService := func(obj ctrlclient.Object) bool {
		_, ok := obj.(*corev1.Service)
		return ok
	}

	testCases := []struct {
		name                string
		objects             []ctrlclient.Object
		interceptors        interceptor.Funcs
		clusterClientGetter remote.ClusterClientGetter
		want                preflight.CheckResult
		messageContains     []string
		internalErrorPrefix string
	}{
		{
			name: "cilium config map does not set loadBalancer mode",
			objects: []ctrlclient.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      CiliumConfigMapName,
						Namespace: CiliumConfigMapNamespace,
					},
					Data: map[string]string{"some-other-key": "value"},
				},
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
			name: "entity group CRD is not installed",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
			},
			interceptors: interceptor.Funcs{
				List: func(
					_ context.Context,
					_ ctrlclient.WithWatch,
					_ ctrlclient.ObjectList,
					_ ...ctrlclient.ListOption,
				) error {
					return entityGroupNoKindMatchError()
				},
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "service referenced as LoadBalancer by entity group is now a ClusterIP",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("stale-type", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ingress",
						Namespace: "kube-system",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeClusterIP,
					},
				},
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name:         "error getting cilium config map",
			interceptors: failGetWhen(isConfigMap),
			internalErrorPrefix: "error checking Cilium loadBalancer mode configuration: " +
				"error getting cilium config map: injected get failure",
		},
		{
			name: "error listing entity groups",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
			},
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
			internalErrorPrefix: "error getting NodePort and LoadBalancer Services in FNS entity groups: " +
				"error listing entity groups: injected list failure",
		},
		{
			name: "malformed entity group",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("bad-services", map[string]any{
					"serviceDetails": "not-a-list",
				}),
			},
			internalErrorPrefix: "error getting NodePort and LoadBalancer Services in FNS entity groups: " +
				`getting serviceDetails from entity group "bad-services": `,
		},
		{
			name: "load balancer service referenced by entity group does not exist",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("missing-service", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "error getting load balancer service",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("annotated", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
				testService("kube-system", "ingress", dsrAnnotations),
			},
			interceptors: failGetWhen(isService),
			internalErrorPrefix: "error checking if Service kube-system/ingress requires Cilium DSR annotation: " +
				"error getting service kube-system/ingress: injected get failure",
		},
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
			name: "entity groups do not reference nodeport or load balancer services",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("other-types", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "clusterip", "default"),
						serviceDetail("ExternalName", "externalname", "default"),
					},
				}),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "entity groups reference nodeport and load balancer services with cilium dsr annotations",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("annotated", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
						serviceDetail("LoadBalancer", "gateway", "istio-system"),
						serviceDetail("NodePort", "metrics", "monitoring"),
					},
				}),
				testService("kube-system", "ingress", dsrAnnotations),
				testService("istio-system", "gateway", dsrAnnotations),
				testServiceOfType(corev1.ServiceTypeNodePort, "monitoring", "metrics", dsrAnnotations),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "service referenced as NodePort by entity group is now a ClusterIP",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("stale-type", map[string]any{
					"serviceDetails": []any{
						serviceDetail("NodePort", "metrics", "monitoring"),
					},
				}),
				testServiceOfType(corev1.ServiceTypeClusterIP, "monitoring", "metrics", nil),
			},
			want: preflight.CheckResult{Allowed: true},
		},
		{
			name: "entity groups reference nodeport and load balancer services missing cilium dsr annotations",
			objects: []ctrlclient.Object{
				testCiliumConfigMap(CiliumLoadBalancerModeValueDSR),
				testEntityGroup("missing-annotations", map[string]any{
					"serviceDetails": []any{
						serviceDetail("ClusterIP", "ignored", "default"),
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
						serviceDetail("LoadBalancer", "gateway", "istio-system"),
						serviceDetail("NodePort", "metrics", "monitoring"),
					},
				}),
				testEntityGroup("same-ingress", map[string]any{
					"serviceDetails": []any{
						serviceDetail("LoadBalancer", "ingress", "kube-system"),
					},
				}),
				testService("kube-system", "ingress", nil),
				testService("istio-system", "gateway", nil),
				testService("default", "ignored", nil),
				testServiceOfType(corev1.ServiceTypeNodePort, "monitoring", "metrics", nil),
			},
			messageContains: []string{
				"This cluster is enrolled in Flow Network Security, and must use Direct Server Return (DSR) " +
					"for these NodePort and LoadBalancer Services: " +
					"istio-system/gateway, kube-system/ingress, monitoring/metrics. " +
					"Add the annotation 'service.cilium.io/forwarding-mode: dsr' to the Service resources.",
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
				WithInterceptorFuncs(tt.interceptors).
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
			if tt.internalErrorPrefix != "" {
				assert.False(t, got.Allowed)
				assert.True(t, got.InternalError)
				require.Len(t, got.Causes, 1)
				assert.True(
					t,
					strings.HasPrefix(got.Causes[0].Message, tt.internalErrorPrefix),
					"message %q does not start with %q",
					got.Causes[0].Message,
					tt.internalErrorPrefix,
				)
				return
			}
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

	const (
		oldVersion = "v1.33.0"
		newVersion = "v1.34.0"
	)

	ciliumClusterConfig := `{"addons":{"cni":{"provider":"Cilium"}}}`

	cluster := func(version string, initialized *bool, clusterConfig string) *clusterv1.Cluster {
		c := workloadCluster()
		c.Spec.Topology.Version = version
		c.Status.Initialization.ControlPlaneInitialized = initialized
		if clusterConfig != "" {
			c.Spec.Topology.Variables = []clusterv1.ClusterVariable{
				{
					Name:  carenv1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{Raw: []byte(clusterConfig)},
				},
			}
		}
		return c
	}

	testCases := []struct {
		name                     string
		cd                       *checkDependencies
		wantCheck                bool
		wantInternalErrorMessage string
	}{
		{
			name: "nil dependencies",
		},
		{
			name: "nil kubernetes client",
			cd: &checkDependencies{
				cluster:    cluster(newVersion, new(true), ciliumClusterConfig),
				oldCluster: cluster(oldVersion, new(true), ciliumClusterConfig),
			},
		},
		{
			name: "nil cluster",
			cd: &checkDependencies{
				kclient:    kclient,
				oldCluster: cluster(oldVersion, new(true), ciliumClusterConfig),
			},
		},
		{
			name: "cluster is being created",
			cd: &checkDependencies{
				kclient: kclient,
				cluster: cluster(newVersion, new(true), ciliumClusterConfig),
			},
		},
		{
			name: "control plane initialized is not set",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, nil, ciliumClusterConfig),
				oldCluster: cluster(oldVersion, nil, ciliumClusterConfig),
			},
		},
		{
			name: "control plane is not initialized",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, new(false), ciliumClusterConfig),
				oldCluster: cluster(oldVersion, new(false), ciliumClusterConfig),
			},
		},
		{
			name: "cluster config variable is not set",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, new(true), ""),
				oldCluster: cluster(oldVersion, new(true), ""),
			},
		},
		{
			name: "cni addon is not set",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, new(true), `{"addons":{}}`),
				oldCluster: cluster(oldVersion, new(true), `{"addons":{}}`),
			},
		},
		{
			name: "cni provider is not cilium",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, new(true), `{"addons":{"cni":{"provider":"Calico"}}}`),
				oldCluster: cluster(oldVersion, new(true), `{"addons":{"cni":{"provider":"Calico"}}}`),
			},
		},
		{
			name: "cni provider cannot be read",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, new(true), `{"addons":{"cni":{"provider":1}}}`),
				oldCluster: cluster(oldVersion, new(true), `{"addons":{"cni":{"provider":1}}}`),
			},
			wantInternalErrorMessage: "failed to determine the CNI provider of the cluster, " +
				"which is needed to decide whether the Cilium loadBalancer mode change affects " +
				"Flow Network Security: error getting CNI provider from Cluster Addons configuration: ",
		},
		{
			name: "cilium cluster is being upgraded",
			cd: &checkDependencies{
				kclient:    kclient,
				cluster:    cluster(newVersion, new(true), ciliumClusterConfig),
				oldCluster: cluster(oldVersion, new(true), ciliumClusterConfig),
			},
			wantCheck: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checks := newFlowNetworkSecurityDSRToSNATChecker(tt.cd)
			if tt.wantInternalErrorMessage != "" {
				require.Len(t, checks, 1)
				assert.Equal(t, "NutanixFlowNetworkSecurityDSRToSNAT", checks[0].Name())
				got := checks[0].Run(t.Context())
				assert.False(t, got.Allowed)
				assert.True(t, got.InternalError)
				require.Len(t, got.Causes, 1)
				assert.True(
					t,
					strings.HasPrefix(got.Causes[0].Message, tt.wantInternalErrorMessage),
					"message %q does not start with %q",
					got.Causes[0].Message,
					tt.wantInternalErrorMessage,
				)
				return
			}
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

func entityGroupNoKindMatchError() error {
	return &meta.NoKindMatchError{
		GroupKind: schema.GroupKind{
			Group: FlowNetworkSecurityGroupVersion.Group,
			Kind:  "EntityGroup",
		},
		SearchedVersions: []string{FlowNetworkSecurityGroupVersion.Version},
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
	return testServiceOfType(corev1.ServiceTypeLoadBalancer, namespace, name, annotations)
}

func testServiceOfType(
	serviceType corev1.ServiceType,
	namespace, name string,
	annotations map[string]string,
) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
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
