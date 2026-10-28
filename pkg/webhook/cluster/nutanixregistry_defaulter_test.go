// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"testing"

	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

// clusterConfigVariable marshals a ClusterConfigSpec into the clusterConfig topology variable.
func clusterConfigVariable(g *WithT, spec *variables.ClusterConfigSpec) clusterv1.ClusterVariable {
	v, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, spec)
	g.Expect(err).ToNot(HaveOccurred())
	return *v
}

// dockerHubNutanixEntry returns the docker.io/nutanix imageRegistries entry from the cluster's
// clusterConfig topology variable, or nil if absent.
func dockerHubNutanixEntry(g *WithT, cluster *clusterv1.Cluster) *carenv1.ImageRegistry {
	cfg, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg).ToNot(BeNil())
	for i := range cfg.ImageRegistries {
		if cfg.ImageRegistries[i].URL == nutanixRegistryURL {
			return &cfg.ImageRegistries[i]
		}
	}
	return nil
}

// stampManagementNode creates a Node carrying the CAPI annotations that the management cluster's
// own Machines stamp onto its Nodes, so the defaulter can resolve the management cluster. It also
// waits until the manager cache (shared with the webhook) observes it.
func stampManagementNode(t *testing.T, g *WithT, mgmtName, mgmtNamespace string) {
	t.Helper()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mgmt-node-",
			Annotations: map[string]string{
				clusterv1.ClusterNameAnnotation:      mgmtName,
				clusterv1.ClusterNamespaceAnnotation: mgmtNamespace,
			},
		},
	}
	g.Expect(env.Client.Create(ctx, node)).To(Succeed())
	t.Cleanup(func() { g.Expect(env.Client.Delete(ctx, node)).To(Succeed()) })
}

func TestDockerHubNutanixDefaulter_InheritsFromManagementCluster(t *testing.T) {
	g := NewWithT(t)

	const mgmtName = "dockerhub-mgmt-cluster"
	mgmtNamespace := metav1.NamespaceDefault

	stampManagementNode(t, g, mgmtName, mgmtNamespace)

	// The management PAT is read from the management cluster's own per-cluster
	// Secret name: <mgmt>-dockerhub-nutanix-credentials.
	mgmtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nutanixRegistrySecretName(mgmtName),
			Namespace: mgmtNamespace,
		},
		StringData: map[string]string{
			nutanixRegistryUsernameKey: "mgmt-user",
			nutanixRegistryPasswordKey: "mgmt-pass",
		},
		Type: corev1.SecretTypeOpaque,
	}
	g.Expect(env.Client.Create(ctx, mgmtSecret)).To(Succeed())
	t.Cleanup(func() { g.Expect(env.Client.Delete(ctx, mgmtSecret)).To(Succeed()) })

	// Ensure the manager cache (shared with the webhook) has observed the node and the management
	// Secret before we create the workload Cluster; otherwise the synchronous webhook could race.
	g.Eventually(func(g Gomega) {
		nodes := &corev1.NodeList{}
		g.Expect(env.Client.List(ctx, nodes)).To(Succeed())
		g.Expect(nodes.Items).ToNot(BeEmpty())
		g.Expect(
			env.Client.Get(ctx, client.ObjectKeyFromObject(mgmtSecret), &corev1.Secret{}),
		).To(Succeed())
	}).Should(Succeed())

	workload := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workload-inherit",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "nkp-nutanix-v2.19.0"},
				Version:   "dummy-version",
				Variables: []clusterv1.ClusterVariable{clusterConfigVariable(g, &variables.ClusterConfigSpec{})},
			},
		},
	}
	g.Expect(env.Client.Create(ctx, workload)).To(Succeed())
	t.Cleanup(func() { g.Expect(env.Client.Delete(ctx, workload)).To(Succeed()) })

	// The webhook should have appended the docker.io/nutanix entry referencing the per-cluster Secret.
	expectedSecretName := nutanixRegistrySecretName(workload.Name)
	entry := dockerHubNutanixEntry(g, workload)
	g.Expect(entry).ToNot(BeNil())
	g.Expect(entry.Credentials).ToNot(BeNil())
	g.Expect(entry.Credentials.SecretRef).ToNot(BeNil())
	g.Expect(entry.Credentials.SecretRef.Name).To(Equal(expectedSecretName))

	// The per-cluster Secret should have been created with the inherited credentials.
	created := &corev1.Secret{}
	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			types.NamespacedName{Namespace: workload.Namespace, Name: expectedSecretName},
			created,
		)
	}).Should(Succeed())
	g.Expect(created.Data).To(HaveKeyWithValue(nutanixRegistryUsernameKey, []byte("mgmt-user")))
	g.Expect(created.Data).To(HaveKeyWithValue(nutanixRegistryPasswordKey, []byte("mgmt-pass")))
	g.Expect(created.Labels).To(HaveKeyWithValue(clusterv1.ClusterNameLabel, workload.Name))
}

func TestDockerHubNutanixDefaulter_PreservesExplicitEntry(t *testing.T) {
	g := NewWithT(t)

	// A management node/secret is present, but the workload already declares an explicit
	// docker.io/nutanix entry, so the defaulter must be a no-op and never overwrite it.
	const mgmtName = "dockerhub-mgmt-cluster-explicit"
	stampManagementNode(t, g, mgmtName, metav1.NamespaceDefault)

	mgmtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nutanixRegistrySecretName(mgmtName),
			Namespace: metav1.NamespaceDefault,
		},
		StringData: map[string]string{
			nutanixRegistryUsernameKey: "mgmt-user",
			nutanixRegistryPasswordKey: "mgmt-pass",
		},
		Type: corev1.SecretTypeOpaque,
	}
	g.Expect(env.Client.Create(ctx, mgmtSecret)).To(Succeed())
	t.Cleanup(func() { g.Expect(env.Client.Delete(ctx, mgmtSecret)).To(Succeed()) })

	const workloadName = "workload-explicit"
	explicitSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nutanixRegistrySecretName(workloadName),
			Namespace: metav1.NamespaceDefault,
		},
		StringData: map[string]string{
			nutanixRegistryUsernameKey: "explicit-user",
			nutanixRegistryPasswordKey: "explicit-pass",
		},
		Type: corev1.SecretTypeOpaque,
	}
	g.Expect(env.Client.Create(ctx, explicitSecret)).To(Succeed())
	t.Cleanup(func() { g.Expect(env.Client.Delete(ctx, explicitSecret)).To(Succeed()) })

	workload := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workloadName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "nkp-nutanix-v2.19.0"},
				Version:  "dummy-version",
				Variables: []clusterv1.ClusterVariable{clusterConfigVariable(g, &variables.ClusterConfigSpec{
					GenericClusterConfigSpec: carenv1.GenericClusterConfigSpec{
						ImageRegistries: []carenv1.ImageRegistry{{
							URL: nutanixRegistryURL,
							Credentials: &carenv1.RegistryCredentials{
								SecretRef: &carenv1.LocalObjectReference{Name: explicitSecret.Name},
							},
						}},
					},
				})},
			},
		},
	}
	g.Expect(env.Client.Create(ctx, workload)).To(Succeed())
	t.Cleanup(func() { g.Expect(env.Client.Delete(ctx, workload)).To(Succeed()) })

	// The explicit entry must be preserved verbatim.
	entry := dockerHubNutanixEntry(g, workload)
	g.Expect(entry).ToNot(BeNil())
	g.Expect(entry.Credentials.SecretRef.Name).To(Equal(explicitSecret.Name))

	// Existing per-cluster credentials must not be overwritten with management credentials.
	preserved := &corev1.Secret{}
	g.Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(explicitSecret), preserved)).To(Succeed())
	g.Expect(preserved.Data).To(HaveKeyWithValue(nutanixRegistryUsernameKey, []byte("explicit-user")))
	g.Expect(preserved.Data).To(HaveKeyWithValue(nutanixRegistryPasswordKey, []byte("explicit-pass")))
}

func TestDockerHubNutanixDefaulter_UpdateCreatesMissingInheritedSecret(t *testing.T) {
	g := NewWithT(t)

	const mgmtName = "dockerhub-mgmt-cluster-update"
	mgmtNamespace := metav1.NamespaceDefault
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mgmt-node-update",
			Annotations: map[string]string{
				clusterv1.ClusterNameAnnotation:      mgmtName,
				clusterv1.ClusterNamespaceAnnotation: mgmtNamespace,
			},
		},
	}
	mgmtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nutanixRegistrySecretName(mgmtName),
			Namespace: mgmtNamespace,
		},
		Data: map[string][]byte{
			nutanixRegistryUsernameKey: []byte("mgmt-user"),
			nutanixRegistryPasswordKey: []byte("mgmt-pass"),
		},
		Type: corev1.SecretTypeOpaque,
	}

	workload := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workload-update-inherit",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			// This simulates an NKPCluster topology that already contains the inherited entry
			// after dry-run defaulting, while dry-run admission could not create its Secret.
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "nkp-nutanix-v2.19.0"},
				Version:  "dummy-version",
				Variables: []clusterv1.ClusterVariable{clusterConfigVariable(g, &variables.ClusterConfigSpec{
					GenericClusterConfigSpec: carenv1.GenericClusterConfigSpec{
						ImageRegistries: []carenv1.ImageRegistry{{
							URL: nutanixRegistryURL,
							Credentials: &carenv1.RegistryCredentials{
								SecretRef: &carenv1.LocalObjectReference{
									Name: nutanixRegistrySecretName("workload-update-inherit"),
								},
							},
						}},
					},
				})},
			},
		},
	}

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, mgmtSecret).
		Build()
	defaulter := &nutanixRegistryDefaulter{client: fakeClient}

	g.Expect(defaulter.mutateCluster(ctx, req(admissionv1.Update), workload)).To(Succeed())

	created := &corev1.Secret{}
	g.Expect(fakeClient.Get(
		ctx,
		types.NamespacedName{
			Namespace: workload.Namespace,
			Name:      nutanixRegistrySecretName(workload.Name),
		},
		created,
	)).To(Succeed())
	g.Expect(created.StringData).To(HaveKeyWithValue(nutanixRegistryUsernameKey, "mgmt-user"))
	g.Expect(created.StringData).To(HaveKeyWithValue(nutanixRegistryPasswordKey, "mgmt-pass"))
}
