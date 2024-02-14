// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

const (
	mirrorCACertPathOnRemote                = "/etc/certs/mirror.pem"
	defaultRegistryMirrorConfigPathOnRemote = "/etc/containerd/certs.d/_default/hosts.toml"
	secretKeyForMirrorCACert                = "ca.crt"
)

var (
	//go:embed templates/hosts.toml.gotmpl
	defaultRegistryMirrorPatch []byte

	defaultRegistryMirrorPatchTemplate = template.Must(
		template.New("").Parse(string(defaultRegistryMirrorPatch)),
	)
)

type mirrorConfig struct {
	URL    string
	CACert string
}

func mirrorConfigForGlobalMirror(
	ctx context.Context,
	c ctrlclient.Client,
	globalMirror v1alpha1.GlobalImageRegistryMirror,
	obj ctrlclient.Object,
) (*mirrorConfig, error) {
	mirrorWithOptionalCACert := &mirrorConfig{
		URL: globalMirror.URL,
	}
	secret, err := secretForMirrorCACert(
		ctx,
		c,
		globalMirror,
		obj.GetNamespace(),
	)
	if err != nil {
		return &mirrorConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Global Image Registry Mirror variable: %w",
			obj.GetNamespace(),
			globalMirror.Credentials.SecretRef.Name,
			err,
		)
	}

	if secret != nil {
		mirrorWithOptionalCACert.CACert = string(secret.Data[secretKeyForMirrorCACert])
	}

	return mirrorWithOptionalCACert, nil
}

// secretForMirrorCACert returns the Secret for the given mirror's CA certificate.
// Returns nil if the secret field is empty.
func secretForMirrorCACert(
	ctx context.Context,
	c ctrlclient.Reader,
	globalMirror v1alpha1.GlobalImageRegistryMirror,
	objectNamespace string,
) (*corev1.Secret, error) {
	if globalMirror.Credentials == nil || globalMirror.Credentials.SecretRef == nil {
		return nil, nil
	}

	key := ctrlclient.ObjectKey{
		Name:      globalMirror.Credentials.SecretRef.Name,
		Namespace: objectNamespace,
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, key, secret)
	return secret, err
}

// Default Mirror for all registries.
// Containerd configuration for global mirror will be created at /etc/containerd/certs.d/_default/hosts.toml
// The upstream registry will be automatically used after all defined mirrors have been tried.
// reference: https://github.com/containerd/containerd/blob/main/docs/hosts.md#setup-default-mirror-for-all-registries
func generateGlobalRegistryMirrorFile(mirror *mirrorConfig) ([]cabpkv1.File, error) {
	if mirror == nil {
		return nil, nil
	}
	templateInput := struct {
		URL        string
		CACertPath string
	}{
		URL: mirror.URL,
	}
	// CA cert is optional for mirror registry.
	// i.e. registry is using signed certificates. Insecure registry will not be allowed.
	if mirror.CACert != "" {
		templateInput.CACertPath = mirrorCACertPathOnRemote
	}

	var b bytes.Buffer
	err := defaultRegistryMirrorPatchTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("failed executing template for registry mirror: %w", err)
	}
	return []cabpkv1.File{
		{
			Path:        defaultRegistryMirrorConfigPathOnRemote,
			Content:     b.String(),
			Permissions: "0600",
		},
	}, nil
}

func generateMirrorCACertFile(
	config *mirrorConfig,
	globalMirror v1alpha1.GlobalImageRegistryMirror,
) []cabpkv1.File {
	if config == nil || config.CACert == "" {
		return nil
	}
	return []cabpkv1.File{
		{
			Path:        mirrorCACertPathOnRemote,
			Permissions: "0600",
			ContentFrom: &cabpkv1.FileSource{
				Secret: cabpkv1.SecretFileSource{
					Name: globalMirror.Credentials.SecretRef.Name,
					Key:  secretKeyForMirrorCACert,
				},
			},
		},
	}
}
