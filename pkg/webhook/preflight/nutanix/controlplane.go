// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func newControlPlaneEndpointChecks(
	cd *checkDependencies,
) []preflight.Check {
	if cd == nil || cd.cluster == nil {
		return []preflight.Check{}
	}

	return []preflight.Check{
		&controlPlaneEndpointCheck{cluster: cd.cluster},
	}
}

// controlPlaneEndpointCheck is a check that verifies that the control plane
// endpoint is not in use, before any control plane nodes have been created. If
// the endpoint is in use, the check should fail, to prevent the cluster from
// being created.
type controlPlaneEndpointCheck struct {
	cluster *clusterv1.Cluster
}

func (c *controlPlaneEndpointCheck) Name() string {
	return "NutanixControlPlaneEndpoint"
}

func (c *controlPlaneEndpointCheck) Run(ctx context.Context) preflight.CheckResult {
	if c.cluster.Spec.ControlPlaneRef.IsDefined() {
		// If the control plane reference is defined, then control plane nodes
		// are already being created, and it is too late to prevent the cluster
		// from being created, so this check is not relevant.
		return preflight.CheckResult{
			Allowed: true,
		}
	}

	// If the control plane reference is not defined, then no control plane
	// nodes have been created.
	//
	// We need to check if the endpoint establishes a TCP connection. If it
	// does, then the most likely conclusion is that the endpoint is in use by
	// another cluster.
	//
	// If the endpoint is a load balancer or proxy that accepts connections even
	// before any control plane nodes have been created, then the user should
	// skip this check.

	dialer := &net.Dialer{
		// The default TCP timeout is 60 seconds. We use a shorter timeout
		// because we want checks to return quickly. Most checks complete in
		// less than a few seconds. All checks run in parallel.
		Timeout: 3 * time.Second,
	}
	addr := net.JoinHostPort(
		c.cluster.Spec.ControlPlaneEndpoint.Host,
		strconv.Itoa(int(c.cluster.Spec.ControlPlaneEndpoint.Port)),
	)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return preflight.CheckResult{
			Allowed: true,
		}
	}

	conn.Close()
	return preflight.CheckResult{
		Allowed: false,
		Causes: []preflight.Cause{
			{
				//nolint:lll // Message is long.
				Message: fmt.Sprintf(
					"The control plane endpoint %s established a TCP connection, before any control plane nodes have been created. The endpoint is in use, possibly by another cluster.",
					addr,
				),
			},
		},
	}
}
