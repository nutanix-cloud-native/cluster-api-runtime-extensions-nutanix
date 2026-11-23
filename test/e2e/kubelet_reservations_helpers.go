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
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/framework/nutanix"
)

const yamlStringTag = "!!str"

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

const (
	// nutanixIPReserveMaxAttempts bounds how many consecutive Nutanix IPAM reservations are tried
	// when reserved addresses already respond on the workload subnet (for example, an orphaned VM
	// from an earlier timed-out cluster teardown still advertising kube-vip or MetalLB).
	nutanixIPReserveMaxAttempts = 5

	// nutanixIPProbeTimeout allows ARP resolution and a TCP SYN retry on the local Nutanix VLAN
	// while keeping unused-IP checks fast during BeforeEach.
	nutanixIPProbeTimeout = 3 * time.Second

	// nutanixClusterCheckTimeout bounds the management-cluster Cluster lookup during DeferCleanup.
	nutanixClusterCheckTimeout = 15 * time.Second
)

// nutanixIPProbePorts are the ports every Nutanix e2e test reaches directly from the runner
// (kube-apiserver on 6443 and the LoadBalancer test Service on 80), ensuring no intermediate
// firewall rule can spoof a TCP RST for an unused address.
var nutanixIPProbePorts = []string{"6443", "80"}

type (
	tcpDialFunc   func(ctx context.Context, network, address string) (net.Conn, error)
	ipReserveFunc func(ctx context.Context) (string, func() error, error)
	ipProbeFunc   func(ctx context.Context, ip string) (bool, string)
)

// generateClusterName builds a random cluster name of maximum length so e2e exercises the 63-char
// DNS label limit, shortened on Docker where CAPI appends 23 characters of random suffixes across
// DockerCluster, MachineDeployment, and Machine names and KinD enforces HOST_NAME_MAX=64.
func generateClusterName(prefix, lowercaseProvider string) string {
	maxClusterNameLength := 63
	if lowercaseProvider == "docker" {
		maxClusterNameLength = 64 - 23
	}
	return prefix + util.RandomString(maxClusterNameLength-len(prefix))
}

// reserveNutanixIPsForCluster reserves unused control-plane endpoint and Kubernetes Service
// load balancer IPs for a Nutanix workload cluster, registers gated cleanup to unreserve them
// only once the workload Cluster is confirmed deleted, and writes the reserved addresses into
// the e2e config variables. It must be called from within a running spec node (e.g. BeforeEach)
// so DeferCleanup is honoured.
func reserveNutanixIPsForCluster(testE2EConfig *clusterctl.E2EConfig, clusterName string) {
	nutanixClient, err := nutanix.NewConvergedV4Client(
		nutanix.CredentialsFromCAPIE2EConfig(testE2EConfig),
	)
	Expect(err).ToNot(HaveOccurred())
	subnetName := testE2EConfig.MustGetVariable("NUTANIX_SUBNET_NAME")
	prismElementClusterName := testE2EConfig.MustGetVariable("NUTANIX_PRISM_ELEMENT_CLUSTER_NAME")

	reserveClusterVariableIP := func(variableName, description string) {
		By("Reserving an IP address for the workload cluster " + description)
		ip, unreserve, reserveErr := reserveUnusedIP(
			context.Background(),
			nutanixIPReserveMaxAttempts,
			func(reserveCtx context.Context) (string, func() error, error) {
				return nutanix.ReserveIP(
					reserveCtx,
					subnetName,
					prismElementClusterName,
					nutanixClient,
				)
			},
			isNutanixIPInUse,
			Logf,
		)
		Expect(reserveErr).ToNot(HaveOccurred())
		DeferCleanup(func(cleanupCtx SpecContext) error {
			var k8sReader client.Reader
			if !skipCleanup && bootstrapClusterProxy != nil {
				k8sReader = bootstrapClusterProxy.GetClient()
			}
			return unreserveIPAfterClusterDeleted(
				cleanupCtx,
				ip,
				unreserve,
				skipCleanup,
				clusterName,
				k8sReader,
				Logf,
			)
		})
		testE2EConfig.Variables[variableName] = ip
		Logf("Reserved %s IP: %s", description, ip)
	}

	reserveClusterVariableIP("CONTROL_PLANE_ENDPOINT_IP", "control plane endpoint")
	reserveClusterVariableIP(
		"KUBERNETES_SERVICE_LOAD_BALANCER_IP",
		"kubernetes Service load balancer",
	)
}

func isNutanixIPInUse(ctx context.Context, ip string) (inUse bool, reason string) {
	probeCtx, cancel := context.WithTimeout(ctx, nutanixIPProbeTimeout)
	defer cancel()
	var dialer net.Dialer
	return probeTCPHostInUse(probeCtx, dialer.DialContext, ip, nutanixIPProbePorts)
}

// probeTCPHostInUse dials ports concurrently and reports whether any port proves a live TCP stack
// owns ip. Both a completed handshake and ECONNREFUSED (TCP RST) confirm a host answered ARP for
// ip, whereas routing/ARP timeouts (DeadlineExceeded, EHOSTUNREACH, ENETUNREACH) mean nothing
// responded at that address.
func probeTCPHostInUse(
	ctx context.Context,
	dialContext tcpDialFunc,
	ip string,
	ports []string,
) (inUse bool, reason string) {
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type probeResult struct {
		inUse  bool
		reason string
	}

	results := make(chan probeResult, len(ports))
	for _, port := range ports {
		addr := net.JoinHostPort(ip, port)
		go func(address string) {
			conn, err := dialContext(probeCtx, "tcp", address)
			if err == nil {
				_ = conn.Close()
				results <- probeResult{
					inUse:  true,
					reason: fmt.Sprintf("tcp %s connected", address),
				}
				return
			}
			if errors.Is(err, syscall.ECONNREFUSED) {
				results <- probeResult{
					inUse:  true,
					reason: fmt.Sprintf("tcp %s connection refused", address),
				}
				return
			}
			results <- probeResult{}
		}(addr)
	}

	for range ports {
		res := <-results
		if res.inUse {
			cancel()
			return true, res.reason
		}
	}
	return false, ""
}

// reserveUnusedIP reserves an IP via reserve and verifies via probe that nothing on the network is
// already responding at that address. Any collided address is intentionally left reserved in IPAM
// (quarantined) so neither the next retry nor concurrent specs receive it while the orphaned VM is
// still running.
func reserveUnusedIP(
	ctx context.Context,
	maxAttempts int,
	reserve ipReserveFunc,
	probe ipProbeFunc,
	logf func(format string, args ...interface{}),
) (reservedIP string, unreserve func() error, err error) {
	if maxAttempts < 1 {
		return "", nil, fmt.Errorf("maxAttempts must be at least 1, got %d", maxAttempts)
	}

	var lastIP, lastReason string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ip, unreserveFn, reserveErr := reserve(ctx)
		if reserveErr != nil {
			return "", nil, reserveErr
		}

		inUse, reason := probe(ctx, ip)
		if !inUse {
			return ip, unreserveFn, nil
		}

		lastIP = ip
		lastReason = reason
		logf(
			"Reserved IP %s is already reachable on the network (%s); "+
				"keeping reservation quarantined and retrying (attempt %d/%d)",
			ip, reason, attempt, maxAttempts,
		)
	}

	return "", nil, fmt.Errorf(
		"failed to reserve an unused IP after %d attempts: last reserved IP %s is already in use (%s)",
		maxAttempts, lastIP, lastReason,
	)
}

// shouldUnreserveClusterIP reports whether it is safe to release a workload cluster's reserved IP
// back to Nutanix IPAM. If cleanup was skipped, the management cluster cannot be queried, or the
// Cluster object still exists (for example because AfterEach timed out waiting for
// BeforeClusterDelete hooks or VM deletion), workload VMs may still hold the VIP via kube-vip or
// MetalLB; keeping the IP reserved quarantines it until those VMs are cleaned up.
func shouldUnreserveClusterIP(
	ctx context.Context,
	skipCleanup bool,
	clusterName string,
	k8sReader client.Reader,
) (canUnreserve bool, reason string) {
	if skipCleanup {
		return false, "skipCleanup is enabled"
	}
	if clusterName == "" {
		return false, "cluster name is empty"
	}
	if k8sReader == nil {
		return false, "management cluster client is unavailable"
	}

	clusterList := &clusterv1.ClusterList{}
	if err := k8sReader.List(ctx, clusterList); err != nil {
		return false, fmt.Sprintf("failed to list Clusters on management cluster: %v", err)
	}

	for i := range clusterList.Items {
		c := &clusterList.Items[i]
		if c.Name == clusterName {
			return false, fmt.Sprintf("Cluster %s/%s still exists", c.Namespace, c.Name)
		}
	}

	return true, ""
}

func unreserveIPAfterClusterDeleted(
	ctx context.Context,
	ip string,
	unreserve func() error,
	skipCleanup bool,
	clusterName string,
	k8sReader client.Reader,
	logf func(format string, args ...interface{}),
) error {
	checkCtx, cancel := context.WithTimeout(ctx, nutanixClusterCheckTimeout)
	defer cancel()

	ok, reason := shouldUnreserveClusterIP(checkCtx, skipCleanup, clusterName, k8sReader)
	if !ok {
		logf(
			"Skipping unreserving Nutanix IP %s (%s); "+
				"keeping IPAM reservation to prevent VIP collision",
			ip, reason,
		)
		return nil
	}

	return unreserve()
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
				Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: kubeletInUserNamespaceFeatureGate,
			})
			return
		}
		if strings.Contains(value.Value, "KubeletInUserNamespace") {
			return
		}
		value.Value += "," + kubeletInUserNamespaceFeatureGate
		value.Tag = yamlStringTag
		value.Style = 0
		return
	}
	seq.Content = append(seq.Content, &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "name"},
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "feature-gates"},
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "value"},
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: kubeletInUserNamespaceFeatureGate},
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
				&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "value"},
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
		{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "automaticReservations"},
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "profile"},
			{
				Kind:  yaml.ScalarNode,
				Tag:   yamlStringTag,
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
		&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: key},
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
