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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/framework/nutanix"
)

const automaticReservationsFlavorSuffix = "-kubelet-reservations"

// kubeletInUserNamespaceEnvVar, when set to a truthy value, makes the e2e ClusterClass enable
// the KubeletInUserNamespace kubelet feature gate on workload nodes. This is required to run
// CAPD workload clusters under rootless Docker, where the kubelet cannot open /dev/kmsg and
// crash-loops with "failed to create kubelet: open /dev/kmsg: operation not permitted". It
// defaults to off so the standard rootful CI path is unaffected.
const kubeletInUserNamespaceEnvVar = "CAREN_E2E_KUBELET_IN_USERNS"

// kubeletInUserNamespaceFeatureGate is the kubelet flag value injected into kubeletExtraArgs.
const kubeletInUserNamespaceFeatureGate = "KubeletInUserNamespace=true"

// applyProviderKubernetesVersionOverride switches the e2e config to a provider-specific
// Kubernetes version when one is configured (e.g. KUBERNETES_VERSION_DOCKER), so that
// providers with differing machine-image availability can be tested independently.
func applyProviderKubernetesVersionOverride(
	testE2EConfig *clusterctl.E2EConfig,
	lowercaseProvider string,
) {
	varName := capie2e.KubernetesVersion + "_" + strings.ToUpper(lowercaseProvider)
	if testE2EConfig.HasVariable(varName) {
		testE2EConfig.Variables[capie2e.KubernetesVersion] = testE2EConfig.MustGetVariable(varName)
	}
}

// reserveNutanixIPsForCluster reserves the control-plane endpoint and Kubernetes Service
// load balancer IPs for a Nutanix workload cluster, registers cleanup to unreserve them,
// and writes the reserved addresses into the e2e config variables. It must be called from
// within a running spec node (e.g. BeforeEach) so DeferCleanup is honoured.
func reserveNutanixIPsForCluster(testE2EConfig *clusterctl.E2EConfig) {
	nutanixClient, err := nutanix.NewConvergedV4Client(
		nutanix.CredentialsFromCAPIE2EConfig(testE2EConfig),
	)
	Expect(err).ToNot(HaveOccurred())
	subnetName := testE2EConfig.MustGetVariable("NUTANIX_SUBNET_NAME")
	prismElementClusterName := testE2EConfig.MustGetVariable("NUTANIX_PRISM_ELEMENT_CLUSTER_NAME")

	By("Reserving an IP address for the workload cluster control plane endpoint")
	controlPlaneEndpointIP, unreserveControlPlaneEndpointIP, err := nutanix.ReserveIP(
		context.Background(),
		subnetName,
		prismElementClusterName,
		nutanixClient,
	)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(unreserveControlPlaneEndpointIP)
	testE2EConfig.Variables["CONTROL_PLANE_ENDPOINT_IP"] = controlPlaneEndpointIP
	Logf("Reserved control-plane endpoint IP: %s", controlPlaneEndpointIP)

	By("Reserving an IP address for the workload cluster kubernetes Service load balancer")
	kubernetesServiceLoadBalancerIP, unreservekubernetesServiceLoadBalancerIP, err := nutanix.ReserveIP(
		context.Background(),
		subnetName,
		prismElementClusterName,
		nutanixClient,
	)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(unreservekubernetesServiceLoadBalancerIP)
	testE2EConfig.Variables["KUBERNETES_SERVICE_LOAD_BALANCER_IP"] = kubernetesServiceLoadBalancerIP
	Logf("Reserved service load balancer IP: %s", kubernetesServiceLoadBalancerIP)
}

// isolateProviderVersions deep-copies the named provider's Versions and their Files slices on a
// testE2EConfig obtained from E2EConfig.DeepCopy. clusterctl's DeepCopy only copies the top-level
// Providers slice; the nested Versions and Files backing arrays remain shared with the original
// config. Without this isolation, appending a flavor (registerAutomaticReservationsFlavor) or
// repointing a file SourcePath (maybeEnableKubeletInUserNamespace) mutates the shared e2eConfig
// and leaks into other specs - e.g. the quick-start spec then tries to read the (now cleaned-up)
// reservations temp template and fails. Call this once after DeepCopy, before mutating Files.
func isolateProviderVersions(testE2EConfig *clusterctl.E2EConfig, lowercaseProvider string) {
	for i := range testE2EConfig.Providers {
		p := &testE2EConfig.Providers[i]
		if p.Name != lowercaseProvider {
			continue
		}
		versions := make([]clusterctl.ProviderVersionSource, len(p.Versions))
		copy(versions, p.Versions)
		for j := range versions {
			files := make([]clusterctl.Files, len(versions[j].Files))
			copy(files, versions[j].Files)
			versions[j].Files = files
		}
		p.Versions = versions
		return
	}
}

// registerAutomaticReservationsFlavor reads the published quick-start template for baseFlavor,
// enables workerConfig.kubeletConfiguration.automaticReservations on it, writes the patched
// template to tmpDir, and registers it as a new flavor in testE2EConfig. It returns the new
// flavor name. The published examples are never modified.
func registerAutomaticReservationsFlavor(
	testE2EConfig *clusterctl.E2EConfig,
	lowercaseProvider, baseFlavor, tmpDir string,
) string {
	baseTarget := "cluster-template-" + baseFlavor + ".yaml"
	newFlavor := baseFlavor + automaticReservationsFlavorSuffix
	newTarget := "cluster-template-" + newFlavor + ".yaml"

	for i := range testE2EConfig.Providers {
		p := &testE2EConfig.Providers[i]
		if p.Name != lowercaseProvider {
			continue
		}
		for j := range p.Versions {
			v := &p.Versions[j]
			for _, f := range v.Files {
				if f.TargetName != baseTarget {
					continue
				}
				raw, err := os.ReadFile(f.SourcePath)
				Expect(err).ToNot(HaveOccurred(),
					"failed to read base template %q", f.SourcePath)
				patched, err := enableAutomaticReservationsInClusterTemplate(raw)
				Expect(err).ToNot(HaveOccurred())
				dst := filepath.Join(tmpDir, newTarget)
				Expect(os.WriteFile(dst, patched, 0o600)).To(Succeed())
				v.Files = append(v.Files, clusterctl.Files{
					SourcePath: dst,
					TargetName: newTarget,
				})
				return newFlavor
			}
		}
	}
	Fail(fmt.Sprintf(
		"base flavor %q not found for provider %q", baseFlavor, lowercaseProvider,
	))
	return ""
}

// kubeletInUserNamespaceEnabled reports whether the KubeletInUserNamespace feature gate should
// be injected, based on the kubeletInUserNamespaceEnvVar environment variable. Defaults to off.
func kubeletInUserNamespaceEnabled() bool {
	v, ok := os.LookupEnv(kubeletInUserNamespaceEnvVar)
	if !ok {
		return false
	}
	enabled, err := strconv.ParseBool(v)
	return err == nil && enabled
}

// maybeEnableKubeletInUserNamespace patches the provider's quick-start ClusterClass to enable
// the KubeletInUserNamespace kubelet feature gate when kubeletInUserNamespaceEnvVar is set,
// writing the patched ClusterClass to tmpDir and repointing the e2e config file entry at it.
// It is a no-op (preserving the rootful default) when the env var is unset or falsey. The
// published ClusterClass source is never modified.
func maybeEnableKubeletInUserNamespace(
	testE2EConfig *clusterctl.E2EConfig,
	lowercaseProvider, tmpDir string,
) {
	if !kubeletInUserNamespaceEnabled() {
		return
	}

	target := "clusterclass-" + lowercaseProvider + "-quick-start.yaml"
	for i := range testE2EConfig.Providers {
		p := &testE2EConfig.Providers[i]
		if p.Name != lowercaseProvider {
			continue
		}
		for j := range p.Versions {
			v := &p.Versions[j]
			for k := range v.Files {
				f := &v.Files[k]
				if f.TargetName != target {
					continue
				}
				raw, err := os.ReadFile(f.SourcePath)
				Expect(err).ToNot(HaveOccurred(),
					"failed to read ClusterClass %q", f.SourcePath)
				patched, err := enableKubeletInUserNamespaceInClusterClass(raw)
				Expect(err).ToNot(HaveOccurred())
				dst := filepath.Join(tmpDir, target)
				Expect(os.WriteFile(dst, patched, 0o600)).To(Succeed())
				f.SourcePath = dst
				Logf(
					"Enabled %s in ClusterClass %q for rootless Docker",
					kubeletInUserNamespaceFeatureGate, target,
				)
				return
			}
		}
	}
	Fail(fmt.Sprintf(
		"ClusterClass file %q not found for provider %q", target, lowercaseProvider,
	))
}

// enableKubeletInUserNamespaceInClusterClass adds the KubeletInUserNamespace kubelet feature
// gate to every kubeletExtraArgs list in the (multi-document) ClusterClass YAML, covering the
// control-plane init/join configurations and the worker bootstrap template. All other content
// and scalar styles are preserved by operating at the YAML node level.
func enableKubeletInUserNamespaceInClusterClass(raw []byte) ([]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unmarshalling ClusterClass document: %w", err)
		}
		docs = append(docs, &doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("ClusterClass contains no YAML documents")
	}

	patched := false
	for _, doc := range docs {
		if ensureKubeletInUserNamespaceGate(doc) {
			patched = true
		}
	}
	if !patched {
		return nil, fmt.Errorf("ClusterClass has no kubeletExtraArgs to patch")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return nil, fmt.Errorf("marshalling patched ClusterClass: %w", err)
		}
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// ensureKubeletInUserNamespaceGate walks a YAML node tree and adds the KubeletInUserNamespace
// feature gate to every kubeletExtraArgs sequence it finds. It returns true if at least one
// kubeletExtraArgs list was found.
func ensureKubeletInUserNamespaceGate(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	found := false
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if ensureKubeletInUserNamespaceGate(child) {
				found = true
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Value == "kubeletExtraArgs" && value.Kind == yaml.SequenceNode {
				upsertFeatureGate(value)
				found = true
			}
			if ensureKubeletInUserNamespaceGate(value) {
				found = true
			}
		}
	}
	return found
}

// upsertFeatureGate ensures the KubeletInUserNamespace gate is present in a kubeletExtraArgs
// sequence node, whose items are {name, value} mappings. An existing feature-gates entry is
// extended; otherwise a new entry is appended.
func upsertFeatureGate(seq *yaml.Node) {
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		name := mappingValue(item, "name")
		if name == nil || name.Value != "feature-gates" {
			continue
		}
		value := mappingValue(item, "value")
		if value == nil {
			setMappingValue(item, "value", &yaml.Node{
				Kind: yaml.ScalarNode, Tag: "!!str", Value: kubeletInUserNamespaceFeatureGate,
			})
			return
		}
		if strings.Contains(value.Value, "KubeletInUserNamespace") {
			return
		}
		value.Value += "," + kubeletInUserNamespaceFeatureGate
		value.Tag = "!!str"
		value.Style = 0
		return
	}
	seq.Content = append(seq.Content, &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "feature-gates"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "value"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: kubeletInUserNamespaceFeatureGate},
		},
	})
}

// enableAutomaticReservationsInClusterTemplate sets
// workerConfig.kubeletConfiguration.automaticReservations.profile=CapacityTiered on the
// cluster-wide workerConfig variable and on any per-MachineDeployment workerConfig override,
// preserving all other content (e.g. provider machine details). The template may contain
// multiple YAML documents (e.g. the Nutanix example prepends credential Secrets); only the
// Cluster document is modified and all other documents are passed through unchanged.
//
// The edit is performed at the YAML node level so that the scalar style of every untouched
// node is preserved. This is essential because the templates contain envsubst placeholders:
// e.g. quoted annotation values like "${WORKER_MACHINE_COUNT}" must stay quoted, otherwise
// after substitution they would parse as integers and break conversion of string-typed fields.
func enableAutomaticReservationsInClusterTemplate(raw []byte) ([]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unmarshalling cluster template: %w", err)
		}
		docs = append(docs, &doc)
	}

	patched := false
	for _, doc := range docs {
		if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			continue
		}
		if kind := mappingValue(root, "kind"); kind == nil || kind.Value != "Cluster" {
			continue
		}
		if err := enableReservationsInClusterDocument(root); err != nil {
			return nil, err
		}
		patched = true
	}
	if !patched {
		return nil, fmt.Errorf("cluster template contains no Cluster document")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return nil, fmt.Errorf("marshalling patched cluster template: %w", err)
		}
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// enableReservationsInClusterDocument enables automatic reservations on the cluster-wide
// workerConfig variable and on any per-MachineDeployment workerConfig override of a single
// Cluster document's mapping root node.
func enableReservationsInClusterDocument(root *yaml.Node) error {
	topology := mappingValue(mappingValue(root, "spec"), "topology")
	if topology == nil {
		return fmt.Errorf("cluster template has no spec.topology")
	}

	if vars := mappingValue(topology, "variables"); vars != nil && vars.Kind == yaml.SequenceNode {
		enableReservationsInWorkerConfigVariables(vars)
	}

	if workers := mappingValue(topology, "workers"); workers != nil {
		if mds := mappingValue(workers, "machineDeployments"); mds != nil &&
			mds.Kind == yaml.SequenceNode {
			for _, md := range mds.Content {
				mdVars := mappingValue(md, "variables")
				if mdVars == nil {
					continue
				}
				if overrides := mappingValue(mdVars, "overrides"); overrides != nil &&
					overrides.Kind == yaml.SequenceNode {
					enableReservationsInWorkerConfigVariables(overrides)
				}
			}
		}
	}
	return nil
}

// enableReservationsInWorkerConfigVariables mutates the workerConfig entry within a topology
// variable (or override) sequence node to enable automatic reservations, leaving other
// variables and existing workerConfig content untouched.
func enableReservationsInWorkerConfigVariables(seq *yaml.Node) {
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		name := mappingValue(item, "name")
		if name == nil || name.Value != v1alpha1.WorkerConfigVariableName {
			continue
		}

		value := mappingValue(item, "value")
		if value == nil {
			value = &yaml.Node{Kind: yaml.MappingNode}
			item.Content = append(
				item.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "value"},
				value,
			)
		}
		// The published examples use either an empty flow mapping ("{}") or a block
		// mapping; normalise to a block mapping so the added key renders cleanly.
		value.Kind = yaml.MappingNode
		value.Tag = "!!map"
		value.Value = ""
		value.Style = 0

		setMappingValue(value, "kubeletConfiguration", reservationsNode())
	}
}

// reservationsNode builds the kubeletConfiguration value enabling capacity-tiered reservations.
func reservationsNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "automaticReservations"},
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "profile"},
			{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: string(v1alpha1.ReservationProfileCapacityTiered),
			},
		}},
	}}
}

// mappingValue returns the value node for key in a YAML mapping node, or nil.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// setMappingValue sets (or replaces) key in a YAML mapping node.
func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(
		node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

// assertWorkerNodesHaveReservedResources verifies that every worker node reports allocatable
// CPU and memory strictly below capacity, which proves the boot-time reservation script ran.
// CPU is the decisive signal: a default kubeadm node reserves no CPU, so allocatable CPU below
// capacity can only result from the injected kubeReserved.
func assertWorkerNodesHaveReservedResources(
	ctx context.Context,
	workloadClient client.Client,
	intervals []interface{},
) {
	Eventually(func(g Gomega) {
		nodes := &corev1.NodeList{}
		g.Expect(workloadClient.List(ctx, nodes)).To(Succeed())

		workerCount := 0
		for i := range nodes.Items {
			node := &nodes.Items[i]
			if _, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]; isControlPlane {
				continue
			}
			workerCount++

			cpuCapacity := node.Status.Capacity.Cpu()
			cpuAllocatable := node.Status.Allocatable.Cpu()
			g.Expect(cpuAllocatable.Cmp(*cpuCapacity)).To(
				Equal(-1),
				"worker %s: allocatable CPU %s should be less than capacity %s",
				node.Name, cpuAllocatable, cpuCapacity,
			)

			memoryCapacity := node.Status.Capacity.Memory()
			memoryAllocatable := node.Status.Allocatable.Memory()
			g.Expect(memoryAllocatable.Cmp(*memoryCapacity)).To(
				Equal(-1),
				"worker %s: allocatable memory %s should be less than capacity %s",
				node.Name, memoryAllocatable, memoryCapacity,
			)
		}
		g.Expect(workerCount).To(
			BeNumerically(">", 0), "expected at least one worker node to assert on",
		)
	}, intervals...).Should(Succeed())
}
