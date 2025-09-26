//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/serviceloadbalancer/metallb"
)

type WaitForServiceLoadBalancerToBeReadyInWorkloadClusterInput struct {
	ServiceLoadBalancer  *v1alpha1.ServiceLoadBalancer
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	DeploymentIntervals  []interface{}
	DaemonSetIntervals   []interface{}
	HelmReleaseIntervals []interface{}
	ResourceIntervals    []interface{}
}

func WaitForServiceLoadBalancerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForServiceLoadBalancerToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.ServiceLoadBalancer == nil {
		return
	}

	switch providerName := input.ServiceLoadBalancer.Provider; providerName {
	case v1alpha1.ServiceLoadBalancerProviderMetalLB:
		waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadCluster(
			ctx,
			waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadClusterInput{
				workloadCluster:      input.WorkloadCluster,
				clusterProxy:         input.ClusterProxy,
				deploymentIntervals:  input.DeploymentIntervals,
				daemonSetIntervals:   input.DaemonSetIntervals,
				helmReleaseIntervals: input.HelmReleaseIntervals,
				resourceIntervals:    input.ResourceIntervals,
			},
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for ServiceLoadBalancer provider %s to be ready",
				providerName,
			),
		)
	}
}

type waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadClusterInput struct {
	workloadCluster      *clusterv1.Cluster
	clusterProxy         framework.ClusterProxy
	helmReleaseIntervals []interface{}
	deploymentIntervals  []interface{}
	daemonSetIntervals   []interface{}
	resourceIntervals    []interface{}
}

func waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadClusterInput, //nolint:gocritic // OK in tests.
) {
	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.clusterProxy.GetClient(),
			Cluster:         input.workloadCluster,
			HelmReleaseName: "metallb",
		},
		input.helmReleaseIntervals...,
	)

	workloadClusterClient := input.clusterProxy.GetWorkloadCluster(
		ctx, input.workloadCluster.Namespace, input.workloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "metallb-controller",
				Namespace: "metallb-system",
			},
		},
	}, input.deploymentIntervals...)

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "metallb-speaker",
				Namespace: "metallb-system",
			},
		},
	}, input.daemonSetIntervals...)

	// Generate the MetalLB configuration objects, so we can wait for them to be
	// created on the workload cluster.
	cos, err := metallb.ConfigurationObjects(&metallb.ConfigurationInput{
		Name:      "metallb",
		Namespace: "metallb-system",
		// We need to populate AddressRanges to generate the configuration,
		// but the values are not important, because this test does not compare
		// them against the actual values.
		AddressRanges: []v1alpha1.AddressRange{
			{
				Start: "1.2.3.4",
				End:   "1.2.3.5",
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	WaitForResources(ctx, WaitForResourcesInput{
		Getter:    workloadClusterClient,
		Resources: cos,
	}, input.resourceIntervals...)
}

type EnsureLoadBalancerServiceInput struct {
	WorkloadCluster  *clusterv1.Cluster
	ClusterProxy     framework.ClusterProxy
	ServiceIntervals []interface{}
}

// EnsureLoadBalancerService creates a test Service of type LoadBalancer and tests that the assigned IP responds.
func EnsureLoadBalancerService(
	ctx context.Context,
	input EnsureLoadBalancerServiceInput,
) {
	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	svc := createTestService(ctx, workloadClusterClient, input.ServiceIntervals)

	By("Testing the LoadBalancer Service responds")
	getClientIPURL := &url.URL{
		Scheme: "http",
		Host:   getLoadBalancerAddress(svc),
		Path:   "/clientip",
	}
	klog.Infof("Testing the LoadBalancer Service on: %q", getClientIPURL.String())
	output := testServiceLoadBalancer(ctx, getClientIPURL, input.ServiceIntervals)
	Expect(output).ToNot(BeEmpty())
	klog.Infof("Got output from Kubernetes LoadBalancer Service: %q", output)
}

func createTestService(
	ctx context.Context,
	workloadClusterClient client.Client,
	intervals []interface{},
) *corev1.Service {
	const (
		name      = "echo"
		namespace = corev1.NamespaceDefault
		appKey    = "app"
		replicas  = int32(1)
		image     = "registry.k8s.io/e2e-test-images/agnhost:2.57"
		port      = 8080
		portName  = "http"
	)

	By("Creating a test Deployment for LoadBalancer Service")
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{appKey: name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{appKey: name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  name,
						Image: image,
						Args:  []string{"netexec", fmt.Sprintf("--http-port=%d", port)},
						Ports: []corev1.ContainerPort{{
							Name:          portName,
							ContainerPort: int32(port),
						}},
					}},
				},
			},
		},
	}
	if err := workloadClusterClient.Create(ctx, deployment); err != nil {
		Expect(err).ToNot(HaveOccurred())
	}
	By("Waiting for Deployment to be ready")
	Eventually(func(g Gomega) {
		g.Expect(workloadClusterClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
		g.Expect(deployment.Status.ReadyReplicas).To(Equal(replicas))
	}, intervals...).Should(Succeed(), "timed out waiting for Deployment to be ready")

	By("Creating a test Service for LoadBalancer Service")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{appKey: name},
			Ports: []corev1.ServicePort{{
				Name:       portName,
				Port:       80,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(port),
			}},
		},
	}
	if err := workloadClusterClient.Create(ctx, service); err != nil {
		Expect(err).ToNot(HaveOccurred())
	}
	By("Waiting for LoadBalacer IP/Hostname to be assigned")
	Eventually(func(g Gomega) {
		g.Expect(workloadClusterClient.Get(ctx, client.ObjectKeyFromObject(service), service)).To(Succeed())

		ingress := service.Status.LoadBalancer.Ingress
		g.Expect(ingress).ToNot(BeEmpty(), "no LoadBalancer ingress yet")

		ip := ingress[0].IP
		hostname := ingress[0].Hostname
		g.Expect(ip == "" && hostname == "").To(BeFalse(), "ingress has neither IP nor Hostname yet")
	}, intervals...).Should(Succeed(), "timed out waiting for LoadBalancer IP/hostname")

	return service
}

func getLoadBalancerAddress(svc *corev1.Service) string {
	ings := svc.Status.LoadBalancer.Ingress
	if len(ings) == 0 {
		return ""
	}
	address := ings[0].IP
	if address == "" {
		address = ings[0].Hostname
	}
	return address
}

func testServiceLoadBalancer(
	ctx context.Context,
	requestURL *url.URL,
	intervals []interface{},
) string {
	hc := &http.Client{Timeout: 5 * time.Second}
	var output string
	Eventually(func(g Gomega) string {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), http.NoBody)
		resp, err := hc.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return ""
		}
		b, _ := io.ReadAll(resp.Body)
		output = strings.TrimSpace(string(b))
		return output
	}, intervals...).ShouldNot(BeEmpty(), "no response from service")
	return output
}
