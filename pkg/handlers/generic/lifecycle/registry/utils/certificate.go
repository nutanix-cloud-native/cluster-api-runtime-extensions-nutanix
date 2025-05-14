package utils

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

var (
	//go:embed embedded/certificate-issuer.yaml
	certificateIssuerObjs []byte

	//go:embed embedded/certificate.yaml.gotmpl
	certificateObj         []byte
	certificateObjTemplate = template.Must(template.New("").Parse(string(certificateObj)))

	defaultDuration    = 2 * 365 * 24 * time.Hour // 2 years
	forceRenewalBefore = 365 * 24 * time.Hour     // 1 year
)

type EnsureCertificateOpts struct {
	// RemoteSecretName is the name of the TLS secret to be created on the remote cluster.
	RemoteSecretName string
	// RemoteSecretNamespace is the namespace of the TLS secret to be created on the remote cluster.
	RemoteSecretNamespace string

	Spec CertificateSpec
}

type CertificateSpec struct {
	// CommonName is the common name to be included in the certificate.
	CommonName string
	// DNSNames is a list of DNS names to be included in the certificate.
	DNSNames []string
	// IPAddresses is a list of IP addresses to be included in the certificate.
	IPAddresses []string
}

// EnsureCertificate ensures that the registry TLS certificate exists on the management cluster.
//
// This function uses cert-manager to create:
// 1. A single selfSigned ClusterIssuer
// 2. A single CA Certificate using the selfSigned ClusterIssuer from 1
// 3. A single ClusterIssuer using the CA Certificate Secret from 2 to sign registry TLS certificates for all clusters
//
// Because cert-manager does not properly support moving ClusterIssuers and Certificates,
// this function will first ensure that the necessary ClusterIssuers and Certificate exist on the management cluster,
// on every function call.
//
// When creating/upgrading a workload cluster:
// - It will add the Cluster as an owner reference to the Certificate object for garbage collection.
// - It will add the Cluster as an owner reference to the Secret for garbage collection.
//
// When creating/upgrading a management cluster:
// - It will skip adding an owner reference to the Certificate object.
// - It will add the Cluster as an owner reference to the Secret for garbage collection.
//
// The high level flow is as follows:
//  1. Ensure the certificate issuer objects exist on the management cluster.
//  2. Ensure the certificate objects exist on the management cluster.
//     a. If creating a workload cluster, add the Cluster as an owner reference to the Certificate object.
//  3. Ensure a certificate secret exists on the management cluster to be used by CAPI for Machine creation.
func EnsureCertificate(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	opts *EnsureCertificateOpts,
) error {
	err := ensureCertificateIssuerObjs(ctx, c, cluster)
	if err != nil {
		return err
	}
	err = ensureCertificateObjs(ctx, c, cluster, opts)
	if err != nil {
		return err
	}
	err = ensureCertificateSecretOnManagementCluster(ctx, c, cluster)
	if err != nil {
		return err
	}

	return nil
}

// EnsureCertificateOnRemoteCluster ensures that the registry TLS certificate secret is copied to the remote cluster.
//
// This function uses cert-manager to create:
// 1. A Certificate for each cluster using the ClusterIssuer from 3 in EnsureCertificate func
//
// The high level flow is as follows:
// 1. Renew the certificat if the certificate is half way through its expiration.
// 2. Copy the certificate secret to the remote cluster to be used by the registry Pods.
func EnsureCertificateOnRemoteCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	remoteCertificateSecretKey ctrlclient.ObjectKey,
) error {
	err := renewCertificate(ctx, c, cluster)
	if err != nil {
		return err
	}
	err = copyCertificateSecretToRemoteCluster(ctx, c, cluster, remoteCertificateSecretKey)
	if err != nil {
		return err
	}

	return nil
}

func GetTLSSecretForCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) (*corev1.Secret, error) {
	tlsSecret := certificateSecretForCluster(cluster)
	err := c.Get(ctx, ctrlclient.ObjectKeyFromObject(tlsSecret), tlsSecret)
	if err != nil {
		return nil, fmt.Errorf("error getting registry TLS secret: %w", err)
	}
	return tlsSecret, nil
}

// ensureCertificateIssuerObjs ensures that the registry TLS certificate issuer exists on the management cluster.
func ensureCertificateIssuerObjs(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) error {
	objs, err := handlersutils.UnstructuredFromBytes(certificateIssuerObjs, cluster.Namespace)
	if err != nil {
		return fmt.Errorf("error converting registry addon certificate issuer objects to unstructured: %w", err)
	}

	err = applyObjs(ctx, c, objs, nil)
	if err != nil {
		return fmt.Errorf("error applying registry addon certificate issuer objects: %w", err)
	}
	return nil
}

// ensureCertificateObjs ensures that the registry TLS certificate exists on the management cluster.
func ensureCertificateObjs(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	opts *EnsureCertificateOpts,
) error {
	certificateBytes, err := templateCertificateObj(cluster, opts)
	if err != nil {
		return err
	}
	objs, err := handlersutils.UnstructuredFromBytes(certificateBytes, cluster.Namespace)
	if err != nil {
		return fmt.Errorf("error converting registry addon certificate objects to unstructured: %w", err)
	}
	owner, err := getCertificateOwner(ctx, c, cluster)
	if err != nil {
		return fmt.Errorf("error getting owner for registry addon certificate objects: %w", err)
	}

	err = applyObjs(ctx, c, objs, owner)
	if err != nil {
		return fmt.Errorf("error applying registry addon certificate objects: %w", err)
	}

	return nil
}

// renewCertificate forces a renewal of the registry TLS certificate.
// Forcing the renewal in the same way certmanager CLI is doing it:
// https://github.com/cert-manager/cmctl/blob/a21678ffc10f02e23715dd0c8b5f43c6baaf3acb/pkg/renew/renew.go#L217
func renewCertificate(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) error {
	certificate := certificateForCluster(cluster)
	var getErr error
	if waitErr := kwait.PollUntilContextTimeout(
		ctx,
		2*time.Second,
		10*time.Second,
		true,
		func(ctx context.Context) (done bool, err error) {
			err = c.Get(ctx, ctrlclient.ObjectKeyFromObject(certificate), certificate)
			if err != nil {
				return false, err
			}

			return true, nil
		},
	); waitErr != nil {
		if getErr != nil {
			return fmt.Errorf("%w: last Get error: %w", waitErr, getErr)
		}
		return fmt.Errorf("%w: error when waiting for the registry addon certificate", waitErr)
	}

	conditions := certificate.Status.Conditions
	conditions = append(
		conditions,
		cmv1.CertificateCondition{
			Type:    cmv1.CertificateConditionIssuing,
			Status:  cmmetav1.ConditionTrue,
			Reason:  "ManuallyTriggered",
			Message: "Certificate re-issuance manually triggered",
		},
	)
	err := c.Status().Update(ctx, certificate)
	if err != nil {
		return fmt.Errorf("error updating registry addon certificate status: %w", err)
	}

	return nil
}

// ensureCertificateSecretOnManagementCluster ensures that the registry TLS Secret exists on the management cluster
func ensureCertificateSecretOnManagementCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) error {
	tlsSecret := certificateSecretForCluster(cluster)
	var getErr error
	if waitErr := kwait.PollUntilContextTimeout(
		ctx,
		2*time.Second,
		10*time.Second,
		true,
		func(ctx context.Context) (done bool, err error) {
			err = c.Get(ctx, ctrlclient.ObjectKeyFromObject(tlsSecret), tlsSecret)
			if err != nil {
				return false, err
			}
			return true, nil
		},
	); waitErr != nil {
		if getErr != nil {
			return fmt.Errorf("%w: last Get error: %w", waitErr, getErr)
		}
		return fmt.Errorf("%w: error when waiting for the registry addon TLS secret", waitErr)
	}
	err := handlersutils.EnsureClusterOwnerReferenceForObject(
		ctx,
		c,
		corev1.TypedLocalObjectReference{
			Kind: "Secret",
			Name: tlsSecret.Name,
		},
		cluster)
	if err != nil {
		return fmt.Errorf("error setting owner reference on registry TLS secret: %w", err)
	}
	return nil
}

// copyCertificateSecretToRemoteCluster copies the registry TLS Secret to the remote cluster.
func copyCertificateSecretToRemoteCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	key ctrlclient.ObjectKey,
) error {
	err := handlersutils.CopySecretToRemoteCluster(
		ctx,
		c,
		certificateSecretNameForCluster(cluster),
		key,
		cluster,
	)
	if err != nil {
		return fmt.Errorf(
			"error creating registry TLS secret on the remote cluster: %w",
			err,
		)
	}

	return nil
}

func certificateForCluster(cluster *clusterv1.Cluster) *cmv1.Certificate {
	return &cmv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: cmv1.SchemeGroupVersion.String(),
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateSecretNameForCluster(cluster),
			Namespace: cluster.Namespace,
		},
	}
}

func templateCertificateObj(
	cluster *clusterv1.Cluster,
	opts *EnsureCertificateOpts,
) ([]byte, error) {
	name := certificateSecretNameForCluster(cluster)
	var b bytes.Buffer
	templateInput := struct {
		CertificateName string
		SecretName      string
		CommonName      string
		DNSNames        []string
		IPAddresses     []string
		Duration        time.Duration
		RenewBefore     time.Duration
	}{
		CertificateName: name,
		SecretName:      name,
		CommonName:      opts.Spec.CommonName,
		DNSNames:        opts.Spec.DNSNames,
		IPAddresses:     opts.Spec.IPAddresses,
		Duration:        defaultDuration,
		RenewBefore:     30 * 24 * time.Hour, // 30 days
	}
	err := certificateObjTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("error templating registry addon certificate objects: %w", err)
	}

	return b.Bytes(), nil
}

// getCertificateOwner returns the owner reference that should be added to the Certificate objects.
// It will return the cluster if creating a workload cluster, otherwise it will return nil.
func getCertificateOwner(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) (ctrlclient.Object, error) {
	managementCluster, err := capiutils.ManagementCluster(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error getting management cluster: %w", err)
	}
	// Do not set the owner reference on the Certificate being created for the management cluster.
	// clusterctl move does not support moving cert-manager Certificate objects.
	//
	// managementCluster will be nil when the client is pointing to a bootstrap cluster.
	if managementCluster == nil || managementCluster.Name == cluster.Name {
		return nil, nil
	}

	return cluster, nil
}

// certificateSecretNameForCluster returns the name of the registry addon TLS Secret on the management cluster.
func certificateSecretNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-registry-tls", cluster.Name)
}

// tlsSecretForCluster returns a Secret object for the registry TLS certificate for the given cluster.
func certificateSecretForCluster(cluster *clusterv1.Cluster) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateSecretNameForCluster(cluster),
			Namespace: cluster.Namespace,
		},
	}
}

func applyObjs(
	ctx context.Context,
	cl ctrlclient.Client,
	objs []unstructured.Unstructured,
	owner ctrlclient.Object,
) error {
	for i := range objs {
		o := &objs[i]
		if owner != nil {
			if err := controllerutil.SetOwnerReference(owner, o, cl.Scheme()); err != nil {
				return fmt.Errorf(
					"failed to set owner reference on object %s %s: %w",
					o.GetKind(),
					ctrlclient.ObjectKeyFromObject(o),
					err,
				)
			}
		}
		err := client.ServerSideApply(
			ctx,
			cl,
			o,
			&ctrlclient.PatchOptions{
				Raw: &metav1.PatchOptions{
					FieldValidation: metav1.FieldValidationStrict,
				},
			},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to apply object %s %s: %w",
				o.GetKind(),
				ctrlclient.ObjectKeyFromObject(o),
				err,
			)
		}
	}

	return nil
}
