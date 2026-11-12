//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

const (
	// nutanixProjectNameVar names the Prism Central project the project e2e scenarios deploy into,
	// and that the project-scoped user is granted on.
	nutanixProjectNameVar = "NUTANIX_PROJECT_NAME"
	// nutanixInaccessibleProjectNameVar names a Prism Central project the project-scoped user is
	// deliberately *not* granted on.
	nutanixInaccessibleProjectNameVar = "NUTANIX_INACCESSIBLE_PROJECT_NAME"
	// nutanixProjectStorageContainerNameVar names the storage container CSI provisions from inside
	// the project, which is not the broad-scope NUTANIX_STORAGE_CONTAINER_NAME.
	nutanixProjectStorageContainerNameVar = "NUTANIX_PROJECT_STORAGE_CONTAINER_NAME"
	// nutanixProjectScopeUserVar and nutanixProjectScopePasswordVar hold the project-scoped Prism
	// Central credentials. They are consumed by the cluster template, not by the test process.
	nutanixProjectScopeUserVar = "NUTANIX_PROJECT_SCOPE_USER"
	//nolint:gosec // This is the name of an environment variable, not a credential.
	nutanixProjectScopePasswordVar = "NUTANIX_PROJECT_SCOPE_PASSWORD"

	// cloudProviderUninitializedTaint is set by the kubelet when it runs with an external cloud
	// provider, and is removed by the CCM once it has initialized the node.
	cloudProviderUninitializedTaint = "node.cloudprovider.kubernetes.io/uninitialized"
	// nutanixProviderIDPrefix prefixes the provider ID the Nutanix CCM assigns to a node.
	nutanixProviderIDPrefix = "nutanix://"
)

// skipUnlessNutanixProjectsConfigured skips the calling spec unless the Nutanix infrastructure
// provider and a Prism Central project are configured.
func skipUnlessNutanixProjectsConfigured() {
	if !slices.Contains(e2eConfig.InfrastructureProviders(), nutanixInfraProviderName) {
		Skip("Nutanix infrastructure provider is not enabled")
	}
	for _, required := range []string{nutanixProjectNameVar, nutanixProjectStorageContainerNameVar} {
		if strings.TrimSpace(e2eConfig.GetVariableOrEmpty(required)) == "" {
			Skip(required + " is required for the Nutanix projects scenarios")
		}
	}
}

// skipUnlessProjectScopedUserConfigured skips the calling spec unless project-scoped Prism Central
// credentials are configured.
func skipUnlessProjectScopedUserConfigured() {
	if strings.TrimSpace(e2eConfig.GetVariableOrEmpty(nutanixProjectScopeUserVar)) == "" ||
		strings.TrimSpace(e2eConfig.GetVariableOrEmpty(nutanixProjectScopePasswordVar)) == "" {
		Skip(fmt.Sprintf(
			"both %s and %s must be set for the project-scoped-user scenarios",
			nutanixProjectScopeUserVar, nutanixProjectScopePasswordVar,
		))
	}
}

// AssertClusterResolvedNutanixProject checks that CAREN resolved the cluster's Nutanix project
// against Prism Central at admission time and recorded the resulting policy on the Cluster. Every
// Nutanix cluster carries a policy: a projectless one resolves against the Prism Central default
// project, and one in a non-default project must resolve to single-project with a concrete UUID.
func AssertClusterResolvedNutanixProject(cluster *clusterv1.Cluster) {
	By("Verifying CAREN recorded the resolved Nutanix project policy on the Cluster")
	policy := cluster.Annotations[nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey]
	Expect(policy).ToNot(BeEmpty(),
		"CAREN should have defaulted the %s annotation",
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey,
	)
	if policy == nutanixprojectpolicy.PolicySingleProject {
		Expect(cluster.Annotations).To(HaveKey(nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey))
		Expect(cluster.Annotations[nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey]).ToNot(BeEmpty(),
			"a single-project policy must carry the resolved project UUID",
		)
	}
}

// AssertClusterPinnedToSingleNutanixProject checks that a cluster which was requested into a
// non-default Prism Central project was recorded as single-project with a concrete UUID. Unlike
// AssertClusterResolvedNutanixProject, a default-only or unrestricted policy fails: that would
// mean defaulting ignored the project references.
func AssertClusterPinnedToSingleNutanixProject(cluster *clusterv1.Cluster) {
	By("Verifying CAREN pinned the Cluster to a single non-default Nutanix project")
	Expect(cluster.Annotations).To(HaveKeyWithValue(
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey,
		nutanixprojectpolicy.PolicySingleProject,
	), "a cluster deployed into NUTANIX_PROJECT_NAME must resolve to single-project")
	Expect(cluster.Annotations[nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey]).ToNot(BeEmpty(),
		"a single-project policy must carry the resolved project UUID",
	)
}

// AssertNodesInitializedByNutanixCCM checks that the Nutanix CCM initialized every node in the
// workload cluster: the kubelet's cloud-provider taint is gone and the CCM has assigned a Nutanix
// provider ID. Both only happen after the CCM successfully talks to Prism Central with the
// cluster's credentials, so this is the functional signal that CCM works for these credentials.
func AssertNodesInitializedByNutanixCCM(
	ctx context.Context,
	clusterProxy capiframework.ClusterProxy,
	cluster *clusterv1.Cluster,
	intervals []interface{},
) {
	By("Verifying the Nutanix CCM initialized every node")
	workloadClient := clusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name).GetClient()

	Eventually(func(g Gomega) {
		nodes := &corev1.NodeList{}
		g.Expect(workloadClient.List(ctx, nodes)).To(Succeed())
		g.Expect(nodes.Items).ToNot(BeEmpty(), "expected at least one node")

		for i := range nodes.Items {
			node := &nodes.Items[i]
			for _, taint := range node.Spec.Taints {
				g.Expect(taint.Key).ToNot(
					Equal(cloudProviderUninitializedTaint),
					"node %s still carries the cloud-provider uninitialized taint", node.Name,
				)
			}
			g.Expect(node.Spec.ProviderID).To(
				HavePrefix(nutanixProviderIDPrefix),
				"node %s should have a Nutanix provider ID assigned by the CCM", node.Name,
			)
		}
	}, intervals...).Should(Succeed())
}

// AssertNutanixCSIDynamicProvisioning provisions a PersistentVolumeClaim from the cluster's default
// StorageClass, writes a token to it from one Pod, then reads that token back from a second Pod
// scheduled after the first is gone. Binding the claim makes the CSI driver create a volume group in
// Prism Central with the cluster's CSI credentials, and reading the token back across two Pods
// proves the data survived a detach/reattach rather than being served from one Pod's page cache -
// which is where volume group bugs actually surface. A project-scoped user can only do any of this
// inside its own project, so a successful round trip is itself the proof that the volume landed in
// the right project. Pods and the claim are deleted in a Go defer so a failed step still
// releases the volume group before QuickStartSpec tears the workload cluster down.
func AssertNutanixCSIDynamicProvisioning(
	ctx context.Context,
	clusterProxy capiframework.ClusterProxy,
	cluster *clusterv1.Cluster,
	intervals []interface{},
) {
	const (
		pvcName    = "caren-e2e-csi"
		writerName = "caren-e2e-csi-writer"
		readerName = "caren-e2e-csi-reader"
		namespace  = corev1.NamespaceDefault
		image      = "registry.k8s.io/e2e-test-images/agnhost:2.57"
		mountPath  = "/data"
		filePath   = mountPath + "/token"
	)

	workloadClient := clusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name).GetClient()
	token := util.RandomString(16)

	By("Creating a PersistentVolumeClaim from the default StorageClass")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	Expect(workloadClient.Create(ctx, pvc)).To(Succeed())

	var (
		writer     *corev1.Pod
		reader     *corev1.Pod
		volumeName string
	)
	// Go defer, not Ginkgo DeferCleanup: QuickStartSpec deletes the workload cluster at the end
	// of the spec body, so a DeferCleanup registered here runs too late and talks to a dead API.
	defer func() {
		By("Deleting CSI test Pods and PersistentVolumeClaim so the Nutanix volume group is released")
		if writer != nil {
			Expect(client.IgnoreNotFound(workloadClient.Delete(ctx, writer))).To(Succeed())
		}
		if reader != nil {
			Expect(client.IgnoreNotFound(workloadClient.Delete(ctx, reader))).To(Succeed())
		}
		Expect(client.IgnoreNotFound(workloadClient.Delete(ctx, pvc))).To(Succeed())
		if volumeName == "" {
			return
		}
		Eventually(func(g Gomega) {
			pv := &corev1.PersistentVolume{}
			err := workloadClient.Get(ctx, client.ObjectKey{Name: volumeName}, pv)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "PersistentVolume %s still exists", volumeName)
		}, intervals...).Should(Succeed())
	}()

	By("Writing a token to the volume")
	writer = csiTestPod(writerName, namespace, image, pvcName, mountPath,
		fmt.Sprintf("echo -n %q > %s && sync", token, filePath),
	)
	Expect(workloadClient.Create(ctx, writer)).To(Succeed())
	waitForPodToSucceed(ctx, workloadClient, writer, intervals)

	Expect(workloadClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
	Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound), "PersistentVolumeClaim is not bound")
	volumeName = pvc.Spec.VolumeName
	Expect(volumeName).ToNot(BeEmpty())

	By("Detaching the volume by removing the writer Pod")
	Expect(client.IgnoreNotFound(workloadClient.Delete(ctx, writer))).To(Succeed())
	Eventually(func(g Gomega) {
		err := workloadClient.Get(ctx, client.ObjectKeyFromObject(writer), &corev1.Pod{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "writer Pod still exists")
	}, intervals...).Should(Succeed())

	By("Reading the token back from a second Pod")
	reader = csiTestPod(readerName, namespace, image, pvcName, mountPath,
		// Exits non-zero, failing the Pod, if the volume came back empty or altered.
		fmt.Sprintf("test \"$(cat %s)\" = %q", filePath, token),
	)
	Expect(workloadClient.Create(ctx, reader)).To(Succeed())
	waitForPodToSucceed(ctx, workloadClient, reader, intervals)
}

// csiTestPod builds a run-once Pod that mounts claimName and runs script against it.
func csiTestPod(name, namespace, image, claimName, mountPath, script string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:         name,
				Image:        image,
				Command:      []string{"/bin/sh", "-c", script},
				VolumeMounts: []corev1.VolumeMount{{Name: "volume", MountPath: mountPath}},
			}},
			Volumes: []corev1.Volume{{
				Name: "volume",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: claimName,
					},
				},
			}},
		},
	}
}

// waitForPodToSucceed waits for a run-once Pod to complete successfully, giving up immediately if
// it fails rather than burning the whole timeout on a Pod that can no longer succeed.
func waitForPodToSucceed(
	ctx context.Context,
	workloadClient client.Client,
	pod *corev1.Pod,
	intervals []interface{},
) {
	key := client.ObjectKeyFromObject(pod)
	Eventually(func(g Gomega) {
		g.Expect(workloadClient.Get(ctx, key, pod)).To(Succeed())
		if pod.Status.Phase == corev1.PodFailed {
			StopTrying(fmt.Sprintf(
				"Pod %s failed: %s", key, podTerminationDetail(pod),
			)).Now()
		}
		g.Expect(pod.Status.Phase).To(
			Equal(corev1.PodSucceeded), "Pod %s has not completed", key,
		)
	}, intervals...).Should(Succeed())
}

// podTerminationDetail summarises why a Pod's containers stopped, for failure messages.
func podTerminationDetail(pod *corev1.Pod) string {
	details := make([]string, 0, len(pod.Status.ContainerStatuses))
	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]
		if terminated := cs.State.Terminated; terminated != nil {
			details = append(details, fmt.Sprintf(
				"%s exited %d (%s)", cs.Name, terminated.ExitCode, terminated.Reason,
			))
			continue
		}
		details = append(details, fmt.Sprintf("%s not terminated", cs.Name))
	}
	if len(details) == 0 {
		return "no container statuses reported"
	}
	return strings.Join(details, "; ")
}

// applyClusterFromFlavorInput is the input for applyClusterFromFlavor.
type applyClusterFromFlavorInput struct {
	testE2EConfig *clusterctl.E2EConfig
	// clusterctlConfigPath must point at a repository built from testE2EConfig, so that the
	// per-spec variables it carries - notably the reserved endpoint IPs - are available to the
	// template.
	clusterctlConfigPath string
	flavor               string
	clusterName          string
	// projectNameOverride, when non-empty, rewrites every Nutanix machineDetails project
	// reference in the rendered Cluster before it is applied.
	projectNameOverride string
}

// applyClusterFromFlavor renders a cluster template and applies it to the management cluster one
// object at a time, returning the error from the first object the API server rejects. It is for
// specs that assert on admission rather than on a provisioned cluster; use QuickStartSpec when the
// cluster is meant to come up. Anything it created, and the namespace holding it, is cleaned up
// when the spec finishes.
func applyClusterFromFlavor(input applyClusterFromFlavorInput) error {
	testE2EConfig := input.testE2EConfig
	clusterName := input.clusterName
	namespace := clusterName

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	Expect(bootstrapClusterProxy.GetClient().Create(ctx, ns)).To(Succeed())

	createdCluster := false
	DeferCleanup(func() {
		if createdCluster && !skipCleanup {
			capiframework.DeleteAllClustersAndWait(
				ctx,
				capiframework.DeleteAllClustersAndWaitInput{
					ClusterProxy:         bootstrapClusterProxy,
					ClusterctlConfigPath: input.clusterctlConfigPath,
					Namespace:            namespace,
				},
				testE2EConfig.GetIntervals("default", "wait-delete-cluster")...,
			)
		}

		err := bootstrapClusterProxy.GetClient().Delete(
			ctx,
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
		)
		if err == nil || apierrors.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())
	})

	template := clusterctl.ConfigCluster(ctx, clusterctl.ConfigClusterInput{
		LogFolder:              artifactFolder,
		ClusterctlConfigPath:   input.clusterctlConfigPath,
		KubeconfigPath:         bootstrapClusterProxy.GetKubeconfigPath(),
		InfrastructureProvider: nutanixInfraProviderName,
		Flavor:                 input.flavor,
		Namespace:              namespace,
		ClusterName:            clusterName,
		KubernetesVersion:      testE2EConfig.MustGetVariable(capie2e.KubernetesVersion),
		// ConfigCluster dereferences both counts unconditionally, and the template needs them
		// substituted, so they are never safe to leave nil.
		ControlPlaneMachineCount: ptr.To[int64](1),
		WorkerMachineCount:       ptr.To[int64](1),
	})

	objects, err := decodeObjects(template)
	Expect(err).ToNot(HaveOccurred())

	for _, obj := range objects {
		if obj.GetKind() == clusterKind && input.projectNameOverride != "" {
			Expect(setNutanixMachineDetailsProject(obj, input.projectNameOverride)).To(Succeed())
		}
		if err := bootstrapClusterProxy.GetClient().Create(ctx, obj); err != nil {
			return err
		}
		if obj.GetKind() == clusterKind && obj.GetName() == clusterName {
			createdCluster = true
		}
	}

	return nil
}

// setNutanixMachineDetailsProject rewrites the project reference of the control plane and of every
// worker pool in a Cluster's topology variables to the named project. It fails if the cluster
// template does not already carry a project reference, so a template change cannot silently turn
// this into a no-op.
func setNutanixMachineDetailsProject(cluster *unstructured.Unstructured, projectName string) error {
	variables, found, err := unstructured.NestedSlice(cluster.Object, "spec", "topology", "variables")
	if err != nil {
		return fmt.Errorf("reading cluster spec.topology.variables: %w", err)
	}
	if !found {
		return fmt.Errorf("cluster has no spec.topology.variables")
	}

	rewritten := 0
	for _, raw := range variables {
		variable, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		// clusterConfig holds the control plane machine details, workerConfig the worker ones.
		var machineDetailsPath []string
		switch variable[yamlKeyName] {
		case "clusterConfig":
			machineDetailsPath = []string{yamlKeyValue, "controlPlane", nutanixInfraProviderName, "machineDetails"}
		case "workerConfig":
			machineDetailsPath = []string{yamlKeyValue, nutanixInfraProviderName, "machineDetails"}
		default:
			continue
		}

		machineDetails, found, err := unstructured.NestedMap(variable, machineDetailsPath...)
		if err != nil || !found {
			continue
		}
		if _, hasProject := machineDetails["project"]; !hasProject {
			continue
		}
		machineDetails["project"] = map[string]interface{}{
			"type":      yamlKeyName,
			yamlKeyName: projectName,
		}
		if err := unstructured.SetNestedMap(variable, machineDetails, machineDetailsPath...); err != nil {
			return err
		}
		rewritten++
	}

	if rewritten == 0 {
		return fmt.Errorf("cluster template carries no Nutanix machineDetails project reference to rewrite")
	}

	return unstructured.SetNestedSlice(cluster.Object, variables, "spec", "topology", "variables")
}

// decodeObjects splits a rendered multi-document cluster template into individual objects.
func decodeObjects(manifest []byte) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	objects := make([]*unstructured.Unstructured, 0)

	for {
		obj := map[string]interface{}{}
		if err := decoder.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to decode manifest: %w", err)
		}
		if len(obj) == 0 {
			continue
		}
		objects = append(objects, &unstructured.Unstructured{Object: obj})
	}

	return objects, nil
}
