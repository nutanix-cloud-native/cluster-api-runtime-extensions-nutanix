// Copyright 2024 Nutanix. All rights reserved.
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
	result := preflight.CheckResult{
		Allowed: true,
	}

	if !c.cluster.Spec.ControlPlaneRef.IsDefined() {
		result.Allowed = true
		return result
	}

	dialer := &net.Dialer{
		Timeout: 3 * time.Second,
	}
	addr := net.JoinHostPort(
		c.cluster.Spec.ControlPlaneEndpoint.Host,
		strconv.Itoa(int(c.cluster.Spec.ControlPlaneEndpoint.Port)),
	)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		result.Allowed = true
		return result
	}
	if conn != nil {
		conn.Close()
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			//nolint:lll // Message is long.
			Message: fmt.Sprintf(
				"The control plane endpoint %s established a TCP connection, before any control plane nodes have been created. The endpoint is in use, possibly by another cluster.",
				addr,
			),
		})
	}

	return result
}
