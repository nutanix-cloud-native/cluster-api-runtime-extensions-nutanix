// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	storageOnly       = "storage-only"
	computeAndStorage = "compute-and-storage"
)

var _ = Describe("StorageNodePoolValidator", func() {
	var validator *storageNodePoolValidator

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(clusterv1beta2.AddToScheme(scheme)).To(Succeed())
		validator = NewStorageNodePoolValidator(
			fake.NewClientBuilder().WithScheme(scheme).Build(),
			admission.NewDecoder(scheme),
		)
	})

	for _, provider := range []string{"nutanix"} {
		Context("provider "+provider, func() {
			DescribeTable("on create",
				func(pools []nodePool, expectedDenied []deniedPool) {
					resp := validator.validate(
						context.Background(),
						storageAdmissionRequest(admissionv1.Create, newStorageTestCluster(provider, pools...), nil),
					)
					expectStorageResponse(resp, expectedDenied)
				},
				Entry("allows no node pools", nil, nil),
				Entry("allows 0 storage node pools",
					[]nodePool{{name: "md-0"}, {name: "md-1"}},
					nil,
				),
				Entry("allows 1 storage-only node pool",
					[]nodePool{{name: "md-0"}, {name: "storage", storage: storageOnly}},
					nil,
				),
				Entry("allows 1 compute-and-storage node pool",
					[]nodePool{{name: "md-0"}, {name: "storage", storage: computeAndStorage}},
					nil,
				),
				Entry("rejects 2 storage-only node pools",
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "md-0"},
						{name: "storage-b", storage: storageOnly},
					},
					[]deniedPool{oneOfMultiple(0, "storage-a", 2), oneOfMultiple(2, "storage-b", 2)},
				),
				Entry("rejects a storage-only and a compute-and-storage node pool",
					[]nodePool{
						{name: "md-0"},
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: computeAndStorage},
					},
					[]deniedPool{oneOfMultiple(1, "storage-a", 2), oneOfMultiple(2, "storage-b", 2)},
				),
			)

			DescribeTable("on update",
				func(oldPools, newPools []nodePool, expectedDenied []deniedPool) {
					resp := validator.validate(
						context.Background(),
						storageAdmissionRequest(
							admissionv1.Update,
							newStorageTestCluster(provider, newPools...),
							newStorageTestCluster(provider, oldPools...),
						),
					)
					expectStorageResponse(resp, expectedDenied)
				},
				Entry("rejects adding a second storage node pool",
					[]nodePool{{name: "storage-a", storage: storageOnly}},
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: computeAndStorage},
					},
					[]deniedPool{added(1, "storage-b", `storage node pool "storage-a"`)},
				),
				Entry("rejects adding the storage label to an existing node pool",
					[]nodePool{{name: "storage-a", storage: computeAndStorage}, {name: "md-0"}},
					[]nodePool{
						{name: "storage-a", storage: computeAndStorage},
						{name: "md-0", storage: storageOnly},
					},
					[]deniedPool{added(1, "md-0", `storage node pool "storage-a"`)},
				),
				Entry("flags the added pool by name when node pools are reordered",
					[]nodePool{{name: "md-0"}, {name: "storage-a", storage: storageOnly}},
					[]nodePool{
						{name: "storage-b", storage: computeAndStorage},
						{name: "storage-a", storage: storageOnly},
						{name: "md-0"},
					},
					[]deniedPool{added(0, "storage-b", `storage node pool "storage-a"`)},
				),
				Entry("rejects adding 2 storage node pools to a cluster that had none",
					[]nodePool{{name: "md-0"}},
					[]nodePool{
						{name: "md-0"},
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: computeAndStorage},
					},
					[]deniedPool{oneOfMultiple(1, "storage-a", 2), oneOfMultiple(2, "storage-b", 2)},
				),
				Entry("does not name a removed storage node pool as existing",
					[]nodePool{{name: "storage-a", storage: storageOnly}},
					[]nodePool{
						{name: "storage-b", storage: storageOnly},
						{name: "storage-c", storage: computeAndStorage},
					},
					[]deniedPool{oneOfMultiple(0, "storage-b", 2), oneOfMultiple(1, "storage-c", 2)},
				),
				Entry("names only the storage node pools that are kept",
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: storageOnly},
					},
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-c", storage: computeAndStorage},
						{name: "storage-d", storage: computeAndStorage},
					},
					[]deniedPool{
						added(1, "storage-c", `storage node pool "storage-a"`),
						added(2, "storage-d", `storage node pool "storage-a"`),
					},
				),
				Entry("does not name a storage node pool whose label was removed",
					[]nodePool{{name: "storage-a", storage: storageOnly}, {name: "md-0"}, {name: "md-1"}},
					[]nodePool{
						{name: "storage-a"},
						{name: "md-0", storage: storageOnly},
						{name: "md-1", storage: computeAndStorage},
					},
					[]deniedPool{oneOfMultiple(1, "md-0", 2), oneOfMultiple(2, "md-1", 2)},
				),
				Entry("allows a cluster that already has 2 storage node pools",
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: computeAndStorage},
					},
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: computeAndStorage},
						{name: "md-0"},
					},
					nil,
				),
				Entry("rejects growing from 2 to 3 storage node pools",
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: storageOnly},
					},
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: storageOnly},
						{name: "storage-c", storage: computeAndStorage},
					},
					[]deniedPool{added(2, "storage-c", `storage node pools "storage-a", "storage-b"`)},
				),
			)

			DescribeTable("with CSI providers on create",
				func(csiProviders []string, pools []nodePool, expectedDenied []deniedPool) {
					cluster := withCSIProviders(newStorageTestCluster(provider, pools...), csiProviders...)
					resp := validator.validate(
						context.Background(),
						storageAdmissionRequest(admissionv1.Create, cluster, nil),
					)
					expectStorageResponse(resp, expectedDenied)
				},
				Entry("allows Nutanix CSI without storage node pools",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "md-0"}},
					nil,
				),
				Entry("allows a storage node pool with only other CSI providers",
					[]string{v1alpha1.CSIProviderLocalPath},
					[]nodePool{{name: "md-0"}, {name: "storage", storage: storageOnly}},
					nil,
				),
				Entry("rejects Nutanix CSI with a storage-only node pool",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "md-0"}, {name: "storage", storage: storageOnly}},
					[]deniedPool{withNutanixCSI(1, "storage")},
				),
				Entry("rejects Nutanix CSI with a compute-and-storage node pool",
					[]string{v1alpha1.CSIProviderNutanix, v1alpha1.CSIProviderLocalPath},
					[]nodePool{{name: "storage", storage: computeAndStorage}, {name: "md-0"}},
					[]deniedPool{withNutanixCSI(0, "storage")},
				),
				Entry("reports both rules for Nutanix CSI with 2 storage node pools",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{
						{name: "storage-a", storage: storageOnly},
						{name: "storage-b", storage: computeAndStorage},
					},
					[]deniedPool{
						oneOfMultiple(0, "storage-a", 2),
						oneOfMultiple(1, "storage-b", 2),
						withNutanixCSI(0, "storage-a"),
						withNutanixCSI(1, "storage-b"),
					},
				),
			)

			DescribeTable("with CSI providers on update",
				func(oldCSI []string, oldPools []nodePool, newCSI []string, newPools []nodePool,
					expectedDenied []deniedPool,
				) {
					resp := validator.validate(
						context.Background(),
						storageAdmissionRequest(
							admissionv1.Update,
							withCSIProviders(newStorageTestCluster(provider, newPools...), newCSI...),
							withCSIProviders(newStorageTestCluster(provider, oldPools...), oldCSI...),
						),
					)
					expectStorageResponse(resp, expectedDenied)
				},
				Entry("rejects adding a storage node pool when Nutanix CSI is configured",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "md-0"}},
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "md-0"}, {name: "storage", storage: computeAndStorage}},
					[]deniedPool{withNutanixCSI(1, "storage")},
				),
				Entry("rejects adding the storage label to an existing node pool when Nutanix CSI is configured",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "md-0"}},
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "md-0", storage: storageOnly}},
					[]deniedPool{withNutanixCSI(0, "md-0")},
				),
				Entry("rejects moving the storage label to another node pool when Nutanix CSI is configured",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "storage-a", storage: storageOnly}, {name: "md-0"}},
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "storage-a"}, {name: "md-0", storage: storageOnly}},
					[]deniedPool{withNutanixCSI(1, "md-0")},
				),
				Entry("flags only the added pool when a cluster with both gains a storage node pool",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "storage-a", storage: storageOnly}},
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{
						{name: "storage-b", storage: computeAndStorage},
						{name: "storage-a", storage: storageOnly},
					},
					[]deniedPool{
						added(0, "storage-b", `storage node pool "storage-a"`),
						withNutanixCSI(0, "storage-b"),
					},
				),
				Entry("rejects adding Nutanix CSI to a cluster with a storage node pool",
					[]string{v1alpha1.CSIProviderLocalPath},
					[]nodePool{{name: "storage", storage: storageOnly}},
					[]string{v1alpha1.CSIProviderLocalPath, v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "storage", storage: storageOnly}},
					[]deniedPool{withNutanixCSI(0, "storage")},
				),
				Entry("allows unrelated updates to a cluster that already has both",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "storage", storage: storageOnly}},
					[]string{v1alpha1.CSIProviderNutanix, v1alpha1.CSIProviderLocalPath},
					[]nodePool{{name: "md-0"}, {name: "storage", storage: storageOnly}},
					nil,
				),
				Entry("allows removing Nutanix CSI from a cluster with a storage node pool",
					[]string{v1alpha1.CSIProviderNutanix},
					[]nodePool{{name: "storage", storage: storageOnly}},
					nil,
					[]nodePool{{name: "storage", storage: storageOnly}},
					nil,
				),
			)
		})
	}

	twoStoragePoolsWithNutanixCSI := func(provider string) *clusterv1beta2.Cluster {
		return withCSIProviders(
			newStorageTestCluster(provider,
				nodePool{name: "storage-a", storage: storageOnly},
				nodePool{name: "storage-b", storage: computeAndStorage},
			),
			v1alpha1.CSIProviderNutanix,
		)
	}

	It("skips other providers", func() {
		resp := validator.validate(
			context.Background(),
			storageAdmissionRequest(admissionv1.Create, twoStoragePoolsWithNutanixCSI("aws"), nil),
		)
		expectStorageResponse(resp, nil)
	})

	It("skips non-topology clusters", func() {
		cluster := twoStoragePoolsWithNutanixCSI("nutanix")
		cluster.Spec.Topology = clusterv1beta2.Topology{}
		resp := validator.validate(context.Background(), storageAdmissionRequest(admissionv1.Create, cluster, nil))
		expectStorageResponse(resp, nil)
	})

	It("skips delete operations", func() {
		resp := validator.validate(
			context.Background(),
			storageAdmissionRequest(admissionv1.Delete, twoStoragePoolsWithNutanixCSI("nutanix"), nil),
		)
		expectStorageResponse(resp, nil)
	})

	It("counts a storage label with an empty value", func() {
		cluster := withCSIProviders(
			newStorageTestCluster("nutanix", nodePool{name: "md-0"}),
			v1alpha1.CSIProviderNutanix,
		)
		cluster.Spec.Topology.Workers.MachineDeployments[0].Metadata.Labels = map[string]string{
			storageNodePoolLabelKey: "",
		}
		resp := validator.validate(context.Background(), storageAdmissionRequest(admissionv1.Create, cluster, nil))
		expectStorageResponse(resp, []deniedPool{withNutanixCSI(0, "md-0")})
	})

	It("treats an undecodable old clusterConfig as already having Nutanix CSI", func() {
		oldCluster := newStorageTestCluster("nutanix", nodePool{name: "storage-a", storage: storageOnly})
		oldCluster.Spec.Topology.Variables = []clusterv1beta2.ClusterVariable{{
			Name:  v1alpha1.ClusterConfigVariableName,
			Value: apiextensionsv1.JSON{Raw: []byte(`{"addons":"not-an-object"}`)},
		}}
		newCluster := withCSIProviders(
			newStorageTestCluster("nutanix",
				nodePool{name: "md-0", storage: computeAndStorage},
				nodePool{name: "storage-a", storage: storageOnly},
			),
			v1alpha1.CSIProviderNutanix,
		)
		resp := validator.validate(
			context.Background(),
			storageAdmissionRequest(admissionv1.Update, newCluster, oldCluster),
		)
		expectStorageResponse(resp, []deniedPool{
			added(0, "md-0", `storage node pool "storage-a"`),
			withNutanixCSI(0, "md-0"),
		})
	})

	It("does not count node pools that only carry the storage taint", func() {
		cluster := newStorageTestCluster("nutanix", nodePool{name: "md-0"}, nodePool{name: "md-1"})
		for i := range cluster.Spec.Topology.Workers.MachineDeployments {
			cluster.Spec.Topology.Workers.MachineDeployments[i].Variables.Overrides = []clusterv1beta2.ClusterVariable{
				storageTaintWorkerConfigVariable(),
			}
		}
		resp := validator.validate(context.Background(), storageAdmissionRequest(admissionv1.Create, cluster, nil))
		expectStorageResponse(resp, nil)
	})
})

type nodePool struct {
	name string
	// storage is the value of the storage label; empty means the label is absent.
	storage string
}

func newStorageTestCluster(provider string, pools ...nodePool) *clusterv1beta2.Cluster {
	mds := make([]clusterv1beta2.MachineDeploymentTopology, 0, len(pools))
	for _, pool := range pools {
		md := clusterv1beta2.MachineDeploymentTopology{Name: pool.name, Class: "default-worker"}
		if pool.storage != "" {
			md.Metadata.Labels = map[string]string{storageNodePoolLabelKey: pool.storage}
		}
		mds = append(mds, md)
	}
	return &clusterv1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
			Labels:    map[string]string{clusterv1beta2.ProviderNameLabel: provider},
		},
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				ClassRef: clusterv1beta2.ClusterClassRef{Name: "test-class"},
				Version:  "v1.30.0",
				Workers:  clusterv1beta2.WorkersTopology{MachineDeployments: mds},
			},
		},
	}
}

func withCSIProviders(cluster *clusterv1beta2.Cluster, providers ...string) *clusterv1beta2.Cluster {
	if len(providers) == 0 {
		return cluster
	}
	csiProviders := make(map[string]v1alpha1.CSIProvider, len(providers))
	for _, provider := range providers {
		csiProviders[provider] = v1alpha1.CSIProvider{}
	}
	variable, err := variables.MarshalToClusterVariable(v1alpha1.ClusterConfigVariableName, variables.ClusterConfigSpec{
		Addons: &variables.Addons{CSI: &variables.CSI{Providers: csiProviders}},
	})
	if err != nil {
		panic(err)
	}
	cluster.Spec.Topology.Variables = append(cluster.Spec.Topology.Variables, *variable)
	return cluster
}

func storageTaintWorkerConfigVariable() clusterv1beta2.ClusterVariable {
	workerConfig := variables.WorkerNodeConfigSpec{}
	workerConfig.Taints = []v1alpha1.Taint{{
		Key:    storageNodePoolLabelKey,
		Effect: v1alpha1.TaintEffectNoSchedule,
	}}
	raw, err := json.Marshal(workerConfig)
	Expect(err).NotTo(HaveOccurred())
	return clusterv1beta2.ClusterVariable{
		Name:  v1alpha1.WorkerConfigVariableName,
		Value: apiextensionsv1.JSON{Raw: raw},
	}
}

func storageAdmissionRequest(
	op admissionv1.Operation, cluster, oldCluster *clusterv1beta2.Cluster,
) admission.Request {
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: op}}
	raw, err := json.Marshal(cluster)
	Expect(err).NotTo(HaveOccurred())
	req.Object = runtime.RawExtension{Raw: raw}
	if oldCluster != nil {
		oldRaw, err := json.Marshal(oldCluster)
		Expect(err).NotTo(HaveOccurred())
		req.OldObject = runtime.RawExtension{Raw: oldRaw}
	}
	return req
}

type deniedPool struct {
	index   int
	message string
}

func oneOfMultiple(index int, name string, count int) deniedPool {
	return deniedPool{index, fmt.Sprintf(
		`node pool %q is one of %d storage node pools (label "node-role.kubernetes.io/storage"); `+
			"at most one is supported per cluster, remove the label from all but one node pool",
		name, count,
	)}
}

func added(index int, name, existing string) deniedPool {
	return deniedPool{index, fmt.Sprintf(
		`node pool %q cannot be added as a storage node pool (label "node-role.kubernetes.io/storage") `+
			"because the cluster already has %s; remove the label from %q or remove the node pool",
		name, existing, name,
	)}
}

func withNutanixCSI(index int, name string) deniedPool {
	return deniedPool{index, fmt.Sprintf(
		`node pool %q is a storage node pool (label "node-role.kubernetes.io/storage"), which is not `+
			`supported when the "nutanix" CSI provider is configured; `+
			"remove the label from the node pool",
		name,
	)}
}

func expectStorageResponse(resp admission.Response, expectedDenied []deniedPool) {
	if len(expectedDenied) == 0 {
		Expect(resp.Allowed).To(BeTrue(), "unexpected denial: %v", resp.Result)
		return
	}
	Expect(resp.Allowed).To(BeFalse())
	Expect(resp.Result.Code).To(Equal(int32(http.StatusUnprocessableEntity)))
	Expect(resp.Result.Details.Causes).To(HaveLen(len(expectedDenied)))
	for i, want := range expectedDenied {
		cause := resp.Result.Details.Causes[i]
		Expect(cause.Field).To(Equal(fmt.Sprintf("spec.topology.workers.machineDeployments[%d]", want.index)))
		Expect(cause.Message).To(ContainSubstring(want.message))
		Expect(resp.Result.Message).To(ContainSubstring(want.message))
	}
}
