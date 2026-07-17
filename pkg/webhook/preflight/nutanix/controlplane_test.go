// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

// listenTCP starts a TCP listener on a random port and returns it.
// The caller is responsible for closing the listener.
func listenTCP(t *testing.T) *net.TCPListener {
	t.Helper()
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	return l
}

func clusterWithControlPlaneEndpoint(host string, port int32) *clusterv1beta2.Cluster {
	return &clusterv1beta2.Cluster{
		Spec: clusterv1beta2.ClusterSpec{
			ControlPlaneEndpoint: clusterv1beta2.APIEndpoint{
				Host: host,
				Port: port,
			},
		},
	}
}

func TestControlPlaneEndpointCheck_ControlPlaneRefDefined(t *testing.T) {
	t.Parallel()

	cluster := clusterWithControlPlaneEndpoint("127.0.0.1", 6443)
	cluster.Spec.ControlPlaneRef = clusterv1beta2.ContractVersionedObjectReference{
		Name:     "test-kcp",
		Kind:     "KubeadmControlPlane",
		APIGroup: "controlplane.cluster.x-k8s.io",
	}
	check := &controlPlaneEndpointCheck{cluster: cluster}
	result := check.Run(context.Background())

	assert.Equal(t, preflight.CheckResult{Allowed: true}, result)
}

func TestControlPlaneEndpointCheck_EndpointAcceptsConnection(t *testing.T) {
	t.Parallel()

	// Start a real TCP listener so the dial succeeds.
	listener := listenTCP(t)
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	cluster := clusterWithControlPlaneEndpoint("127.0.0.1", int32(addr.Port))

	check := &controlPlaneEndpointCheck{cluster: cluster}
	result := check.Run(context.Background())

	expectedAddr := fmt.Sprintf("127.0.0.1:%d", addr.Port)
	assert.False(t, result.Allowed)
	require.Len(t, result.Causes, 1)
	assert.Contains(t, result.Causes[0].Message, expectedAddr)
	assert.Contains(t, result.Causes[0].Message, "established a TCP connection")
}

func TestControlPlaneEndpointCheck_EndpointRefusesConnection(t *testing.T) {
	t.Parallel()

	// Bind a listener, record its port, then close it immediately so the
	// port is free but nothing is accepting connections.
	listener := listenTCP(t)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cluster := clusterWithControlPlaneEndpoint("127.0.0.1", int32(port))

	check := &controlPlaneEndpointCheck{cluster: cluster}
	result := check.Run(context.Background())

	assert.Equal(t, preflight.CheckResult{Allowed: true}, result)
}

func TestControlPlaneEndpointCheck_EndpointTimeout(t *testing.T) {
	t.Parallel()

	cluster := clusterWithControlPlaneEndpoint("127.0.0.1", 6443)

	check := &controlPlaneEndpointCheck{cluster: cluster}
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately to simulate a timeout.
	result := check.Run(cancelledCtx)

	assert.Equal(t, preflight.CheckResult{Allowed: true}, result)
}
