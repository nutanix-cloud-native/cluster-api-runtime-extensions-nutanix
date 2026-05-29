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

type controlPlaneEndpointCheck struct {
	cluster *clusterv1.Cluster
}

func (c *controlPlaneEndpointCheck) Name() string {
	return "NutanixControlPlaneEndpoint"
}

func (c *controlPlaneEndpointCheck) Run(ctx context.Context) preflight.CheckResult {
	if !c.cluster.Spec.ControlPlaneRef.IsDefined() {
		// The control plane reference is defined, so the control plane has been initialized,
		// and the endpoint may establish a TCP connection.
		return preflight.CheckResult{
			Allowed: true,
		}
	}

	// The control plane reference is not defined, so the control plane has not been initialized,
	// and if the endpoint establishes a TCP connection, then the most likely conclusion is that the
	// endpoint is in use by another cluster. It is possible that the endpoint is a load balancer or
	// proxy that accepts connections even if the control plane is not initialized, but in that
	// case, the user can skip this check.

	dialer := &net.Dialer{
		// The default TCP timeout is 60 seconds, which is too long for this check.
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
