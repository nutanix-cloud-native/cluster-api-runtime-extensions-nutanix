//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	helmaddonsv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

const (
	carenSystemNamespace  = "caren-system"
	carenManagerContainer = "manager"
	carenLogTailLines     = int64(400)
)

var workloadDiagnosticNamespaces = []string{
	"kube-system",
	"flow-cni-system",
	"flow-cns-system",
	"ovn-kubernetes",
	"ntnx-system",
}

// dumpAddonDiagnostics writes HelmChartProxy/HelmReleaseProxy status, CAREN controller
// logs, and workload-cluster CNI-related objects to the Ginkgo output. It is intended
// to run from AfterEach on failure, before CAPI deletes the workload cluster.
func dumpAddonDiagnostics(ctx context.Context, proxy framework.ClusterProxy) {
	defer func() {
		if r := recover(); r != nil {
			Logf("Failed to dump addon diagnostics: %v", r)
		}
	}()

	if proxy == nil {
		Log("Skipping addon diagnostics: bootstrap cluster proxy is nil")
		return
	}

	capie2e.Byf("Dumping addon diagnostics for the failed spec")
	cl := proxy.GetClient()

	dumpHelmChartProxies(ctx, cl)
	dumpHelmReleaseProxies(ctx, cl)
	dumpCARENLogs(ctx, proxy)
	dumpWorkloadClusterCNIState(ctx, proxy)
}

func dumpHelmChartProxies(ctx context.Context, cl ctrlclient.Client) {
	list := &helmaddonsv1.HelmChartProxyList{}
	if err := cl.List(ctx, list); err != nil {
		Logf("Failed to list HelmChartProxies: %v", err)
		return
	}

	Logf("Found %d HelmChartProxy object(s)", len(list.Items))
	for i := range list.Items {
		hcp := &list.Items[i]
		Logf(
			"HelmChartProxy %s/%s chart=%s version=%s repo=%s release=%s/%s generation=%d observedGeneration=%d conditions=%s",
			hcp.Namespace,
			hcp.Name,
			hcp.Spec.ChartName,
			hcp.Spec.Version,
			hcp.Spec.RepoURL,
			hcp.Spec.ReleaseNamespace,
			hcp.Spec.ReleaseName,
			hcp.Generation,
			hcp.Status.ObservedGeneration,
			formatMetaConditions(hcp.GetConditions()),
		)
	}
}

func dumpHelmReleaseProxies(ctx context.Context, cl ctrlclient.Client) {
	list := &helmaddonsv1.HelmReleaseProxyList{}
	if err := cl.List(ctx, list); err != nil {
		Logf("Failed to list HelmReleaseProxies: %v", err)
		return
	}

	Logf("Found %d HelmReleaseProxy object(s)", len(list.Items))
	for i := range list.Items {
		hrp := &list.Items[i]
		Logf(
			"HelmReleaseProxy %s/%s cluster=%s release=%s/%s status=%s "+
				"revision=%d generation=%d observedGeneration=%d conditions=%s",
			hrp.Namespace,
			hrp.Name,
			hrp.Spec.ClusterRef.Name,
			hrp.Spec.ReleaseNamespace,
			hrp.Spec.ReleaseName,
			hrp.Status.Status,
			hrp.Status.Revision,
			hrp.Generation,
			hrp.Status.ObservedGeneration,
			formatMetaConditions(hrp.GetConditions()),
		)
	}
}

func dumpCARENLogs(ctx context.Context, proxy framework.ClusterProxy) {
	clientset := proxy.GetClientSet()
	if clientset == nil {
		Log("Skipping CAREN logs: clientset is nil")
		return
	}

	pods, err := clientset.CoreV1().Pods(carenSystemNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		Logf("Failed to list CAREN pods in %s: %v", carenSystemNamespace, err)
		return
	}
	if len(pods.Items) == 0 {
		Logf("No CAREN pods found in namespace %s", carenSystemNamespace)
		return
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		Logf(
			"CAREN pod %s/%s phase=%s ready=%s",
			pod.Namespace,
			pod.Name,
			pod.Status.Phase,
			podReadyReason(pod),
		)
		logs, logErr := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: carenManagerContainer,
			TailLines: ptr.To(carenLogTailLines),
		}).Do(ctx).Raw()
		if logErr != nil {
			Logf("Failed to get logs for CAREN pod %s/%s: %v", pod.Namespace, pod.Name, logErr)
			continue
		}
		Logf("----- begin CAREN logs %s/%s -----", pod.Namespace, pod.Name)
		Log(strings.TrimRight(string(logs), "\n"))
		Logf("----- end CAREN logs %s/%s -----", pod.Namespace, pod.Name)
	}
}

func dumpWorkloadClusterCNIState(ctx context.Context, proxy framework.ClusterProxy) {
	clusterList := &clusterv1.ClusterList{}
	if err := proxy.GetClient().List(ctx, clusterList); err != nil {
		Logf("Failed to list Clusters: %v", err)
		return
	}

	Logf("Found %d Cluster object(s)", len(clusterList.Items))
	for i := range clusterList.Items {
		cluster := &clusterList.Items[i]
		Logf(
			"Cluster %s/%s phase=%s conditions=%s",
			cluster.Namespace,
			cluster.Name,
			cluster.Status.GetTypedPhase(),
			formatMetaConditions(cluster.GetConditions()),
		)

		workloadProxy, err := workloadClusterProxy(ctx, proxy, cluster)
		if err != nil {
			Logf(
				"Failed to get workload cluster client for %s/%s: %v",
				cluster.Namespace,
				cluster.Name,
				err,
			)
			continue
		}
		dumpWorkloadNodesAndPods(ctx, workloadProxy, cluster)
	}
}

func workloadClusterProxy(
	ctx context.Context,
	proxy framework.ClusterProxy,
	cluster *clusterv1.Cluster,
) (workload framework.ClusterProxy, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic getting workload cluster proxy: %v", r)
		}
	}()
	return proxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name), nil
}

func dumpWorkloadNodesAndPods(
	ctx context.Context,
	workload framework.ClusterProxy,
	cluster *clusterv1.Cluster,
) {
	cl := workload.GetClient()

	nodes := &corev1.NodeList{}
	if err := cl.List(ctx, nodes); err != nil {
		Logf("Failed to list Nodes on workload cluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
	} else {
		Logf("Workload cluster %s/%s has %d Node(s)", cluster.Namespace, cluster.Name, len(nodes.Items))
		for i := range nodes.Items {
			node := &nodes.Items[i]
			Logf("Node %s conditions=%s", node.Name, formatNodeConditions(node.Status.Conditions))
		}
	}

	for _, ns := range workloadDiagnosticNamespaces {
		dumpNamespacePodsAndSecrets(ctx, cl, cluster, ns)
		dumpNamespaceWarningEvents(ctx, workload, cluster, ns)
	}
}

func dumpNamespacePodsAndSecrets(
	ctx context.Context,
	cl ctrlclient.Client,
	cluster *clusterv1.Cluster,
	namespace string,
) {
	pods := &corev1.PodList{}
	if err := cl.List(ctx, pods, ctrlclient.InNamespace(namespace)); err != nil {
		Logf(
			"Failed to list Pods in %s on workload cluster %s/%s: %v",
			namespace,
			cluster.Namespace,
			cluster.Name,
			err,
		)
	} else {
		Logf(
			"Workload cluster %s/%s namespace %s has %d Pod(s)",
			cluster.Namespace,
			cluster.Name,
			namespace,
			len(pods.Items),
		)
		for i := range pods.Items {
			Logf("Pod %s", formatPod(&pods.Items[i]))
		}
	}

	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets, ctrlclient.InNamespace(namespace)); err != nil {
		Logf(
			"Failed to list Secrets in %s on workload cluster %s/%s: %v",
			namespace,
			cluster.Namespace,
			cluster.Name,
			err,
		)
		return
	}
	names := make([]string, 0, len(secrets.Items))
	for i := range secrets.Items {
		names = append(names, secrets.Items[i].Name)
	}
	Logf(
		"Workload cluster %s/%s namespace %s secrets=%v",
		cluster.Namespace,
		cluster.Name,
		namespace,
		names,
	)
}

func dumpNamespaceWarningEvents(
	ctx context.Context,
	workload framework.ClusterProxy,
	cluster *clusterv1.Cluster,
	namespace string,
) {
	clientset := workload.GetClientSet()
	if clientset == nil {
		return
	}
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		Logf(
			"Failed to list warning Events in %s on workload cluster %s/%s: %v",
			namespace,
			cluster.Namespace,
			cluster.Name,
			err,
		)
		return
	}
	if len(events.Items) == 0 {
		return
	}
	Logf(
		"Workload cluster %s/%s namespace %s has %d warning Event(s)",
		cluster.Namespace,
		cluster.Name,
		namespace,
		len(events.Items),
	)
	limit := min(len(events.Items), 30)
	for i := range events.Items[:limit] {
		ev := &events.Items[i]
		Logf(
			"Event %s/%s count=%d reason=%s object=%s/%s message=%s",
			ev.Namespace,
			ev.Name,
			ev.Count,
			ev.Reason,
			ev.InvolvedObject.Kind,
			ev.InvolvedObject.Name,
			ev.Message,
		)
	}
}

func formatPod(pod *corev1.Pod) string {
	parts := []string{
		fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
		fmt.Sprintf("phase=%s", pod.Status.Phase),
		fmt.Sprintf("ready=%s", podReadyReason(pod)),
	}
	if pod.Status.Reason != "" {
		parts = append(parts, "reason="+pod.Status.Reason)
	}
	if pod.Status.Message != "" {
		parts = append(parts, "message="+pod.Status.Message)
	}
	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]
		switch {
		case cs.State.Waiting != nil:
			parts = append(parts, fmt.Sprintf(
				"container=%s waiting reason=%s message=%q",
				cs.Name,
				cs.State.Waiting.Reason,
				cs.State.Waiting.Message,
			))
		case cs.State.Terminated != nil:
			parts = append(parts, fmt.Sprintf(
				"container=%s terminated reason=%s message=%q exit=%d",
				cs.Name,
				cs.State.Terminated.Reason,
				cs.State.Terminated.Message,
				cs.State.Terminated.ExitCode,
			))
		}
	}
	return strings.Join(parts, " ")
}

func podReadyReason(pod *corev1.Pod) string {
	for i := range pod.Status.Conditions {
		c := pod.Status.Conditions[i]
		if c.Type == corev1.PodReady {
			return fmt.Sprintf("%s reason=%s msg=%q", c.Status, c.Reason, c.Message)
		}
	}
	return "unknown"
}

func formatMetaConditions(conditions []metav1.Condition) string {
	if len(conditions) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(conditions))
	for _, c := range conditions {
		parts = append(parts, fmt.Sprintf(
			"%s=%s reason=%s msg=%q",
			c.Type,
			c.Status,
			c.Reason,
			c.Message,
		))
	}
	return strings.Join(parts, "; ")
}

func formatNodeConditions(conditions []corev1.NodeCondition) string {
	if len(conditions) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(conditions))
	for _, c := range conditions {
		parts = append(parts, fmt.Sprintf(
			"%s=%s reason=%s msg=%q",
			c.Type,
			c.Status,
			c.Reason,
			c.Message,
		))
	}
	return strings.Join(parts, "; ")
}
