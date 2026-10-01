// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	filesconfig "github.com/nutanix/ntnx-api-golang-clients/files-go-client/v4/models/files/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

func TestStorageClassPinsNFSv4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mountOptions []string
		want         bool
	}{
		{name: "no mount options", mountOptions: nil, want: false},
		{name: "nfsvers=4", mountOptions: []string{"nfsvers=4"}, want: true},
		{name: "nfsvers=4.1", mountOptions: []string{"hard", "nfsvers=4.1"}, want: true},
		{name: "vers=4", mountOptions: []string{"vers=4"}, want: true},
		{name: "vers=4.2 with spaces", mountOptions: []string{" vers = 4.2 "}, want: true},
		{name: "uppercase NFSVERS", mountOptions: []string{"NFSVERS=4.0"}, want: true},
		{name: "nfsvers=3", mountOptions: []string{"nfsvers=3"}, want: false},
		{name: "vers=3.0", mountOptions: []string{"vers=3.0"}, want: false},
		{name: "unrelated option", mountOptions: []string{"hard", "rsize=1048576"}, want: false},
		{name: "vers without value", mountOptions: []string{"vers"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, storageClassPinsNFSv4(tt.mountOptions))
		})
	}
}

// mapNFSVersionGetter is a filesNFSVersionGetter that resolves the version and error for a file
// server from maps keyed by the file server name, and records that it was called.
type mapNFSVersionGetter struct {
	versions map[string]nfsVersion
	errs     map[string]error
	called   *int
}

func (m *mapNFSVersionGetter) NFSVersion(_ context.Context, fileServer string) (nfsVersion, error) {
	*m.called++
	if err, ok := m.errs[fileServer]; ok {
		return nfsVersionUnknown, err
	}
	return m.versions[fileServer], nil
}

// filesStorageClass returns a Nutanix Files StorageClass with the given file server, mount options,
// and provisioner name (defaulting to the Nutanix CSI provisioner).
func filesStorageClass(name, provisioner, fileServer string, mountOptions []string) *storagev1.StorageClass {
	if provisioner == "" {
		provisioner = nutanixCSIProvisioner
	}
	parameters := map[string]string{csiParameterKeyStorageType: csiStorageTypeNutanixFiles}
	if fileServer != "" {
		parameters[csiParameterKeyNFSServerName] = fileServer
	}
	return &storagev1.StorageClass{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Provisioner:  provisioner,
		Parameters:   parameters,
		MountOptions: mountOptions,
	}
}

// staticFilesStorageClass returns a static Nutanix Files StorageClass: it identifies the file server
// by mount endpoint FQDN ("nfsServer") plus "nfsPath", with no "nfsServerName".
func staticFilesStorageClass(name, nfsServer, nfsPath string, mountOptions []string) *storagev1.StorageClass {
	parameters := map[string]string{csiParameterKeyStorageType: csiStorageTypeNutanixFiles}
	if nfsServer != "" {
		parameters[csiParameterKeyNFSServer] = nfsServer
	}
	if nfsPath != "" {
		parameters["nfsPath"] = nfsPath
	}
	return &storagev1.StorageClass{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Provisioner:  nutanixCSIProvisioner,
		Parameters:   parameters,
		MountOptions: mountOptions,
	}
}

func TestFilesNFSv4Check_Run(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	newCluster := func(version string) *clusterv1.Cluster {
		return &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			Spec: clusterv1.ClusterSpec{
				Topology: clusterv1.Topology{Version: version},
			},
		}
	}

	tests := []struct {
		name             string
		newVersion       string
		oldVersion       string
		createOp         bool // oldCluster is nil
		storageClasses   []ctrlclient.Object
		clusterClientErr error
		listErr          bool
		factoryErr       error
		versions         map[string]nfsVersion
		lookupErrs       map[string]error
		wantAllowed      bool
		wantCauses       int
		wantWarnings     int
		wantGetterCalled int
	}{
		{
			// Create operations are skipped: there is no existing workload cluster (and no
			// pre-existing NFS mounts) to protect, so the check allows silently.
			name:        "create op is allowed silently",
			newVersion:  "v1.30.0",
			createOp:    true,
			wantAllowed: true,
		},
		{
			// The Kubernetes-version upgrade gate is disabled, so a same-version update is no
			// longer special: it proceeds and, absent any Nutanix Files StorageClass, allows.
			name:        "same-version update with no Files StorageClass is allowed",
			newVersion:  "v1.30.0",
			oldVersion:  "v1.30.0",
			wantAllowed: true,
		},
		{
			// A downgrade is also treated as a normal update now; absent a Files SC it allows.
			name:        "downgrade update with no Files StorageClass is allowed",
			newVersion:  "v1.29.5",
			oldVersion:  "v1.30.0",
			wantAllowed: true,
		},
		{
			name:       "no NutanixFiles StorageClass is allowed",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				&storagev1.StorageClass{
					ObjectMeta:  metav1.ObjectMeta{Name: "block-sc"},
					Provisioner: nutanixCSIProvisioner,
					Parameters:  map[string]string{csiParameterKeyStorageType: "NutanixVolumes"},
				},
				filesStorageClass("other-files", "other.csi.example.com", "fs-1", nil),
			},
			wantAllowed: true,
		},
		{
			name:       "SC with nfsvers=4.1 mountOption is allowed without a Files lookup",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", []string{"nfsvers=4.1"}),
			},
			wantAllowed:      true,
			wantGetterCalled: 0,
		},
		{
			name:       "SC with vers=4 mountOption is allowed without a Files lookup",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", []string{"vers=4"}),
			},
			wantAllowed:      true,
			wantGetterCalled: 0,
		},
		{
			name:       "file server with NFSV4 is allowed",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", nil),
			},
			versions:         map[string]nfsVersion{"fs-1": nfsVersionV4},
			wantAllowed:      true,
			wantGetterCalled: 1,
		},
		{
			name:       "file server with NFSV3V4 is allowed",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", nil),
			},
			versions:         map[string]nfsVersion{"fs-1": nfsVersionV3V4},
			wantAllowed:      true,
			wantGetterCalled: 1,
		},
		{
			name:       "file server with NFSV3 only fails with a cause",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "prod-fs", nil),
			},
			versions:         map[string]nfsVersion{"prod-fs": nfsVersionV3},
			wantAllowed:      false,
			wantCauses:       1,
			wantGetterCalled: 1,
		},
		{
			// An unknown/redacted/missing version is not a confirmed NFSv3-only server, so it must
			// never block the upgrade.
			name:       "file server with unknown NFS version warns and allows",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", nil),
			},
			versions:         map[string]nfsVersion{"fs-1": nfsVersionUnknown},
			wantAllowed:      true,
			wantWarnings:     1,
			wantGetterCalled: 1,
		},
		{
			name:       "multiple SCs with one NFSV3 only fails with a single cause",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("good-sc", "", "fs-v4", nil),
				filesStorageClass("bad-sc", "", "fs-v3", nil),
				filesStorageClass("pinned-sc", "", "fs-v3", []string{"nfsvers=4.1"}),
			},
			versions: map[string]nfsVersion{
				"fs-v4": nfsVersionV4,
				"fs-v3": nfsVersionV3,
			},
			wantAllowed:      false,
			wantCauses:       1,
			wantGetterCalled: 2, // pinned-sc short-circuits before the lookup
		},
		{
			name:             "missing kubeconfig secret warns and allows",
			newVersion:       "v1.30.0",
			oldVersion:       "v1.29.5",
			clusterClientErr: errors.New("kubeconfig secret not found"),
			wantAllowed:      true,
			wantWarnings:     1,
		},
		{
			name:         "remote list error warns and allows",
			newVersion:   "v1.30.0",
			oldVersion:   "v1.29.5",
			listErr:      true,
			wantAllowed:  true,
			wantWarnings: 1,
		},
		{
			name:       "SC with neither nfsServerName nor nfsServer warns and allows",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "", nil),
			},
			wantAllowed:      true,
			wantWarnings:     1,
			wantGetterCalled: 0,
		},
		{
			// A static SC identifies the file server by mount endpoint FQDN ("nfsServer"); an
			// NFSv3-only server must still block.
			name:       "static SC with NFSV3 file server fails with a cause",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				staticFilesStorageClass("static-sc", "fs.example.com", "/share", nil),
			},
			versions:         map[string]nfsVersion{"fs.example.com": nfsVersionV3},
			wantAllowed:      false,
			wantCauses:       1,
			wantGetterCalled: 1,
		},
		{
			name:       "static SC with NFSV4 file server is allowed",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				staticFilesStorageClass("static-sc", "fs.example.com", "/share", nil),
			},
			versions:         map[string]nfsVersion{"fs.example.com": nfsVersionV4},
			wantAllowed:      true,
			wantGetterCalled: 1,
		},
		{
			name:       "static SC pinning nfsvers=4.1 is allowed without a lookup",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				staticFilesStorageClass("static-sc", "fs.example.com", "/share", []string{"nfsvers=4.1"}),
			},
			wantAllowed:      true,
			wantGetterCalled: 0,
		},
		{
			name:       "Files lookup error warns and allows",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", nil),
			},
			lookupErrs:       map[string]error{"fs-1": errors.New("unreachable")},
			wantAllowed:      true,
			wantWarnings:     1,
			wantGetterCalled: 1,
		},
		{
			name:       "unresolvable credentials warns and allows",
			newVersion: "v1.30.0",
			oldVersion: "v1.29.5",
			storageClasses: []ctrlclient.Object{
				filesStorageClass("nfs-sc", "", "fs-1", nil),
			},
			factoryErr:   errors.New("no usable credentials"),
			wantAllowed:  true,
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := fake.NewClientBuilder().WithScheme(scheme)
			if len(tt.storageClasses) > 0 {
				builder = builder.WithObjects(tt.storageClasses...)
			}
			if tt.listErr {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(
						context.Context,
						ctrlclient.WithWatch,
						ctrlclient.ObjectList,
						...ctrlclient.ListOption,
					) error {
						return errors.New("workload cluster unreachable")
					},
				})
			}
			remoteClient := builder.Build()

			getterCalled := 0

			var oldCluster *clusterv1.Cluster
			if !tt.createOp {
				oldCluster = newCluster(tt.oldVersion)
			}

			check := &filesNFSv4Check{
				cluster:       newCluster(tt.newVersion),
				oldCluster:    oldCluster,
				kclient:       remoteClient,
				log:           logr.Discard(),
				pcCredentials: &prismgoclient.Credentials{Endpoint: "pc.example.com:9440"},
				clusterClientGetter: func(
					_ context.Context,
					_ string,
					_ ctrlclient.Client,
					_ ctrlclient.ObjectKey,
				) (ctrlclient.Client, error) {
					if tt.clusterClientErr != nil {
						return nil, tt.clusterClientErr
					}
					return remoteClient, nil
				},
				nfsVersionGetterFactory: func(
					_ context.Context,
					_ ctrlclient.Client,
					_ *storagev1.StorageClass,
					_ fileServerRef,
					_ *prismgoclient.Credentials,
				) (filesNFSVersionGetter, error) {
					if tt.factoryErr != nil {
						return nil, tt.factoryErr
					}
					return &mapNFSVersionGetter{
						versions: tt.versions,
						errs:     tt.lookupErrs,
						called:   &getterCalled,
					}, nil
				},
			}

			result := check.Run(context.Background())

			assert.Equal(t, tt.wantAllowed, result.Allowed, "Allowed")
			assert.False(t, result.InternalError, "InternalError must never be set")
			assert.Len(t, result.Causes, tt.wantCauses, "Causes")
			assert.Len(t, result.Warnings, tt.wantWarnings, "Warnings")
			assert.Equal(t, tt.wantGetterCalled, getterCalled, "filesNFSVersionGetter call count")
		})
	}
}

func TestFilesNFSv4Check_Name(t *testing.T) {
	t.Parallel()
	check := &filesNFSv4Check{}
	assert.Equal(t, "NutanixFilesNFSv4", check.Name())
}

func TestNewFilesNFSv4Checks(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	kclient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "default"}}

	tests := []struct {
		name      string
		cd        *checkDependencies
		wantCount int
	}{
		{name: "nil dependencies", cd: nil, wantCount: 0},
		{
			name: "missing Prism Central version is gated out",
			cd:   &checkDependencies{cluster: cluster, kclient: kclient, log: logr.Discard()},
		},
		{
			name: "missing cluster is gated out",
			cd:   &checkDependencies{kclient: kclient, pcVersion: "7.5.0", log: logr.Discard()},
		},
		{
			name:      "all preconditions met registers one check",
			cd:        &checkDependencies{cluster: cluster, kclient: kclient, pcVersion: "7.5.0", log: logr.Discard()},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Len(t, newFilesNFSv4Checks(tt.cd), tt.wantCount)
		})
	}
}

func TestMapNFSVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   filesconfig.NfsVersion
		want nfsVersion
	}{
		{in: filesconfig.NFSVERSION_NFSV4, want: nfsVersionV4},
		{in: filesconfig.NFSVERSION_NFSV3V4, want: nfsVersionV3V4},
		{in: filesconfig.NFSVERSION_NFSV3, want: nfsVersionV3},
		{in: filesconfig.NFSVERSION_UNKNOWN, want: nfsVersionUnknown},
		{in: filesconfig.NFSVERSION_REDACTED, want: nfsVersionUnknown},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, mapNFSVersion(tt.in))
	}
}

func TestSplitHostPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint string
		wantHost string
		wantPort int
	}{
		{name: "host only", endpoint: "fs.example.com", wantHost: "fs.example.com", wantPort: defaultNutanixFilesAPIPort},
		{name: "host and port", endpoint: "fs.example.com:9441", wantHost: "fs.example.com", wantPort: 9441},
		{
			name:     "https scheme is stripped",
			endpoint: "https://pc.example.com:9440",
			wantHost: "pc.example.com",
			wantPort: 9440,
		},
		{
			name:     "invalid port falls back to default",
			endpoint: "fs.example.com:abc",
			wantHost: "fs.example.com",
			wantPort: defaultNutanixFilesAPIPort,
		},
		{name: "empty is left as-is with default port", endpoint: "", wantHost: "", wantPort: defaultNutanixFilesAPIPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, port := splitHostPort(tt.endpoint, defaultNutanixFilesAPIPort)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}

func TestFilesConnectionFromProvisionerSecret(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	scWithSecret := func() *storagev1.StorageClass {
		sc := filesStorageClass("nfs-sc", "", "fs-1", nil)
		sc.Parameters[csiParameterKeyProvisionerSecretName] = "csi-secret"
		sc.Parameters[csiParameterKeyProvisionerSecretNamespace] = "kube-system"
		return sc
	}

	filesKeySecret := func(value string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "csi-secret", Namespace: "kube-system"},
			Data:       map[string][]byte{filesKeySecretDataKey: []byte(value)},
		}
	}

	t.Run("files-key resolves a direct connection", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(filesKeySecret("fs.example.com:admin:s3cr3t")).Build()

		conn, err := filesConnectionFromProvisionerSecret(context.Background(), remoteClient, scWithSecret())
		require.NoError(t, err)
		require.NotNil(t, conn)
		assert.Equal(t, "fs.example.com", conn.host)
		assert.Equal(t, defaultNutanixFilesAPIPort, conn.port)
		assert.Equal(t, "admin", conn.username)
		assert.Equal(t, "s3cr3t", conn.password)
		assert.True(t, conn.insecure)
	})

	t.Run("no provisioner secret reference errors", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		conn, err := filesConnectionFromProvisionerSecret(
			context.Background(),
			remoteClient,
			filesStorageClass("nfs-sc", "", "fs-1", nil),
		)
		require.Error(t, err)
		assert.Nil(t, conn)
	})

	t.Run("secret not found errors", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		conn, err := filesConnectionFromProvisionerSecret(context.Background(), remoteClient, scWithSecret())
		require.Error(t, err)
		assert.Nil(t, conn)
	})

	t.Run("malformed files-key errors", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(filesKeySecret("only-one-field")).Build()

		conn, err := filesConnectionFromProvisionerSecret(context.Background(), remoteClient, scWithSecret())
		require.Error(t, err)
		assert.Nil(t, conn)
	})

	t.Run("secret without usable credentials errors", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "csi-secret", Namespace: "kube-system"},
			Data:       map[string][]byte{"unrelated": []byte("value")},
		}).Build()

		conn, err := filesConnectionFromProvisionerSecret(context.Background(), remoteClient, scWithSecret())
		require.Error(t, err)
		assert.Nil(t, conn)
	})
}

func TestNewFilesNFSVersionGetter(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	scWithSecret := func() *storagev1.StorageClass {
		sc := filesStorageClass("nfs-sc", "", "fs-1", nil)
		sc.Parameters[csiParameterKeyProvisionerSecretName] = "csi-secret"
		sc.Parameters[csiParameterKeyProvisionerSecretNamespace] = "kube-system"
		return sc
	}

	t.Run("SC provisioner secret is preferred", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "csi-secret", Namespace: "kube-system"},
			Data:       map[string][]byte{filesKeySecretDataKey: []byte("fs.example.com:admin:s3cr3t")},
		}).Build()

		getter, err := newFilesNFSVersionGetter(
			context.Background(),
			remoteClient,
			scWithSecret(),
			fileServerRef{identifier: "fs-1"},
			&prismgoclient.Credentials{Endpoint: "pc.example.com:9440", Username: "pc", Password: "pc-pass"},
		)
		require.NoError(t, err)
		fc, ok := getter.(*filesClient)
		require.True(t, ok)
		assert.Equal(t, "fs.example.com", fc.conn.host)
		assert.Equal(t, "admin", fc.conn.username)
		assert.False(t, fc.conn.pcScoped)
	})

	t.Run("dynamic SC falls back to Prism Central credentials (pc-scoped)", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		getter, err := newFilesNFSVersionGetter(
			context.Background(),
			remoteClient,
			filesStorageClass("nfs-sc", "", "fs-1", nil),
			fileServerRef{identifier: "fs-1", isFQDN: false},
			&prismgoclient.Credentials{Endpoint: "pc.example.com:9440", Username: "pc", Password: "pc-pass", Insecure: false},
		)
		require.NoError(t, err)
		fc, ok := getter.(*filesClient)
		require.True(t, ok)
		assert.Equal(t, "pc.example.com", fc.conn.host)
		assert.Equal(t, 9440, fc.conn.port)
		assert.Equal(t, "pc", fc.conn.username)
		assert.True(t, fc.conn.pcScoped)
		// The Prism Central fallback always uses insecure TLS, matching the Nutanix CSI driver,
		// regardless of the credentials' Insecure flag.
		assert.True(t, fc.conn.insecure)
	})

	t.Run("static SC without a secret resolves via Prism Central by FQDN/IP", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		getter, err := newFilesNFSVersionGetter(
			context.Background(),
			remoteClient,
			staticFilesStorageClass("static-sc", "10.15.98.173", "/share", nil),
			fileServerRef{identifier: "10.15.98.173", isFQDN: true},
			&prismgoclient.Credentials{Endpoint: "pc.example.com:9440", Username: "pc", Password: "pc-pass"},
		)
		require.NoError(t, err)
		fc, ok := getter.(*filesClient)
		require.True(t, ok)
		// The mount endpoint is not a Files API endpoint, so we query Prism Central and match the
		// mount endpoint against the file server list by FQDN/IP.
		assert.Equal(t, "pc.example.com", fc.conn.host)
		assert.Equal(t, 9440, fc.conn.port)
		assert.Equal(t, "pc", fc.conn.username)
		assert.True(t, fc.conn.pcScoped)
		assert.True(t, fc.conn.matchByFQDN)
		assert.True(t, fc.conn.insecure)
	})

	t.Run("static SC with a files-key secret uses the secret's FQDN directly", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "csi-secret", Namespace: "kube-system"},
			Data:       map[string][]byte{filesKeySecretDataKey: []byte("fs-mgmt.example.com:admin:s3cr3t")},
		}).Build()
		sc := staticFilesStorageClass("static-sc", "10.15.98.173", "/share", nil)
		sc.Parameters[csiParameterKeyProvisionerSecretName] = "csi-secret"
		sc.Parameters[csiParameterKeyProvisionerSecretNamespace] = "kube-system"

		getter, err := newFilesNFSVersionGetter(
			context.Background(),
			remoteClient,
			sc,
			fileServerRef{identifier: "10.15.98.173", isFQDN: true},
			&prismgoclient.Credentials{Endpoint: "pc.example.com:9440", Username: "pc", Password: "pc-pass"},
		)
		require.NoError(t, err)
		fc, ok := getter.(*filesClient)
		require.True(t, ok)
		// The files-key secret points at the file server's own management FQDN: use it directly.
		assert.Equal(t, "fs-mgmt.example.com", fc.conn.host)
		assert.Equal(t, "admin", fc.conn.username)
		assert.False(t, fc.conn.pcScoped)
	})

	t.Run("no usable credentials errors", func(t *testing.T) {
		t.Parallel()
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		getter, err := newFilesNFSVersionGetter(
			context.Background(),
			remoteClient,
			filesStorageClass("nfs-sc", "", "fs-1", nil),
			fileServerRef{identifier: "fs-1"},
			nil,
		)
		require.Error(t, err)
		assert.Nil(t, getter)
	})
}

func TestFileServerRefFromStorageClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sc      *storagev1.StorageClass
		wantRef fileServerRef
		wantOK  bool
	}{
		{
			name:    "dynamic SC uses nfsServerName as a name",
			sc:      filesStorageClass("dyn", "", "fs-1", nil),
			wantRef: fileServerRef{identifier: "fs-1", isFQDN: false},
			wantOK:  true,
		},
		{
			name:    "static SC uses nfsServer as an FQDN",
			sc:      staticFilesStorageClass("stat", "fs.example.com", "/share", nil),
			wantRef: fileServerRef{identifier: "fs.example.com", isFQDN: true},
			wantOK:  true,
		},
		{
			name: "nfsServerName is preferred over nfsServer when both are present",
			sc: func() *storagev1.StorageClass {
				sc := filesStorageClass("both", "", "fs-name", nil)
				sc.Parameters[csiParameterKeyNFSServer] = "fs.example.com"
				return sc
			}(),
			wantRef: fileServerRef{identifier: "fs-name", isFQDN: false},
			wantOK:  true,
		},
		{
			name:   "neither parameter present returns ok=false",
			sc:     filesStorageClass("none", "", "", nil),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ref, ok := fileServerRefFromStorageClass(tt.sc)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantRef, ref)
			}
		})
	}
}

func TestBasicAuth(t *testing.T) {
	t.Parallel()
	got := basicAuth("admin", "s3cr3t")
	want := base64.StdEncoding.EncodeToString([]byte("admin:s3cr3t"))
	assert.Equal(t, want, got)
}

func TestNFSVersionFromNameServicesResponse(t *testing.T) {
	t.Parallel()

	v4 := filesconfig.NFSVERSION_NFSV4
	specV4 := filesconfig.NewNameServiceSpec()
	specV4.NfsVersion = &v4
	respV4 := filesconfig.NewNameServicesApiResponse()
	require.NoError(t, respV4.SetData(*specV4))

	respNoVersion := filesconfig.NewNameServicesApiResponse()
	require.NoError(t, respNoVersion.SetData(*filesconfig.NewNameServiceSpec()))

	tests := []struct {
		name string
		resp *filesconfig.NameServicesApiResponse
		want nfsVersion
	}{
		{name: "nil response", resp: nil, want: nfsVersionUnknown},
		{name: "empty response", resp: filesconfig.NewNameServicesApiResponse(), want: nfsVersionUnknown},
		{name: "spec without nfsVersion", resp: respNoVersion, want: nfsVersionUnknown},
		{name: "nfsv4", resp: respV4, want: nfsVersionV4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, nfsVersionFromNameServicesResponse(tt.resp))
		})
	}
}

// nameServicesJSON marshals a name-services API response carrying the given NFS version, as the
// Prism Central / file server API would return it.
func nameServicesJSON(t *testing.T, v filesconfig.NfsVersion) []byte {
	t.Helper()
	spec := filesconfig.NewNameServiceSpec()
	spec.NfsVersion = &v
	resp := filesconfig.NewNameServicesApiResponse()
	require.NoError(t, resp.SetData(*spec))
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

// clientForServer returns a filesClient pointed at the given test server, with the pcScoped flag set
// as requested.
func clientForServer(t *testing.T, srv *httptest.Server, pcScoped bool) *filesClient {
	t.Helper()
	hostPort := strings.TrimPrefix(srv.URL, "https://")
	host, portStr, ok := strings.Cut(hostPort, ":")
	require.True(t, ok)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return &filesClient{conn: filesConnection{
		host:     host,
		port:     port,
		username: "admin",
		password: "s3cr3t",
		insecure: true,
		pcScoped: pcScoped,
	}}
}

func TestFilesClient_NFSVersion_PCScoped(t *testing.T) {
	t.Parallel()

	const (
		fsName  = "csi-files"
		fsExtID = "d8b59823-3416-4149-62c2-c8e86e1990d1"
	)

	t.Run("resolves version via plural PC paths", func(t *testing.T) {
		t.Parallel()

		var listPath, nsPath string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == filesPCFileServersURI:
				listPath = r.URL.Path
				_, _ = w.Write([]byte(
					`{"data":[{"name":"other","extId":"11111111-1111-1111-1111-111111111111"},` +
						`{"name":"` + fsName + `","extId":"` + fsExtID + `"}]}`,
				))
			case strings.HasSuffix(r.URL.Path, "/name-services"):
				nsPath = r.URL.Path
				_, _ = w.Write(nameServicesJSON(t, filesconfig.NFSVERSION_NFSV3V4))
			default:
				http.Error(w, "unexpected path", http.StatusNotFound)
			}
		}))
		defer srv.Close()

		got, err := clientForServer(t, srv, true).NFSVersion(context.Background(), fsName)
		require.NoError(t, err)
		assert.Equal(t, nfsVersionV3V4, got)
		assert.Equal(t, filesPCFileServersURI, listPath)
		assert.Equal(t, "/api/files/v4.0.a2/config/file-servers/"+fsExtID+"/name-services", nsPath)
	})

	t.Run("file server not found errors", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"name":"someone-else","extId":"x"}]}`))
		}))
		defer srv.Close()

		_, err := clientForServer(t, srv, true).NFSVersion(context.Background(), fsName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("sends basic auth credentials", func(t *testing.T) {
		t.Parallel()

		var gotAuth string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				gotAuth = r.Header.Get("Authorization")
			}
			switch {
			case r.URL.Path == filesPCFileServersURI:
				_, _ = w.Write([]byte(`{"data":[{"name":"` + fsName + `","extId":"` + fsExtID + `"}]}`))
			default:
				_, _ = w.Write(nameServicesJSON(t, filesconfig.NFSVERSION_NFSV4))
			}
		}))
		defer srv.Close()

		_, err := clientForServer(t, srv, true).NFSVersion(context.Background(), fsName)
		require.NoError(t, err)
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:s3cr3t"))
		assert.Equal(t, want, gotAuth)
	})
}

func TestFilesClient_NFSVersion_FileServerDirect(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write(nameServicesJSON(t, filesconfig.NFSVERSION_NFSV4))
	}))
	defer srv.Close()

	got, err := clientForServer(t, srv, false).NFSVersion(context.Background(), "ignored")
	require.NoError(t, err)
	assert.Equal(t, nfsVersionV4, got)
	// File server-direct uses the SDK's singular, unscoped path.
	assert.Equal(t, "/api/files/v4.0.a2/config/file-server/name-services", gotPath)
}

func TestFilesClient_NFSVersion_PCScoped_Pagination(t *testing.T) {
	t.Parallel()

	const (
		fsName  = "csi-files"
		fsExtID = "d8b59823-3416-4149-62c2-c8e86e1990d1"
	)

	// 120 file servers; the target is the last one, beyond any single default page.
	const total = 120
	all := make([]pcFileServer, 0, total)
	for i := 0; i < total-1; i++ {
		all = append(all, pcFileServer{Name: "fs-" + strconv.Itoa(i), ExtID: "id-" + strconv.Itoa(i)})
	}
	all = append(all, pcFileServer{Name: fsName, ExtID: fsExtID})

	t.Run("finds a file server beyond the first page", func(t *testing.T) {
		t.Parallel()

		var nsPath string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != filesPCFileServersURI {
				nsPath = r.URL.Path
				_, _ = w.Write(nameServicesJSON(t, filesconfig.NFSVERSION_NFSV3))
				return
			}
			// Honor $page/$limit like the Prism Central v4 API, capping the page size at 50.
			page, _ := strconv.Atoi(r.URL.Query().Get("$page"))
			limit, err := strconv.Atoi(r.URL.Query().Get("$limit"))
			if err != nil || limit <= 0 || limit > 50 {
				limit = 50
			}
			start := min(page*limit, total)
			end := min(start+limit, total)
			body, err := json.Marshal(map[string]any{
				"data":     all[start:end],
				"metadata": map[string]any{"totalAvailableResults": total},
			})
			require.NoError(t, err)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		got, err := clientForServer(t, srv, true).NFSVersion(context.Background(), fsName)
		require.NoError(t, err)
		assert.Equal(t, nfsVersionV3, got)
		assert.Equal(t, "/api/files/v4.0.a2/config/file-servers/"+fsExtID+"/name-services", nsPath)
	})

	t.Run("not found after paginating everything errors", func(t *testing.T) {
		t.Parallel()

		pages := 0
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pages++
			page, _ := strconv.Atoi(r.URL.Query().Get("$page"))
			data := []pcFileServer{}
			if page == 0 {
				data = all[:10]
			}
			body, err := json.Marshal(map[string]any{
				"data":     data,
				"metadata": map[string]any{"totalAvailableResults": 10},
			})
			require.NoError(t, err)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		_, err := clientForServer(t, srv, true).NFSVersion(context.Background(), "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Equal(t, 1, pages, "must stop paging once all results are consumed")
	})
}

func TestPCFileServerMatches(t *testing.T) {
	t.Parallel()

	// The file server's FQDN is "<name>.<dnsDomainName>" = "csi-files.afs-delhi.com".
	fs := pcFileServer{
		ExtID:         "ext-1",
		Name:          "csi-files",
		DNSDomainName: "afs-delhi.com",
		VirtualIPAddress: &pcIPAddress{
			IPv4: &pcIPValue{Value: "10.0.0.9"},
		},
		ExternalNetworks: []pcFileServerNetwork{
			{IPAddresses: []pcIPAddress{
				{IPv4: &pcIPValue{Value: "10.15.98.173"}},
				{IPv6: &pcIPValue{Value: "fe80::1"}},
			}},
		},
	}

	tests := []struct {
		name       string
		identifier string
		byFQDN     bool
		want       bool
	}{
		{name: "name match (dynamic)", identifier: "csi-files", byFQDN: false, want: true},
		{name: "extId match (dynamic)", identifier: "ext-1", byFQDN: false, want: true},
		{name: "FQDN not matched when byFQDN is false", identifier: "csi-files.afs-delhi.com", byFQDN: false, want: false},
		{name: "IP not matched when byFQDN is false", identifier: "10.15.98.173", byFQDN: false, want: false},
		{name: "full FQDN match (static)", identifier: "csi-files.afs-delhi.com", byFQDN: true, want: true},
		{name: "full FQDN match is case-insensitive", identifier: "CSI-Files.AFS-Delhi.com", byFQDN: true, want: true},
		{name: "absolute FQDN with trailing dot matches", identifier: "csi-files.afs-delhi.com.", byFQDN: true, want: true},
		{name: "bare name is not a valid nfsServer (static)", identifier: "csi-files", byFQDN: true, want: false},
		{name: "domain suffix alone does not match", identifier: "afs-delhi.com", byFQDN: true, want: false},
		{name: "wrong domain does not match", identifier: "csi-files.other.com", byFQDN: true, want: false},
		{name: "external IP match (static)", identifier: "10.15.98.173", byFQDN: true, want: true},
		{name: "virtual IP match (static)", identifier: "10.0.0.9", byFQDN: true, want: true},
		{name: "host:port identifier matches on host (static)", identifier: "10.15.98.173:9440", byFQDN: true, want: true},
		{name: "FQDN with port matches (static)", identifier: "csi-files.afs-delhi.com:9440", byFQDN: true, want: true},
		{name: "unrelated identifier does not match", identifier: "other", byFQDN: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, fs.matches(tt.identifier, tt.byFQDN))
		})
	}
}

func TestFilesClient_NFSVersion_PCScoped_StaticByFQDN(t *testing.T) {
	t.Parallel()

	const fsExtID = "d8b59823-3416-4149-62c2-c8e86e1990d1"

	var nsPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == filesPCFileServersURI {
			_, _ = w.Write([]byte(`{"data":[` +
				`{"name":"other","extId":"11111111-1111-1111-1111-111111111111"},` +
				`{"name":"prod-fs","extId":"` + fsExtID + `",` +
				`"externalNetworks":[{"ipAddresses":[{"ipv4":{"value":"10.15.98.173"}}]}]}` +
				`]}`))
			return
		}
		nsPath = r.URL.Path
		_, _ = w.Write(nameServicesJSON(t, filesconfig.NFSVERSION_NFSV3))
	}))
	defer srv.Close()

	c := clientForServer(t, srv, true)
	c.conn.matchByFQDN = true

	// The static SC's mount IP (with a port) is matched against the file server's external IP.
	got, err := c.NFSVersion(context.Background(), "10.15.98.173:9440")
	require.NoError(t, err)
	assert.Equal(t, nfsVersionV3, got)
	assert.Equal(t, "/api/files/v4.0.a2/config/file-servers/"+fsExtID+"/name-services", nsPath)
}

func TestFilesClient_NFSVersion_MalformedResponseDoesNotPanic(t *testing.T) {
	t.Parallel()

	// A JSON object without "$objectType" makes the vendored SDK's oneOf decoder dereference a nil
	// pointer; the client must turn that into an error instead of panicking.
	const noObjectType = `{"data":{"nfsVersion":"NFSV3"}}`

	t.Run("PC scoped", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == filesPCFileServersURI {
				_, _ = w.Write([]byte(`{"data":[{"name":"fs","extId":"id-1"}]}`))
				return
			}
			_, _ = w.Write([]byte(noObjectType))
		}))
		defer srv.Close()

		assert.NotPanics(t, func() {
			_, err := clientForServer(t, srv, true).NFSVersion(context.Background(), "fs")
			require.Error(t, err)
		})
	})

	t.Run("file server direct", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(noObjectType))
		}))
		defer srv.Close()

		assert.NotPanics(t, func() {
			got, err := clientForServer(t, srv, false).NFSVersion(context.Background(), "ignored")
			// Either an error or an unknown version is acceptable; it must never claim a version.
			if err == nil {
				assert.Equal(t, nfsVersionUnknown, got)
			}
		})
	})
}

func TestFilesClient_NFSVersion_InsecureTLS(t *testing.T) {
	t.Parallel()

	const (
		fsName  = "csi-files"
		fsExtID = "d8b59823-3416-4149-62c2-c8e86e1990d1"
	)

	// The test server presents a self-signed certificate; the client must still succeed, because it
	// connects with insecure TLS (matching the Nutanix CSI driver).
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == filesPCFileServersURI {
			_, _ = w.Write([]byte(`{"data":[{"name":"` + fsName + `","extId":"` + fsExtID + `"}]}`))
			return
		}
		_, _ = w.Write(nameServicesJSON(t, filesconfig.NFSVERSION_NFSV4))
	}))
	defer srv.Close()

	got, err := clientForServer(t, srv, true).NFSVersion(context.Background(), fsName)
	require.NoError(t, err)
	assert.Equal(t, nfsVersionV4, got)
}
