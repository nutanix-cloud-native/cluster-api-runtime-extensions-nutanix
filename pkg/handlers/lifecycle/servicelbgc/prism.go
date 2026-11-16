// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	subnetmodels "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	prismmodels "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/prism-go-client/converged"
	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismtypes "github.com/nutanix-cloud-native/prism-go-client/environment/types"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	// credentialsSecretDataKey is the key the Prism Central credentials Secret
	// stores the username and password under.
	credentialsSecretDataKey = "credentials"

	// reservedIPPageSize bounds one page of the reserved-IP listing. The list is
	// paged to exhaustion: a truncated page would read as "nothing of ours is
	// reserved" and leak every address past it.
	reservedIPPageSize = 100

	// unreserveTaskTimeout bounds the wait for one unreserve task. The hook
	// retries, so a timeout is reported rather than waited out.
	unreserveTaskTimeout = 60 * time.Second
	unreserveTaskPoll    = 2 * time.Second
)

// convergedClientCache keys Prism Central clients by credentials Secret, so a
// retried teardown reuses the connection instead of opening a new one.
//
//nolint:gochecknoglobals // A client cache has to outlive one hook invocation.
var convergedClientCache = convergedv4.NewClientCache()

// cacheParams adapts a management endpoint to the client cache's key interface.
type cacheParams struct {
	key      string
	endpoint prismtypes.ManagementEndpoint
}

func (c *cacheParams) ManagementEndpoint() prismtypes.ManagementEndpoint { return c.endpoint }

func (c *cacheParams) Key() string { return c.key }

// newReservationReleaser builds a Prism Central client for the cluster's
// credentials.
func newReservationReleaser(
	ctx context.Context,
	kclient ctrlclient.Client,
	namespace string,
	endpoint *carenv1.NutanixPrismCentralEndpointSpec,
) (reservationReleaser, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("prismCentralEndpoint is not set")
	}
	host, port, err := endpoint.ParseURL()
	if err != nil {
		return nil, fmt.Errorf("parse prismCentralEndpoint url: %w", err)
	}
	if endpoint.Credentials.SecretRef.Name == "" {
		return nil, fmt.Errorf("prismCentralEndpoint.credentials.secretRef.name is not set")
	}

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: namespace, Name: endpoint.Credentials.SecretRef.Name}
	if err := kclient.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("get credentials Secret %s: %w", secretKey, err)
	}
	data, ok := secret.Data[credentialsSecretDataKey]
	if !ok {
		return nil, fmt.Errorf("credentials Secret %s has no %q key", secretKey, credentialsSecretDataKey)
	}
	usernamePassword, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		return nil, fmt.Errorf("parse credentials from Secret %s: %w", secretKey, err)
	}

	var trustBundlePEM string
	if endpoint.AdditionalTrustBundle != "" {
		decoded, err := base64.StdEncoding.DecodeString(endpoint.AdditionalTrustBundle)
		if err != nil {
			return nil, fmt.Errorf("decode additionalTrustBundle: %w", err)
		}
		trustBundlePEM = string(decoded)
	}

	address, err := url.Parse(fmt.Sprintf("https://%s:%d", host, port))
	if err != nil {
		return nil, fmt.Errorf("parse prism central address: %w", err)
	}
	client, err := convergedClientCache.GetOrCreate(&cacheParams{
		key: secretKey.String(),
		endpoint: prismtypes.ManagementEndpoint{
			Address:               address,
			Insecure:              endpoint.Insecure,
			AdditionalTrustBundle: trustBundlePEM,
			ApiCredentials: prismtypes.ApiCredentials{
				Username: usernamePassword.Username,
				Password: usernamePassword.Password,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create Prism Central client: %w", err)
	}
	return &convergedReleaser{client: client}, nil
}

// convergedReleaser is the Prism Central implementation of reservationReleaser.
type convergedReleaser struct {
	client *convergedv4.Client
}

// ResolveSubnet returns the extID of the subnet named by subnetExtIDOrName.
//
// A name is matched in Go. Prism Central 7.6 answers HTTP 400 "Error while
// parsing URI" for a $filter holding any non-ASCII character, so the
// server-side name filter is only an optimisation and a name it cannot express
// falls back to a full list. The name is compared here either way, so the
// result never depends on the server's matching rules. This mirrors what the
// CCM does with the same field, so both sides resolve a name identically.
func (r *convergedReleaser) ResolveSubnet(
	ctx context.Context,
	subnetExtIDOrName, cluster string,
) (string, error) {
	if subnetExtIDOrName == "" {
		return "", fmt.Errorf("subnet extID or name is required")
	}
	if _, err := uuid.Parse(subnetExtIDOrName); err == nil {
		return subnetExtIDOrName, nil
	}

	clusterExtID := ""
	if cluster != "" {
		resolved, err := r.resolveCluster(ctx, cluster)
		if err != nil {
			return "", err
		}
		clusterExtID = resolved
	}

	subnets, err := r.listSubnetsByName(ctx, subnetExtIDOrName)
	if err != nil {
		return "", err
	}

	matched := make([]string, 0, 1)
	for i := range subnets {
		subnet := &subnets[i]
		if subnet.Name == nil || *subnet.Name != subnetExtIDOrName {
			continue
		}
		if subnet.ExtId == nil || *subnet.ExtId == "" {
			continue
		}
		// An overlay subnet declares no cluster, so only cluster-scoped subnets
		// are narrowed by it.
		if clusterExtID != "" && subnetDeclaresCluster(subnet) && !subnetInCluster(subnet, clusterExtID) {
			continue
		}
		matched = append(matched, *subnet.ExtId)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("%w: no subnet named %q", errNothingToRelease, subnetExtIDOrName)
	default:
		return "", fmt.Errorf("subnet name %q matches %d subnets (%s); use the subnet extID instead",
			subnetExtIDOrName, len(matched), strings.Join(matched, ", "))
	}
}

func (r *convergedReleaser) listSubnetsByName(
	ctx context.Context,
	name string,
) ([]subnetmodels.Subnet, error) {
	if isASCII(name) {
		filter := fmt.Sprintf("name eq '%s'", strings.ReplaceAll(name, "'", "''"))
		if subnets, err := r.client.Subnets.List(ctx, converged.WithFilter(filter)); err == nil {
			return subnets, nil
		}
	}
	subnets, err := r.client.Subnets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list subnets: %w", err)
	}
	return subnets, nil
}

func (r *convergedReleaser) resolveCluster(ctx context.Context, clusterExtIDOrName string) (string, error) {
	if _, err := uuid.Parse(clusterExtIDOrName); err == nil {
		return clusterExtIDOrName, nil
	}
	clusters, err := r.client.Clusters.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list clusters: %w", err)
	}
	matched := make([]string, 0, 1)
	for i := range clusters {
		if clusters[i].Name != nil && *clusters[i].Name == clusterExtIDOrName &&
			clusters[i].ExtId != nil && *clusters[i].ExtId != "" {
			matched = append(matched, *clusters[i].ExtId)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no cluster named %q", clusterExtIDOrName)
	default:
		return "", fmt.Errorf("cluster name %q matches %d clusters; use the cluster extID instead",
			clusterExtIDOrName, len(matched))
	}
}

// ReservedIPs pages the subnet's reservations to exhaustion.
func (r *convergedReleaser) ReservedIPs(ctx context.Context, subnetExtID string) ([]ReservedIP, error) {
	all := make([]ReservedIP, 0, 1)
	for page := 0; ; page++ {
		resp, err := r.client.Subnets.ListReservedIpsBySubnetId(ctx, subnetExtID,
			converged.WithPage(page), converged.WithLimit(reservedIPPageSize))
		if err != nil {
			if isUnreservableSubnet(err) {
				return nil, fmt.Errorf("%w: subnet %s: %w", errNothingToRelease, subnetExtID, err)
			}
			return nil, fmt.Errorf("list reserved IPs on subnet %s: %w", subnetExtID, err)
		}
		rows, err := reservedIPRows(resp)
		if err != nil {
			return nil, fmt.Errorf("list reserved IPs on subnet %s: %w", subnetExtID, err)
		}
		for i := range rows {
			row := &rows[i]
			reserved := ReservedIP{}
			if row.Ipv4Address != nil {
				reserved.Address = *row.Ipv4Address
			}
			if row.ClientContext != nil {
				reserved.ClientContext = *row.ClientContext
			}
			all = append(all, reserved)
		}
		if len(rows) < reservedIPPageSize {
			return all, nil
		}
	}
}

// reservedIPRows unwraps the reserved-IP list response. A subnet with no
// reservations returns a nil Data, which is not an error.
func reservedIPRows(resp any) ([]subnetmodels.ReservedIp, error) {
	if resp == nil {
		return nil, nil
	}
	listResp, ok := resp.(*subnetmodels.ListSubnetReservedIpsApiResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected reserved IP list response type %T", resp)
	}
	data := listResp.GetData()
	if data == nil {
		return nil, nil
	}
	rows, ok := data.([]subnetmodels.ReservedIp)
	if !ok {
		return nil, fmt.Errorf("unexpected reserved IP list data type %T", data)
	}
	return rows, nil
}

// UnreserveContext releases every address held under clientContext and waits
// for the Prism task. The 202 is not success: the task carries the outcome.
func (r *convergedReleaser) UnreserveContext(ctx context.Context, subnetExtID, clientContext string) error {
	unreserveType := subnetmodels.UNRESERVETYPE_CONTEXT
	spec := &subnetmodels.IpUnreserveSpec{
		UnreserveType: &unreserveType,
		ClientContext: &clientContext,
	}
	// No NTNX-Request-Id is set. An unreserve by context is idempotent, so there
	// is nothing to deduplicate, and Prism replays a repeated request id as the
	// original task, which would report a release that did not happen.
	taskRef, err := r.client.Subnets.UnreserveIpsBySubnetId(ctx, subnetExtID, spec)
	if err != nil {
		if isReservationGone(err) {
			return nil
		}
		return fmt.Errorf("unreserve context %s on subnet %s: %w", clientContext, subnetExtID, err)
	}
	if taskRef == nil || taskRef.ExtId == nil || *taskRef.ExtId == "" {
		return fmt.Errorf("unreserve context %s returned no task", clientContext)
	}
	return r.waitTask(ctx, *taskRef.ExtId)
}

func (r *convergedReleaser) waitTask(ctx context.Context, taskExtID string) error {
	var task *prismmodels.Task
	err := kwait.PollUntilContextTimeout(ctx, unreserveTaskPoll, unreserveTaskTimeout, true,
		func(ctx context.Context) (bool, error) {
			current, err := r.client.Tasks.Get(ctx, taskExtID)
			if err != nil {
				return false, nil // transient; keep polling until the deadline
			}
			task = current
			if current == nil || current.Status == nil {
				return false, nil
			}
			switch *current.Status {
			case prismmodels.TASKSTATUS_SUCCEEDED:
				return true, nil
			case prismmodels.TASKSTATUS_FAILED,
				prismmodels.TASKSTATUS_CANCELED,
				prismmodels.TASKSTATUS_CANCELING:
				return false, fmt.Errorf("unreserve task %s: %s", taskExtID, taskFailure(current))
			default:
				return false, nil
			}
		})
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("unreserve task %s was never read", taskExtID)
	}
	return nil
}

// taskFailure reports every message the task carried, verbatim: the v4 message
// is often a pointer to legacyErrorMessage, which holds the real cause.
func taskFailure(task *prismmodels.Task) string {
	messages := make([]string, 0, 2)
	for _, m := range task.ErrorMessages {
		if m.Message != nil && *m.Message != "" {
			messages = append(messages, *m.Message)
		}
	}
	if task.LegacyErrorMessage != nil && *task.LegacyErrorMessage != "" {
		messages = append(messages, *task.LegacyErrorMessage)
	}
	if len(messages) == 0 {
		return "no error message"
	}
	return strings.Join(messages, "; ")
}

// isReservationGone reports whether err means there was nothing to release,
// which makes the sweep idempotent.
func isReservationGone(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

// isUnreservableSubnet reports whether the subnet cannot hold a reservation of
// ours, so listing its reservations is not a failure to retry.
//
// An unmanaged subnet answers NETWORKING-10056 "Cannot fetch reserved IP
// addresses for unmanaged subnets" (measured on PC 7.6 GA). Nothing could have
// been reserved on it, so there is nothing to release. A subnet that is gone
// took its reservations with it.
func isUnreservableSubnet(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unmanaged subnet") ||
		strings.Contains(message, "networking-10056") ||
		strings.Contains(message, "not found")
}

func subnetDeclaresCluster(subnet *subnetmodels.Subnet) bool {
	return (subnet.ClusterReference != nil && *subnet.ClusterReference != "") ||
		len(subnet.ClusterReferenceList) > 0
}

func subnetInCluster(subnet *subnetmodels.Subnet, clusterExtID string) bool {
	if subnet.ClusterReference != nil && *subnet.ClusterReference == clusterExtID {
		return true
	}
	return slices.Contains(subnet.ClusterReferenceList, clusterExtID)
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}
