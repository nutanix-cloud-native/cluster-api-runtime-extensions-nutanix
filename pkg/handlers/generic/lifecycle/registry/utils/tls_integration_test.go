// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var (
	testCrt = []byte(`-----BEGIN CERTIFICATE-----
MIIC/jCCAeagAwIBAgIQNIZl/oI199Zgn4a2c6pAADANBgkqhkiG9w0BAQsFADAZ
MRcwFQYDVQQDEw5yZWdpc3RyeS1hZGRvbjAeFw0yNTA1MTUxOTU2MDhaFw0zNTA1
MTMxOTU2MDhaMBkxFzAVBgNVBAMTDnJlZ2lzdHJ5LWFkZG9uMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAo5eCNCJwyZf66WJw5+YjsHRKvtFzMx08Q2cn
doCaRj1/enTo2h48o2NMvG2MNYg0SAyAvSrSpy8fWZhXyU/SdUJv+DGCczwMtkH0
DHXONlFYBkbe16v4QwB5TCX0G+IWZgzfFwX+vT/KVXxjJMmkdSdkvJN6kpD/8knM
jmxTAxjUIKMygfaut21MHI5YlD2h3gfyEsyJrlhhxZK8sWEfdEMd9z259PnthgW/
ZWVf30L1xy40ErPiZwVZ/X8Y+99+xGXn5unQkTclHDvdXMX1xi7XkhN6kYuJ2CKZ
mWk160lydz0nSP8v7TIv7Mj78WbPnkQH19I9G938mB7d/L7IXQIDAQABo0IwQDAO
BgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUwp+UqNe3
CN+mT8CMYf+QvgBHo1swDQYJKoZIhvcNAQELBQADggEBAGcbZb34EEhm9kCfus/c
N53oX15qpiwIKpddE5Vi/MXspLfncRMFqWagoSgvP8zVO5FxyOcMAIPO1lVgdxwU
ATGGHj84AeKQ5wNYyIiZkac/cL2sBjWSogHivCHmMbLIngx2km3LO0iKPF1H6eGp
c9J7CRrehChgLD1Fy4v6CbIf5lzUwhelJtRgXZW0G/LPY3q9DAEwQ0IUFsZz/S3v
H+n9yKYUvKRDCSRoHpL12Jw4XibHfpoQWi3GiaWo+wtInD6/gZ3wOo+2lvz4e1qx
PZHZMQ492XZprH0DkOLIj+oiDKE/RNZAtPfaE2hFr0cs3EnN6NlGOesNtVTn0avf
3YA=
-----END CERTIFICATE-----`)

	testKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAo5eCNCJwyZf66WJw5+YjsHRKvtFzMx08Q2cndoCaRj1/enTo
2h48o2NMvG2MNYg0SAyAvSrSpy8fWZhXyU/SdUJv+DGCczwMtkH0DHXONlFYBkbe
16v4QwB5TCX0G+IWZgzfFwX+vT/KVXxjJMmkdSdkvJN6kpD/8knMjmxTAxjUIKMy
gfaut21MHI5YlD2h3gfyEsyJrlhhxZK8sWEfdEMd9z259PnthgW/ZWVf30L1xy40
ErPiZwVZ/X8Y+99+xGXn5unQkTclHDvdXMX1xi7XkhN6kYuJ2CKZmWk160lydz0n
SP8v7TIv7Mj78WbPnkQH19I9G938mB7d/L7IXQIDAQABAoIBAEU2xQ/pwm6IrtAv
pjV3WYI+saEqXOMza1vZOQkaQCuXuWfGLv6Z7G30hXLzpm6/wd756z4d8CJr/Yea
vQmfjBuwkE8iI189+OLj5K2g6i5xHB0Lvxzg1ZkDik59gFqLvY5Pw9Op5a2MX77r
cccOyVYH5MckXqfEUYXhU3quujCEk4ha5C+cVEJlnfeO4hTAoKXqhQ3eQ70hQURi
x9zxXOuHZbPr//YVtjeEYcLTeJe7YtV4+pFE8J2QS1L3CXzw1j4uULCUf0yPypXQ
LZL9Z0HIkMrm470G7semL39EbNJhsnFW/SWtgwfwQpH42JcQGWjISZZpNfKws8eo
f7wn9gECgYEA0I4MLibtJZrlvSJVi7ixn5fLSnl0h7DqwIivHhGZP2xHIzUzNh2/
rW6+7DKQqw6/K2BCkO7gNIBvKEe3XdlzP5+3PZeChFHOt+vWuEVo4v52IiYY7ing
aL8HhLVBUMROm2dpVJlVFtfRVb6SxJtINyQVLB0sM44fYn8ZSh+aEPkCgYEAyM7f
4BEsEQa5MHZWG48meZKBZCkxcBWbu8aHCIrd8TILepRhMPM9+j6ex+e9jnOPzmHv
xehMyvqFRZ9OCrgdxw4vyJpl0l5KObsBZvIJXmuxclMXOAUwHdx+lFLpAJDC9QVm
YDvAxD3Sy67rOWfECdhD9eM+9j14hchmj9UYb4UCgYAkXRAsn+brjqWOI8Vstkhq
PkpY8vJpkmRsK6j1AjaJQ3Tn46fJQMiiEdRCVNK6sLiOdJtGsA/xt48qI88KExcw
OcX2fEtqjOURVpK60IdoRNwOOjxQkoapXN2PuxbnYUMff5lzAcU/VWQPoknu8/BU
hPsYFQIW/ynjv6uGLBpt6QKBgGRVydMBgY04WMv4NOosSsMwCurrEkK46UmX1tzT
1jWwFcA356A3yd4B8ABesH4/C7nJga7XdZduOa0h/jKo8GgHlKSdUQceCeRypi6z
/S5qjQ1cqxtYrEQfajfefYHE00TuX8rx0E29vlf7nJjgWjm5D6wK0ejjqhbenTB8
/2qpAoGAZCXNflc2JVU1Lx235KdJg8jIdo8rutowBQ7GtDrJXzqPpIwv6M9hEsnr
I0EWIHsyNzyT3eqsXIO2ZIFKtqN83bPEFmVkKXAUV6lb0/nVSLrRTppFWlNyg1o5
FYnq6/jDVxCbWmmP2u4TT557gMqao0DaJstf/NSXlK0bhA2B64M=
-----END RSA PRIVATE KEY-----`)

	testRegistryAddonRootCASecretName = "registry-addon-root-ca"
)

var _ = Describe("Test EnsureRegistryAddonRootCASecret", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	It("new root CA Secret should be created", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-management-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		err = EnsureRegistryAddonRootCASecret(ctx, c, cluster)
		Expect(err).To(Succeed())

		// Verify the global TLS certificate secret is created.
		globalTLSCertificateSecret, err := handlersutils.SecretForRegistryAddonRootCA(ctx, c)
		Expect(err).To(Succeed())
		Expect(globalTLSCertificateSecret.OwnerReferences).To(
			ContainElement(
				metav1.OwnerReference{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       clusterv1.ClusterKind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			),
		)
		Expect(globalTLSCertificateSecret.Data["ca.crt"]).ToNot(BeEmpty())
		Expect(globalTLSCertificateSecret.Data[corev1.TLSCertKey]).ToNot(BeEmpty())
		Expect(globalTLSCertificateSecret.Data[corev1.TLSPrivateKeyKey]).ToNot(BeEmpty())
	})

	It("a root CA Secret should not be re-created", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-management-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		err = EnsureRegistryAddonRootCASecret(ctx, c, cluster)
		Expect(err).To(Succeed())

		globalTLSCertificateSecret, err := handlersutils.SecretForRegistryAddonRootCA(ctx, c)
		Expect(err).To(Succeed())
		Expect(globalTLSCertificateSecret.Data["ca.crt"]).ToNot(BeEmpty())
		Expect(globalTLSCertificateSecret.Data[corev1.TLSCertKey]).ToNot(BeEmpty())
		Expect(globalTLSCertificateSecret.Data[corev1.TLSPrivateKeyKey]).ToNot(BeEmpty())
		data := globalTLSCertificateSecret.Data

		// Verify the data is not changed when running the function again.
		err = EnsureRegistryAddonRootCASecret(ctx, c, cluster)
		Expect(err).To(Succeed())

		globalTLSCertificateSecret, err = handlersutils.SecretForRegistryAddonRootCA(ctx, c)
		Expect(err).To(Succeed())

		Expect(globalTLSCertificateSecret.Data).To(Equal(data))
	})
	AfterEach(func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		globalSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      handlersutils.RegistryAddonRootCASecretName,
				Namespace: corev1.NamespaceDefault,
			},
		}
		Expect(c.Delete(ctx, globalSecret)).To(
			Or(
				Succeed(),
				MatchError("secrets \"registry-addon-root-ca\" not found"),
			),
		)
	})
})

var _ = Describe("Test EnsureCASecretForCluster", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	It("CA Secret should be created for a cluster", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		globalSecret := testGlobalRegistryAddonTLSCertificate()
		Expect(c.Create(ctx, globalSecret)).To(Succeed())

		Expect(EnsureCASecretForCluster(ctx, c, cluster)).To(Succeed())
		caSecretKey := ctrlclient.ObjectKey{
			Name:      fmt.Sprintf("%s-registry-addon-ca", cluster.Name),
			Namespace: corev1.NamespaceDefault,
		}
		caSecret := &corev1.Secret{}
		Expect(c.Get(ctx, caSecretKey, caSecret)).To(Succeed())
		Expect(caSecret.OwnerReferences).To(
			ContainElement(
				metav1.OwnerReference{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       clusterv1.ClusterKind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			),
		)
		Expect(caSecret.Data["ca.crt"]).To(Equal(testCrt))
		Expect(caSecret.Data[corev1.TLSCertKey]).To(BeEmpty())
		Expect(caSecret.Data[corev1.TLSPrivateKeyKey]).To(BeEmpty())
	})
	It("CA Secret should not be created when missing global CA Secret", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		Expect(EnsureCASecretForCluster(ctx, c, cluster)).To(
			MatchError("error getting registry addon root CA secret: " +
				"secrets \"registry-addon-root-ca\" not found",
			),
		)
		caSecretKey := ctrlclient.ObjectKey{
			Name:      fmt.Sprintf("%s-registry-addon-ca", cluster.Name),
			Namespace: corev1.NamespaceDefault,
		}
		caSecret := &corev1.Secret{}
		Expect(c.Get(ctx, caSecretKey, caSecret)).ToNot(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		globalSecret := testGlobalRegistryAddonTLSCertificate()
		Expect(c.Delete(ctx, globalSecret)).To(
			Or(
				Succeed(),
				MatchError("secrets \"registry-addon-root-ca\" not found"),
			),
		)
	})
})

var _ = Describe("Test EnsureRegistryServerCertificateSecretOnRemoteCluster", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	It("TLS Secret should be created/updated on the remote cluster", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())
		Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

		globalSecret := testGlobalRegistryAddonTLSCertificate()
		Expect(c.Create(ctx, globalSecret)).To(Succeed())

		remoteTLSSecretName := "registry-tls"
		opts := &EnsureCertificateOpts{
			RemoteSecretKey: ctrlclient.ObjectKey{
				Name:      remoteTLSSecretName,
				Namespace: corev1.NamespaceDefault,
			},
			Spec: CertificateSpec{
				CommonName:  "registry",
				DNSNames:    []string{"registry"},
				IPAddresses: []string{"127.0.0.1"},
				Duration:    24 * 365 * time.Hour,
			},
		}
		remoteTLSSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      remoteTLSSecretName,
				Namespace: corev1.NamespaceDefault,
			},
		}

		remoteClient, err := remote.NewClusterClient(ctx, "", c, ctrlclient.ObjectKeyFromObject(cluster))
		Expect(err).To(BeNil())

		// Create the initial TLS secret on the remote cluster.
		Expect(EnsureRegistryServerCertificateSecretOnRemoteCluster(ctx, c, cluster, opts)).To(Succeed())
		err = remoteClient.Get(ctx, ctrlclient.ObjectKeyFromObject(remoteTLSSecret), remoteTLSSecret)
		Expect(err).To(BeNil())

		initialCA := remoteTLSSecret.Data["ca.crt"]
		initialCert := remoteTLSSecret.Data["tls.crt"]
		initialKey := remoteTLSSecret.Data["tls.key"]
		Expect(initialCA).To(Equal(testCrt))
		Expect(initialCert).ToNot(BeEmpty())
		Expect(initialKey).ToNot(BeEmpty())

		// Run the function again to update the TLS secret.
		Expect(EnsureRegistryServerCertificateSecretOnRemoteCluster(ctx, c, cluster, opts)).To(Succeed())
		err = remoteClient.Get(ctx, ctrlclient.ObjectKeyFromObject(remoteTLSSecret), remoteTLSSecret)
		Expect(err).To(BeNil())

		updatedCA := remoteTLSSecret.Data["ca.crt"]
		updatedCert := remoteTLSSecret.Data["tls.crt"]
		updatedKey := remoteTLSSecret.Data["tls.key"]
		Expect(updatedCA).To(Equal(testCrt))
		Expect(updatedCert).ToNot(BeEmpty())
		Expect(updatedKey).ToNot(BeEmpty())
		Expect(updatedCert).ToNot(Equal(initialCert))
		Expect(updatedKey).ToNot(Equal(initialKey))
	})

	It("Should error when missing global CA Secret", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		// Expect this to fail because the global CA secret is missing.
		Expect(EnsureRegistryServerCertificateSecretOnRemoteCluster(ctx, c, cluster, nil)).To(
			MatchError("failed to get TLS secret used to sign the certificate: " +
				"error getting registry addon root CA secret: " +
				"secrets \"registry-addon-root-ca\" not found"),
		)
	})

	AfterEach(func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		globalSecret := testGlobalRegistryAddonTLSCertificate()
		Expect(c.Delete(ctx, globalSecret)).To(
			Or(
				Succeed(),
				MatchError("secrets \"registry-addon-root-ca\" not found"),
			),
		)
	})
})

func testGlobalRegistryAddonTLSCertificate() *corev1.Secret {
	secretData := map[string][]byte{
		"ca.crt":  testCrt,
		"tls.crt": testCrt,
		"tls.key": testKey,
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRegistryAddonRootCASecretName,
			Namespace: corev1.NamespaceDefault,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}
}
