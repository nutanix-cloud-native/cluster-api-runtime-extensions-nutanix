//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type spyLogCollector struct {
	machineCalls     int
	machinePoolCalls int
	infraCalls       int
	err              error
}

func (s *spyLogCollector) CollectMachineLog(
	_ context.Context,
	_ client.Client,
	_ *clusterv1.Machine,
	_ string,
) error {
	s.machineCalls++
	return s.err
}

func (s *spyLogCollector) CollectMachinePoolLog(
	_ context.Context,
	_ client.Client,
	_ *clusterv1.MachinePool,
	_ string,
) error {
	s.machinePoolCalls++
	return s.err
}

func (s *spyLogCollector) CollectInfrastructureLogs(
	_ context.Context,
	_ client.Client,
	_ *clusterv1.Cluster,
	_ string,
) error {
	s.infraCalls++
	return s.err
}

func TestClusterLogCollector_CollectMachineLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		machine     *clusterv1.Machine
		wantCollect bool
	}{
		{
			name: "DevMachine delegates to Docker collector",
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "cp-0"},
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "DevMachine",
					},
				},
			},
			wantCollect: true,
		},
		{
			name: "DockerMachine delegates to Docker collector",
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "cp-0"},
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "DockerMachine",
					},
				},
			},
			wantCollect: true,
		},
		{
			name: "NutanixMachine is skipped",
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "nutanix-cp-0"},
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "NutanixMachine",
					},
				},
			},
			wantCollect: false,
		},
		{
			name: "AWSMachine is skipped",
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{Name: "aws-cp-0"},
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSMachine",
					},
				},
			},
			wantCollect: false,
		},
		{
			name:        "unset InfrastructureRef is skipped",
			machine:     &clusterv1.Machine{},
			wantCollect: false,
		},
		{
			name:        "nil Machine is skipped",
			machine:     nil,
			wantCollect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spy := &spyLogCollector{}
			collector := dockerClusterLogCollector{docker: spy}

			err := collector.CollectMachineLog(t.Context(), nil, tt.machine, "/tmp/out")
			require.NoError(t, err)

			expectedCalls := 0
			if tt.wantCollect {
				expectedCalls = 1
			}
			assert.Equal(t, expectedCalls, spy.machineCalls)
		})
	}
}

func TestClusterLogCollector_CollectMachinePoolLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		machinePool *clusterv1.MachinePool
		wantCollect bool
	}{
		{
			name: "DevMachinePool delegates to Docker collector",
			machinePool: &clusterv1.MachinePool{
				Spec: clusterv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: clusterv1.ContractVersionedObjectReference{
								Kind: "DevMachinePool",
							},
						},
					},
				},
			},
			wantCollect: true,
		},
		{
			name: "DockerMachinePool delegates to Docker collector",
			machinePool: &clusterv1.MachinePool{
				Spec: clusterv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: clusterv1.ContractVersionedObjectReference{
								Kind: "DockerMachinePool",
							},
						},
					},
				},
			},
			wantCollect: true,
		},
		{
			name: "non-Docker MachinePool is skipped",
			machinePool: &clusterv1.MachinePool{
				Spec: clusterv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: clusterv1.ContractVersionedObjectReference{
								Kind: "AWSManagedMachinePool",
							},
						},
					},
				},
			},
			wantCollect: false,
		},
		{
			name:        "nil MachinePool is skipped",
			machinePool: nil,
			wantCollect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spy := &spyLogCollector{}
			collector := dockerClusterLogCollector{docker: spy}

			err := collector.CollectMachinePoolLog(t.Context(), nil, tt.machinePool, "/tmp/out")
			require.NoError(t, err)

			expectedCalls := 0
			if tt.wantCollect {
				expectedCalls = 1
			}
			assert.Equal(t, expectedCalls, spy.machinePoolCalls)
		})
	}
}

func TestClusterLogCollector_CollectInfrastructureLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cluster     *clusterv1.Cluster
		wantCollect bool
	}{
		{
			name: "DevCluster delegates to Docker collector",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "docker-cluster"},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "DevCluster",
					},
				},
			},
			wantCollect: true,
		},
		{
			name: "DockerCluster delegates to Docker collector",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "docker-cluster"},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "DockerCluster",
					},
				},
			},
			wantCollect: true,
		},
		{
			name: "NutanixCluster is skipped",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "nutanix-cluster"},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "NutanixCluster",
					},
				},
			},
			wantCollect: false,
		},
		{
			name: "AWSCluster is skipped",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "aws-cluster"},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			wantCollect: false,
		},
		{
			name:        "unset InfrastructureRef is skipped",
			cluster:     &clusterv1.Cluster{},
			wantCollect: false,
		},
		{
			name:        "nil Cluster is skipped",
			cluster:     nil,
			wantCollect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spy := &spyLogCollector{}
			collector := dockerClusterLogCollector{docker: spy}

			err := collector.CollectInfrastructureLogs(t.Context(), nil, tt.cluster, "/tmp/out")
			require.NoError(t, err)

			expectedCalls := 0
			if tt.wantCollect {
				expectedCalls = 1
			}
			assert.Equal(t, expectedCalls, spy.infraCalls)
		})
	}
}

func TestClusterLogCollector_PropagatesDockerErrorsAndHonorsSkip(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("docker exec failed")
	spy := &spyLogCollector{err: wantErr}
	collector := dockerClusterLogCollector{docker: spy}

	dockerMachine := &clusterv1.Machine{
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{Kind: "DevMachine"},
		},
	}
	err := collector.CollectMachineLog(t.Context(), nil, dockerMachine, "/tmp/out")
	require.ErrorIs(t, err, wantErr)

	skipped := newClusterLogCollector(true)
	require.NoError(t, skipped.CollectMachineLog(t.Context(), nil, dockerMachine, "/tmp/out"))
	require.NoError(
		t,
		skipped.CollectMachinePoolLog(
			t.Context(),
			nil,
			&clusterv1.MachinePool{
				Spec: clusterv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							InfrastructureRef: clusterv1.ContractVersionedObjectReference{
								Kind: "DevMachinePool",
							},
						},
					},
				},
			},
			"/tmp/out",
		),
	)
	require.NoError(
		t,
		skipped.CollectInfrastructureLogs(
			t.Context(),
			nil,
			&clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "DevCluster",
					},
				},
			},
			"/tmp/out",
		),
	)
}
