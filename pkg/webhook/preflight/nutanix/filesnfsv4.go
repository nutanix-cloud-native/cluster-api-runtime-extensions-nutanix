// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

/*
NutanixFilesNFSv4 runs only for Cluster updates; create operations pass silently.


Flow:
preflight runner → nutanixChecker.Init() → credentials check(validates Prism Central credentials and stores them as fallback) →
newFilesNFSv4Checks() → Run() creates workload-cluster client →
listNutanixFilesStorageClasses(lists Files StorageClasses(storageType=NutanixFiles) → evaluates each StorageClass.)

SC Evaluation:
  - Skips evaluation when mountOptions contains nfsvers=4* or vers=4*.
  - Resolve file server from StorageClass:
	- nfsServerName for dynamic provisioning.
	- nfsServer for static provisioning.
	- If neither exists, add warning and skip that StorageClass.
  - Resolve credentials from CSI provisioner Secret.
	- If Secret is missing, unreadable, malformed, or lacks files-key, fall back to validated Prism Central credentials.
  - Queries the file server’s name-services API to get NFS version.
	- NFSV4/NFSV3V4: allow that StorageClass.
	- UNKNOWN: warning, allow.
	- NFSV3: add blocking cause.
  - Validate NFS version
  - StorageClass evaluation continues independently after one warning.

Validation, Errors:
- Fail-open cases produce warnings and leave upgrade allowed:
	- Errors encountered while inspecting an individual Files StorageClass fail open for this check
	- only confirmed NFSv3-only servers block the upgrade.
- Confirmed NFSV4 or NFSV3V4 allows upgrade.
- Only a confirmed NFSV3-only server without an explicit NFSv4 mount option blocks upgrade by adding a preflight cause for that StorageClass.
*/

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	filesapi "github.com/nutanix/ntnx-api-golang-clients/files-go-client/v4/api"
	filesclient "github.com/nutanix/ntnx-api-golang-clients/files-go-client/v4/client"
	filesconfig "github.com/nutanix/ntnx-api-golang-clients/files-go-client/v4/models/files/v4/config"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	// nutanixCSIProvisioner is the provisioner name used by the Nutanix CSI driver.
	nutanixCSIProvisioner = "csi.nutanix.com"

	// StorageClass "storageType" parameter key identifies the backing storage type of a Nutanix CSI StorageClass.
	csiParameterKeyStorageType = "storageType"
	// StorageClass "storageType" parameter value that specifies Nutanix Files.
	csiStorageTypeNutanixFiles = "NutanixFiles"
	// StorageClass "nfsServerName" parameter key identifies the Nutanix Files file server backing a
	// StorageClass by name. It is used for dynamic provisioning (dynamicProv=ENABLED) and is
	// resolvable through Prism Central.
	csiParameterKeyNFSServerName = "nfsServerName"
	// StorageClass "nfsServer" parameter key identifies the Nutanix Files file server by its mount
	// endpoint FQDN/IP. Static StorageClasses carry this (plus "nfsPath") instead of "nfsServerName";
	// an FQDN is reachable directly but is not resolvable by name through Prism Central.
	csiParameterKeyNFSServer = "nfsServer"

	// csiParameterKeyProvisionerSecretName and csiParameterKeyProvisionerSecretNamespace identify the
	// CSI provisioner Secret (on the workload cluster) used to authenticate to Nutanix Files.
	csiParameterKeyProvisionerSecretName      = "csi.storage.k8s.io/provisioner-secret-name"
	csiParameterKeyProvisionerSecretNamespace = "csi.storage.k8s.io/provisioner-secret-namespace"

	// filesKeySecretDataKey holds a "fqdn:username:password" value used to reach a Nutanix Files
	// file server directly, matching the CSI behavior for files-key StorageClasses.
	filesKeySecretDataKey = "files-key"

	// defaultNutanixFilesAPIPort is the default port used to reach the Nutanix Files v4 API.
	defaultNutanixFilesAPIPort = 9440

	// Prism Central-scoped (plural) file-servers collection endpoint.
	filesPCFileServersURI = "/api/files/v4.0.a2/config/file-servers"

	// Prism Central-scoped name-services endpoint, scoped to a specific file server by its external ID (UUID).
	filesPCNameServicesURIFormat = "/api/files/v4.0.a2/config/file-servers/%s/name-services"

	// filesPCListPageSize and filesPCListMaxPages bound pagination of the Prism Central file-servers
	// collection (the v4 API caps $limit at 100).
	filesPCListPageSize = 100
	filesPCListMaxPages = 100

	// filesNFSv4SubCallTimeout bounds each external call (remote SC list, each Files lookup) so a
	// single slow endpoint cannot consume the shared preflight check budget or hard-block the upgrade.
	filesNFSv4SubCallTimeout = 5 * time.Second
)

// NFS protocol version configured on a Nutanix Files server.
type nfsVersion string

const (
	nfsVersionUnknown nfsVersion = "UNKNOWN"
	nfsVersionV3      nfsVersion = "NFSV3"
	nfsVersionV4      nfsVersion = "NFSV4"
	nfsVersionV3V4    nfsVersion = "NFSV3V4"
)

// supportsNFSv4 reports whether the file server allows NFSv4 mounts.
func (v nfsVersion) supportsNFSv4() bool {
	return v == nfsVersionV4 || v == nfsVersionV3V4
}

// filesNFSVersionGetter returns the NFS version configured on a Nutanix Files file server.
type filesNFSVersionGetter interface {
	// NFSVersion returns the file server's configured NFS version.
	NFSVersion(ctx context.Context, fileServer string) (nfsVersion, error)
}

// fileServerRef identifies a StorageClass's Nutanix Files server and how it can be looked up.
type fileServerRef struct {
	// identifier is the file server name (dynamic SCs, via "nfsServerName") or the mount endpoint
	// FQDN/IP (static SCs, via "nfsServer").
	identifier string
	// isFQDN reports whether identifier is a mount endpoint FQDN/IP (static SC) rather than a file
	// server name (dynamic SC). An FQDN is reachable directly but cannot be resolved by name through
	// Prism Central.
	isFQDN bool
}

// filesNFSVersionGetterFactory builds a filesNFSVersionGetter using credentials resolved for a
// candidate StorageClass. remoteClient is the workload-cluster client used to read the SC's CSI
// provisioner Secret; pcFallback are CAREN's Prism Central credentials, used when the SC Secret is
// absent or unusable. It returns an error when no usable credentials can be resolved.
type filesNFSVersionGetterFactory func(
	ctx context.Context,
	remoteClient ctrlclient.Client,
	sc *storagev1.StorageClass,
	fsRef fileServerRef,
	pcFallback *prismgoclient.Credentials,
) (filesNFSVersionGetter, error)

type filesNFSv4Check struct {
	cluster    *clusterv1.Cluster
	oldCluster *clusterv1.Cluster
	kclient    ctrlclient.Client
	log        logr.Logger

	// pcCredentials are CAREN's validated Prism Central credentials, used as a fallback source of
	// Nutanix Files credentials when a StorageClass does not carry files secret.
	pcCredentials *prismgoclient.Credentials

	// clusterClientGetter builds a controller-runtime client for the workload cluster from its
	// kubeconfig Secret.
	clusterClientGetter remote.ClusterClientGetter

	// nfsVersionGetterFactory resolves credentials and builds a filesNFSVersionGetter per StorageClass.
	nfsVersionGetterFactory filesNFSVersionGetterFactory
}

func (c *filesNFSv4Check) Name() string {
	return "NutanixFilesNFSv4"
}

func (c *filesNFSv4Check) Run(ctx context.Context) preflight.CheckResult {
	c.log.V(5).Info("Running Nutanix Files NFSv4 preflight check")

	result := preflight.CheckResult{Allowed: true}

	// Trigger gating. Evaluated here (not at registration time) so the check stays registered, and
	// therefore skippable and covered by the framework's panic recovery.
	//
	// This check runs on every update and skips only pure creates: a
	// create has no existing workload cluster (and therefore no pre-existing NFS mounts to protect),
	// so there is nothing to validate. Non-actionable update cases still fall through to the
	// fail-open handling below (a missing workload-cluster kubeconfig or absence of Nutanix Files
	// StorageClasses simply warn + allow).
	if !c.isUpdate() {
		c.log.V(5).Info(
			"Skipping Nutanix Files NFSv4 preflight check: not a Cluster update "+
				"(create operation has no existing workload cluster to protect)",
			"operation", "create",
		)
		return result
	}

	// StorageClasses live on the workload cluster, so build a remote client from its kubeconfig Secret.
	// A missing Secret (control plane not ready) or an unreachable cluster is unresolvable: warn + allow.
	remoteClient, err := c.clusterClientGetter(ctx, "", c.kclient, ctrlclient.ObjectKeyFromObject(c.cluster))
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Could not connect to the workload cluster to inspect StorageClasses (%s); skipping Nutanix Files NFSv4 preflight.",
			err,
		))
		return result
	}

	candidates, err := listNutanixFilesStorageClasses(ctx, remoteClient)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Could not list StorageClasses on the workload cluster (%s); skipping Nutanix Files NFSv4 preflight.",
			err,
		))
		return result
	}

	// No StorageClass uses Nutanix Files, so there is nothing to check.
	if len(candidates) == 0 {
		c.log.V(5).Info("No Nutanix Files StorageClasses found on the workload cluster; nothing to check")
		return result
	}

	candidateNames := make([]string, len(candidates))
	for i := range candidates {
		candidateNames[i] = candidates[i].Name
	}
	c.log.V(5).Info(
		"Evaluating Nutanix Files StorageClasses",
		"count", len(candidates),
		"storageClasses", candidateNames,
	)

	// Each candidate is evaluated independently; results aggregate into this single CheckResult.
	for i := range candidates {
		c.evaluateStorageClass(ctx, remoteClient, &candidates[i], &result)
	}

	c.log.V(5).Info(
		"Completed Nutanix Files NFSv4 preflight check",
		"allowed", result.Allowed,
		"causes", len(result.Causes),
		"warnings", len(result.Warnings),
	)
	return result
}

// isUpdate reports whether the admission request is a Cluster update (as opposed to a create). Only
// updates can affect a running workload cluster with pre-existing NFS mounts; a create has no
// existing workload cluster (and therefore no StorageClasses) to protect.
func (c *filesNFSv4Check) isUpdate() bool {
	// Create operation: there is no existing (old) Cluster, hence no workload cluster yet.
	return c.oldCluster != nil
}

// evaluateStorageClass evaluates a single candidate StorageClass and folds its outcome into result.
func (c *filesNFSv4Check) evaluateStorageClass(
	ctx context.Context,
	remoteClient ctrlclient.Client,
	sc *storagev1.StorageClass,
	result *preflight.CheckResult,
) {
	// Step A: mountOptions escape hatch. An SC that pins NFSv4 is safe regardless of the file server.
	if storageClassPinsNFSv4(sc.MountOptions) {
		c.log.V(5).Info(
			"StorageClass pins NFSv4 via mountOptions; skipping file server lookup",
			"storageClass", sc.Name,
		)
		return
	}

	// Step B: identify the file server. Dynamic SCs reference it by name ("nfsServerName"); static
	// SCs reference it by mount endpoint FQDN/IP ("nfsServer"). Prefer the name, which Prism Central
	// can resolve; fall back to the FQDN, which is reachable directly.
	fsRef, ok := fileServerRefFromStorageClass(sc)
	if !ok {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"StorageClass %q specifies neither %q nor %q, so its Nutanix Files server cannot be identified; "+
				"skipping its NFSv4 check.",
			sc.Name,
			csiParameterKeyNFSServerName,
			csiParameterKeyNFSServer,
		))
		return
	}
	fileServer := fsRef.identifier
	c.log.V(5).Info(
		"Resolving NFS version for Nutanix Files StorageClass",
		"storageClass", sc.Name,
		"fileServer", fileServer,
		"identifiedByFQDN", fsRef.isFQDN,
	)

	getter, err := c.nfsVersionGetterFactory(ctx, remoteClient, sc, fsRef, c.pcCredentials)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Could not resolve credentials to check the NFS version of Nutanix Files server %q for StorageClass %q (%s); "+
				"skipping its NFSv4 check.",
			fileServer,
			sc.Name,
			err,
		))
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, filesNFSv4SubCallTimeout)
	version, err := getter.NFSVersion(lookupCtx, fileServer)
	cancel()
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"Could not determine the NFS version of Nutanix Files server %q for StorageClass %q (%s); skipping its NFSv4 check.",
			fileServer,
			sc.Name,
			err,
		))
		return
	}

	// An unknown version (empty or unparseable response, redacted value, missing field) is not a
	// confirmed NFSv3-only server, so it must never block the upgrade: warn + allow.
	if version == nfsVersionUnknown {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"The NFS version of Nutanix Files server %q for StorageClass %q could not be determined "+
				"(the Files API returned no usable value); skipping its NFSv4 check.",
			fileServer,
			sc.Name,
		))
		return
	}

	c.log.V(5).Info(
		"Resolved NFS version for Nutanix Files server",
		"storageClass", sc.Name,
		"fileServer", fileServer,
		"nfsVersion", string(version),
		"supportsNFSv4", version.supportsNFSv4(),
	)

	if version.supportsNFSv4() {
		return
	}

	// Confirmed NFSv3-only file server with no NFSv4 mountOption: block the upgrade.
	c.log.V(5).Info(
		"Nutanix Files server is NFSv3-only; blocking the upgrade",
		"storageClass", sc.Name,
		"fileServer", fileServer,
	)
	result.Allowed = false
	result.Causes = append(result.Causes, preflight.Cause{
		Message: fmt.Sprintf(
			"StorageClass %q uses Nutanix Files server %q which is configured for NFSv3 only. NKP upgrade requires NFSv4: "+
				"either enable NFSv4 on the file server, or add `nfsvers=4.1` to the StorageClass `mountOptions`.",
			sc.Name,
			fileServer,
		),
		Field: fmt.Sprintf("StorageClass/%s", sc.Name),
	})
}

// fileServerRefFromStorageClass resolves how to identify a StorageClass's Nutanix Files server. It
// prefers the file server name ("nfsServerName", dynamic SCs) over the mount endpoint FQDN/IP
// ("nfsServer", static SCs). It returns ok=false when the SC carries neither.
func fileServerRefFromStorageClass(sc *storagev1.StorageClass) (fileServerRef, bool) {
	if name := strings.TrimSpace(sc.Parameters[csiParameterKeyNFSServerName]); name != "" {
		return fileServerRef{identifier: name, isFQDN: false}, true
	}
	if fqdn := strings.TrimSpace(sc.Parameters[csiParameterKeyNFSServer]); fqdn != "" {
		return fileServerRef{identifier: fqdn, isFQDN: true}, true
	}
	return fileServerRef{}, false
}

// storageClassPinsNFSv4 reports whether any mount option pins NFSv4, i.e. an "nfsvers="/"vers="
// option whose value starts with "4" (4, 4.0, 4.1, 4.2).
func storageClassPinsNFSv4(mountOptions []string) bool {
	for _, opt := range mountOptions {
		key, value, found := strings.Cut(strings.TrimSpace(opt), "=")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "nfsvers", "vers":
			if strings.HasPrefix(strings.TrimSpace(value), "4") {
				return true
			}
		}
	}
	return false
}

// Lists StorageClasses on the workload cluster and returns those
// provisioned by the Nutanix CSI driver with storageType=NutanixFiles.
func listNutanixFilesStorageClasses(
	ctx context.Context,
	remoteClient ctrlclient.Client,
) ([]storagev1.StorageClass, error) {
	listCtx, cancel := context.WithTimeout(ctx, filesNFSv4SubCallTimeout)
	defer cancel()

	scList := &storagev1.StorageClassList{}
	if err := remoteClient.List(listCtx, scList); err != nil {
		return nil, err
	}

	candidates := make([]storagev1.StorageClass, 0, len(scList.Items))
	for i := range scList.Items {
		sc := scList.Items[i]
		if sc.Provisioner != nutanixCSIProvisioner {
			continue
		}
		if sc.Parameters[csiParameterKeyStorageType] != csiStorageTypeNutanixFiles {
			continue
		}
		candidates = append(candidates, sc)
	}
	return candidates, nil
}

func newFilesNFSv4Checks(cd *checkDependencies) []preflight.Check {
	// Gate on the same preconditions as the other Prism Central-dependent checks: a validated Prism
	// Central (pcVersion set) and the objects needed to reach the workload cluster.
	if cd == nil || cd.cluster == nil || cd.kclient == nil || cd.pcVersion == "" {
		return nil
	}

	return []preflight.Check{
		&filesNFSv4Check{
			cluster:                 cd.cluster,
			oldCluster:              cd.oldCluster,
			kclient:                 cd.kclient,
			log:                     cd.log,
			pcCredentials:           cd.pcCredentials,
			clusterClientGetter:     remote.NewClusterClient,
			nfsVersionGetterFactory: newFilesNFSVersionGetter,
		},
	}
}

// filesConnection holds the endpoint and credentials used to reach the Nutanix Files v4 API.
type filesConnection struct {
	host     string
	port     int
	username string
	password string

	// insecure disables TLS certificate verification. It is always true, matching the Nutanix CSI
	// driver, which connects to Nutanix Files (both the files-key file server and the Prism Central
	// fallback) with Insecure: true (k8s-csi pkg/util/client.go V4ClientParams).
	insecure bool

	// pcScoped selects the Prism Central-scoped (plural) Files API paths. Prism Central only serves
	// the plural collection endpoints and rejects the file server-direct (singular) paths the SDK
	// targets by default. When false, the file server-direct (singular) SDK path is used, which is
	// correct when connecting to a file server directly (the files-key Secret case).
	pcScoped bool

	// matchByFQDN, when pcScoped is true, matches the file server identifier against each file
	// server's name/FQDN/IP addresses (static SCs, whose identifier is a mount endpoint FQDN/IP)
	// instead of matching on name/extId only (dynamic SCs).
	matchByFQDN bool
}

// filesClient is the concrete filesNFSVersionGetter backed by the Nutanix files-go-client.
type filesClient struct {
	conn filesConnection
}

func (f *filesClient) NFSVersion(ctx context.Context, fileServer string) (nfsVersion, error) {
	if f.conn.pcScoped {
		return f.nfsVersionPCScoped(ctx, fileServer)
	}
	return f.nfsVersionFileServerDirect(ctx, f.newAPIClient())
}

// Builds a files-go-client ApiClient configured with this connection's endpoint and credentials.
// The SDK reads the password from a default Authorization header in addition to the
// Password field, so both are set (matching the Nutanix CSI driver's workaround).
func (f *filesClient) newAPIClient() *filesclient.ApiClient {
	apiClient := filesclient.NewApiClient()
	apiClient.Host = f.conn.host
	if f.conn.port != 0 {
		apiClient.Port = f.conn.port
	}
	apiClient.Username = f.conn.username
	apiClient.Password = f.conn.password
	apiClient.SetVerifySSL(!f.conn.insecure)
	apiClient.AddDefaultHeader("Authorization", "Basic "+basicAuth(f.conn.username, f.conn.password))
	return apiClient
}

// Issues an authenticated GET against a Prism Central-scoped Files API path and returns the
// raw response body. It reuses the vendored SDK's HTTP layer; the vendored SDK does not expose the
// Prism Central-scoped (plural) paths, so the URI is built and called manually.
func (f *filesClient) pcGet(
	ctx context.Context,
	apiClient *filesclient.ApiClient,
	uri string,
	query url.Values,
) ([]byte, error) {
	return callWithContext(ctx, func() ([]byte, error) {
		localURI := uri
		return apiClient.CallApi(
			&localURI,
			http.MethodGet,
			nil,
			query,
			map[string]string{},
			url.Values{},
			[]string{"application/json"},
			[]string{},
			[]string{"basicAuthScheme"},
		)
	})
}

// Resolves the NFS version by talking to a file server directly, using the SDK's (singular, unscoped) name-services endpoint.
// This is correct when connecting straight to a file server (the files-key Secret case).
func (f *filesClient) nfsVersionFileServerDirect(
	ctx context.Context,
	apiClient *filesclient.ApiClient,
) (nfsVersion, error) {
	nsAPI := filesapi.NewNameServicesApi(apiClient)
	resp, err := callWithContext(ctx, func() (*filesconfig.NameServicesApiResponse, error) {
		// The SDK decodes the response with code that can panic on unexpected payloads. This runs in
		// its own goroutine, where a panic would crash the process, so convert it to an error.
		return recoverToError(nsAPI.GetNameServicesByFileServer)
	})
	if err != nil {
		return nfsVersionUnknown, err
	}
	return nfsVersionFromNameServicesResponse(resp), nil
}

// Resolves the NFS version through Prism Central. Prism Central only serves the
// plural, file server-scoped collection endpoints, so this first resolves the file server's external ID (UUID)
// from its name, then queries that file server's name-services.
// Both calls target manually-built URIs because the vendored SDK does not expose the Prism Central-scoped paths (mirroring the Nutanix CSI driver).
func (f *filesClient) nfsVersionPCScoped(ctx context.Context, fileServer string) (nfsVersion, error) {
	apiClient := f.newAPIClient()

	extID, err := f.fileServerExtIDByName(ctx, apiClient, fileServer)
	if err != nil {
		return nfsVersionUnknown, err
	}

	body, err := f.pcGet(ctx, apiClient, fmt.Sprintf(filesPCNameServicesURIFormat, url.PathEscape(extID)), nil)
	if err != nil {
		return nfsVersionUnknown, err
	}
	resp, err := decodeNameServicesResponse(body)
	if err != nil {
		return nfsVersionUnknown, err
	}
	return nfsVersionFromNameServicesResponse(resp), nil
}

// Extracts the NFS version from a name-services API response,
// returning nfsVersionUnknown when the response carries no usable NFS version.
func nfsVersionFromNameServicesResponse(resp *filesconfig.NameServicesApiResponse) nfsVersion {
	if resp == nil || resp.GetData() == nil {
		return nfsVersionUnknown
	}
	spec, ok := resp.GetData().(filesconfig.NameServiceSpec)
	if !ok || spec.NfsVersion == nil {
		return nfsVersionUnknown
	}
	return mapNFSVersion(*spec.NfsVersion)
}

// Decodes a name-services response body into the SDK's response type. The
// SDK's oneOf decoder dereferences "$objectType" without a nil check and panics on payloads that
// lack it, so any panic is recovered and returned as an error. An empty body yields (nil, nil).
func decodeNameServicesResponse(body []byte) (resp *filesconfig.NameServicesApiResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			resp = nil
			err = fmt.Errorf("failed to decode name-services response: %v", r)
		}
	}()

	if len(body) == 0 {
		return nil, nil
	}
	decoded := new(filesconfig.NameServicesApiResponse)
	if err := json.Unmarshal(body, decoded); err != nil {
		return nil, fmt.Errorf("failed to decode name-services response: %w", err)
	}
	return decoded, nil
}

// recoverToError calls f and converts a panic into an error.
func recoverToError[T any](f func(args ...map[string]interface{}) (T, error)) (res T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T
			res = zero
			err = fmt.Errorf("unexpected panic calling the Nutanix Files API: %v", r)
		}
	}()
	return f()
}

// pcFileServer is a minimal view of a Prism Central file-servers list entry. The vendored SDK has
// no list response type for the Prism Central-scoped (plural) endpoint, so the response is decoded
// into this minimal shape, matching the fields the Nutanix CSI driver reads.
type pcFileServer struct {
	ExtID string `json:"extId"`
	Name  string `json:"name"`
	// DNSDomainName is the file server's fully qualified domain name namespace, e.g.
	// "fileserver_name.corp.example.com".
	DNSDomainName string `json:"dnsDomainName"`
	// ExternalNetworks carry the external IP addresses clients use to mount the file server; a static
	// SC's "nfsServer" is typically one of these (or the file server VIP).
	ExternalNetworks []pcFileServerNetwork `json:"externalNetworks"`
	// VirtualIPAddress is the file server's virtual IP, another value a static SC may use.
	VirtualIPAddress *pcIPAddress `json:"virtualIpAddress"`
}

// pcFileServerNetwork is a minimal view of a file server network, carrying its assigned IP addresses.
type pcFileServerNetwork struct {
	IPAddresses []pcIPAddress `json:"ipAddresses"`
}

// pcIPAddress is a minimal view of an IP address object (IPv4 or IPv6) in the Files API.
type pcIPAddress struct {
	IPv4 *pcIPValue `json:"ipv4"`
	IPv6 *pcIPValue `json:"ipv6"`
}

type pcIPValue struct {
	Value string `json:"value"`
}

// matches reports whether the given identifier refers to this file server. When byFQDN is false the
// match is on name or extId (dynamic SCs). When true it matches a static SC's "nfsServer" mount
// endpoint, which per the Nutanix CSI docs is the file server's external IP address or FQDN :
//   - an IP address (e.g. "1.2.3.4"), matched against the file server's external/virtual IPs; or
//   - an FQDN (e.g. "csi-files.afs-delhi.com"), matched against the file server's fully qualified
//     name. The Files API exposes the FQDN as two parts: the file server "name" (e.g. "csi-files")
//     and the "dnsDomainName" suffix (e.g. "afs-delhi.com"); the FQDN is "<name>.<dnsDomainName>".
func (fs pcFileServer) matches(identifier string, byFQDN bool) bool {
	if !byFQDN {
		// Dynamic SCs identify the file server by name (or, defensively, extId).
		return fs.Name == identifier || (fs.ExtID != "" && fs.ExtID == identifier)
	}

	// Normalize: drop any scheme and :port, lowercase, and trim a trailing dot (FQDNs may be absolute).
	id := strings.TrimSpace(strings.ToLower(identifier))
	id = strings.TrimPrefix(strings.TrimPrefix(id, "https://"), "http://")
	if host, _, found := strings.Cut(id, ":"); found {
		id = host
	}
	id = strings.TrimSuffix(id, ".")
	if id == "" {
		return false
	}

	// FQDN match: the full "<name>.<dnsDomainName>" (the bare name is not a valid nfsServer value).
	name := strings.ToLower(strings.TrimSpace(fs.Name))
	domain := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(fs.DNSDomainName, ".")))
	if name != "" && domain != "" && id == name+"."+domain {
		return true
	}

	// IP match: external network IPs and the file server virtual IP.
	if fs.VirtualIPAddress != nil && ipValueMatches(fs.VirtualIPAddress, id) {
		return true
	}
	for i := range fs.ExternalNetworks {
		for j := range fs.ExternalNetworks[i].IPAddresses {
			if ipValueMatches(&fs.ExternalNetworks[i].IPAddresses[j], id) {
				return true
			}
		}
	}
	return false
}

// ipValueMatches reports whether an IP address object equals the given lowercased identifier.
func ipValueMatches(addr *pcIPAddress, id string) bool {
	if addr == nil {
		return false
	}
	if addr.IPv4 != nil && strings.ToLower(strings.TrimSpace(addr.IPv4.Value)) == id {
		return true
	}
	if addr.IPv6 != nil && strings.ToLower(strings.TrimSpace(addr.IPv6.Value)) == id {
		return true
	}
	return false
}

// pcFileServersResponse is a minimal view of a page of the Prism Central file-servers list response.
type pcFileServersResponse struct {
	Data     []pcFileServer `json:"data"`
	Metadata struct {
		TotalAvailableResults *int `json:"totalAvailableResults"`
	} `json:"metadata"`
}

// fileServerExtIDByName resolves a file server's external ID (UUID) from its name (or accepts a UUID
// directly) by listing the Prism Central-scoped file-servers collection and matching on name or
// extId. The collection is paginated, so every page is consumed until a match is found; a server
// beyond the first page must not be reported as missing. It returns an error when no matching file
// server is found.
func (f *filesClient) fileServerExtIDByName(
	ctx context.Context,
	apiClient *filesclient.ApiClient,
	fileServer string,
) (string, error) {
	seen := 0
	for page := 0; page < filesPCListMaxPages; page++ {
		query := url.Values{}
		query.Set("$page", strconv.Itoa(page))
		query.Set("$limit", strconv.Itoa(filesPCListPageSize))

		body, err := f.pcGet(ctx, apiClient, filesPCFileServersURI, query)
		if err != nil {
			return "", err
		}
		if len(body) == 0 {
			return "", fmt.Errorf("empty response listing Nutanix Files servers")
		}

		var resp pcFileServersResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", fmt.Errorf("failed to decode Nutanix Files servers list: %w", err)
		}

		for i := range resp.Data {
			fs := resp.Data[i]
			if fs.matches(fileServer, f.conn.matchByFQDN) {
				if fs.ExtID == "" {
					return "", fmt.Errorf("file server %q has no external ID", fileServer)
				}
				return fs.ExtID, nil
			}
		}

		seen += len(resp.Data)
		total := resp.Metadata.TotalAvailableResults
		switch {
		case len(resp.Data) == 0:
			return "", fmt.Errorf("file server %q not found on Prism Central", fileServer)
		case total != nil && seen >= *total:
			return "", fmt.Errorf("file server %q not found on Prism Central", fileServer)
		case total == nil && len(resp.Data) < filesPCListPageSize:
			// No total reported: a short page means this was the last one.
			return "", fmt.Errorf("file server %q not found on Prism Central", fileServer)
		}
	}
	return "", fmt.Errorf(
		"file server %q not found within the first %d Prism Central file servers",
		fileServer,
		filesPCListMaxPages*filesPCListPageSize,
	)
}

// basicAuth returns the base64-encoded "user:password" value for an HTTP Basic Authorization header.
func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// mapNFSVersion maps a files-go-client NfsVersion enum to the local nfsVersion type.
func mapNFSVersion(v filesconfig.NfsVersion) nfsVersion {
	switch v {
	case filesconfig.NFSVERSION_NFSV4:
		return nfsVersionV4
	case filesconfig.NFSVERSION_NFSV3V4:
		return nfsVersionV3V4
	case filesconfig.NFSVERSION_NFSV3:
		return nfsVersionV3
	default:
		return nfsVersionUnknown
	}
}

// Resolves credentials for the candidate StorageClass (its CSI provisioner
// Secret first, then CAREN's Prism Central credentials) and returns a filesNFSVersionGetter.
func newFilesNFSVersionGetter(
	ctx context.Context,
	remoteClient ctrlclient.Client,
	sc *storagev1.StorageClass,
	fsRef fileServerRef,
	pcFallback *prismgoclient.Credentials,
) (filesNFSVersionGetter, error) {
	// Prefer the StorageClass's own files-key Secret: it carries "fqdn:user:password" where fqdn is the
	// file server's management endpoint, so the file-server-direct lookup works for both dynamic and
	// static SCs. This matches the Nutanix CSI driver, which uses Files secrets when present.
	conn, err := filesConnectionFromProvisionerSecret(ctx, remoteClient, sc)
	if err == nil && conn != nil {
		return &filesClient{conn: *conn}, nil
	}

	// No usable Secret: fall back to CAREN's Prism Central credentials. The identifier is matched
	// against the Prism Central file-servers collection: dynamic SCs by name, static SCs (which only
	// carry a mount endpoint FQDN/IP via "nfsServer") by the file server's name/FQDN/IP addresses.
	// The mount endpoint itself is NOT a Files config API endpoint, so it must never be queried
	// directly (doing so returns an Aplos 400).
	if pcFallback != nil {
		host, port := splitHostPort(pcFallback.Endpoint, defaultNutanixFilesAPIPort)
		return &filesClient{conn: filesConnection{
			host:     host,
			port:     port,
			username: pcFallback.Username,
			password: pcFallback.Password,
			// insecure is always true: the Nutanix CSI driver connects to Nutanix Files with
			// Insecure: true even for the Prism Central path (k8s-csi V4ClientParams), so the check
			// probes it the same way rather than verifying against CAREN's PC trust bundle.
			insecure:    true,
			pcScoped:    true,
			matchByFQDN: fsRef.isFQDN,
		}}, nil
	}

	return nil, fmt.Errorf("no usable credentials to query Nutanix Files: %w", err)
}

// Builds a filesConnection from the StorageClass's CSI
// provisioner Secret on the workload cluster. It returns (nil, error) when the Secret is absent or
// does not carry usable credentials, so the caller can fall back to Prism Central credentials.
func filesConnectionFromProvisionerSecret(
	ctx context.Context,
	remoteClient ctrlclient.Client,
	sc *storagev1.StorageClass,
) (*filesConnection, error) {
	name := sc.Parameters[csiParameterKeyProvisionerSecretName]
	namespace := sc.Parameters[csiParameterKeyProvisionerSecretNamespace]
	if name == "" || namespace == "" {
		return nil, fmt.Errorf("StorageClass %q does not reference a CSI provisioner secret", sc.Name)
	}

	getCtx, cancel := context.WithTimeout(ctx, filesNFSv4SubCallTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := remoteClient.Get(
		getCtx,
		types.NamespacedName{Name: name, Namespace: namespace},
		secret,
	); err != nil {
		return nil, fmt.Errorf("failed to get CSI provisioner secret %s/%s: %w", namespace, name, err)
	}

	// files-key SCs talk to the file server directly at "fqdn:username:password".
	if raw, ok := secret.Data[filesKeySecretDataKey]; ok {
		parts := strings.SplitN(strings.TrimSpace(string(raw)), ":", 3)
		if len(parts) != 3 || parts[0] == "" {
			return nil, fmt.Errorf("files-key in secret %s/%s is malformed", namespace, name)
		}
		host, port := splitHostPort(parts[0], defaultNutanixFilesAPIPort)
		return &filesConnection{
			host:     host,
			port:     port,
			username: parts[1],
			password: parts[2],
			insecure: true,
		}, nil
	}

	return nil, fmt.Errorf(
		"CSI provisioner secret %s/%s does not contain usable Nutanix Files credentials",
		namespace,
		name,
	)
}

// Splits a "host" or "host:port" string, returning the host and, when present and
// valid, its port; otherwise it returns defaultPort.
func splitHostPort(endpoint string, defaultPort int) (host string, port int) {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	host, portStr, found := strings.Cut(endpoint, ":")
	if !found {
		return endpoint, defaultPort
	}
	parsedPort, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || parsedPort <= 0 {
		return host, defaultPort
	}
	return host, parsedPort
}
