// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

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

//go:embed templates/hosts.toml.gotmpl
var defaultRegistryMirrorPatch []byte

type mirrorConfig struct {
	URL    string
	CACert string
}

func mirrorFromImageRegistry(
	ctx context.Context,
	c ctrlclient.Client,
	imageRegistry v1alpha1.ImageRegistry,
	obj ctrlclient.Object,
) (*mirrorConfig, error) {
	// using the registry as a mirror is supported by including empty mirror object or
	// mirror with CA certificate to the registry variable.
	// ex.
	// - url: https://my-registry.com
	//   mirror: {}
	if imageRegistry.Mirror == nil {
		return nil, nil
	}
	mirrorWithOptionalCACert := &mirrorConfig{
		URL: imageRegistry.URL,
	}
	secret, err := secretForMirrorCACert(
		ctx,
		c,
		imageRegistry,
		obj.GetNamespace(),
	)
	if err != nil {
		return &mirrorConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Image Registry variable: %w",
			obj.GetNamespace(),
			imageRegistry.Mirror.SecretRef.Name,
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
	registry v1alpha1.ImageRegistry,
	objectNamespace string,
) (*corev1.Secret, error) {
	if registry.Mirror == nil || registry.Mirror.SecretRef == nil {
		return nil, nil
	}

	namespace := objectNamespace
	if registry.Mirror.SecretRef.Namespace != "" {
		namespace = registry.Mirror.SecretRef.Namespace
	}

	key := ctrlclient.ObjectKey{
		Name:      registry.Mirror.SecretRef.Name,
		Namespace: namespace,
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, key, secret)
	return secret, err
}

// Default Mirror for all registries. Use a mirror regardless of the intended registry.
// The upstream registry will be automatically used after all defined mirrors have been tried.
// reference: https://github.com/containerd/containerd/blob/main/docs/hosts.md#setup-default-mirror-for-all-registries
func generateDefaultRegistryMirrorFile(mirror *mirrorConfig) ([]cabpkv1.File, error) {
	if mirror == nil {
		return nil, nil
	}
	t, err := template.New("").Parse(string(defaultRegistryMirrorPatch))
	if err != nil {
		return nil, fmt.Errorf("fail to parse go template for registry mirror: %w", err)
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
	err = t.Execute(&b, templateInput)
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
	registry v1alpha1.ImageRegistry,
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
					Name: registry.Mirror.SecretRef.Name,
					Key:  secretKeyForMirrorCACert,
				},
			},
		},
	}
}
