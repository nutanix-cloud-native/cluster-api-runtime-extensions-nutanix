//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2eframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This code was inspired from kubernetes/kubernetes, specifically
// https://github.com/oomichi/kubernetes/blob/master/test/e2e/framework/log.go

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func logf(level, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf prints info logs with a timestamp and formatting.
func Logf(format string, args ...interface{}) {
	logf("INFO", format, args...)
}

// LogWarningf prints warning logs with a timestamp and formatting.
func LogWarningf(format string, args ...interface{}) {
	logf("WARNING", format, args...)
}

// Log prints info logs with a timestamp.
func Log(message string) {
	logf("INFO", message)
}

// LogWarning prints warning logs with a timestamp.
func LogWarning(message string) {
	logf("WARNING", message)
}

// dockerClusterLogCollector delegates workload log collection to CAPD's DockerLogCollector
// only for Docker-backed resources, avoiding bogus container exec/inspect errors on
// non-Docker providers such as Nutanix and AWS.
type dockerClusterLogCollector struct {
	docker capie2eframework.ClusterLogCollector
}

func newClusterLogCollector(skip bool) capie2eframework.ClusterLogCollector {
	if skip {
		return dockerClusterLogCollector{}
	}

	return dockerClusterLogCollector{
		docker: capie2eframework.DockerLogCollector{},
	}
}

func (c dockerClusterLogCollector) CollectMachineLog(
	ctx context.Context,
	managementClusterClient client.Client,
	m *clusterv1.Machine,
	outputPath string,
) error {
	if c.docker == nil || m == nil {
		return nil
	}
	if m.Spec.InfrastructureRef.Kind != "DevMachine" &&
		m.Spec.InfrastructureRef.Kind != "DockerMachine" {
		return nil
	}

	return c.docker.CollectMachineLog(ctx, managementClusterClient, m, outputPath)
}

func (c dockerClusterLogCollector) CollectMachinePoolLog(
	ctx context.Context,
	managementClusterClient client.Client,
	mp *clusterv1.MachinePool,
	outputPath string,
) error {
	if c.docker == nil || mp == nil {
		return nil
	}
	if mp.Spec.Template.Spec.InfrastructureRef.Kind != "DevMachinePool" &&
		mp.Spec.Template.Spec.InfrastructureRef.Kind != "DockerMachinePool" {
		return nil
	}

	return c.docker.CollectMachinePoolLog(ctx, managementClusterClient, mp, outputPath)
}

func (c dockerClusterLogCollector) CollectInfrastructureLogs(
	ctx context.Context,
	managementClusterClient client.Client,
	cluster *clusterv1.Cluster,
	outputPath string,
) error {
	if c.docker == nil || cluster == nil {
		return nil
	}
	if cluster.Spec.InfrastructureRef.Kind != "DevCluster" &&
		cluster.Spec.InfrastructureRef.Kind != "DockerCluster" {
		return nil
	}

	return c.docker.CollectInfrastructureLogs(ctx, managementClusterClient, cluster, outputPath)
}
