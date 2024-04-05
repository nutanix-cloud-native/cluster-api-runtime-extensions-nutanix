// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	mirrorCACertPathOnRemote                = "/etc/certs/mirror.pem"
	defaultRegistryMirrorConfigPathOnRemote = "/etc/containerd/certs.d/_default/hosts.toml"
	secretKeyForMirrorCACert                = "ca.crt"

	tomlMergeImage                              = "ghcr.io/mesosphere/toml-merge:v0.2.0"
	containerdPatchesDirOnRemote                = "/etc/containerd/cre.d"
	containerdApplyPatchesScriptOnRemote        = "/etc/containerd/apply-patches.sh"
	containerdApplyPatchesScriptOnRemoteCommand = "/bin/bash " + containerdApplyPatchesScriptOnRemote
)

var (
	//go:embed templates/hosts.toml.gotmpl
	defaultRegistryMirrorPatch []byte

	defaultRegistryMirrorPatchTemplate = template.Must(
		template.New("").Parse(string(defaultRegistryMirrorPatch)),
	)

	//go:embed templates/containerd-registry-config-drop-in.toml
	containerdRegistryConfigDropIn             []byte
	containerdRegistryConfigDropInFileOnRemote = path.Join(
		containerdPatchesDirOnRemote,
		"registry-config.toml",
	)

	//go:embed templates/containerd-apply-patches.sh.gotmpl
	containerdApplyConfigPatchesScript []byte
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
	formattedURL, err := formatURLForContainerd(mirror.URL)
	if err != nil {
		return nil, fmt.Errorf("failed formatting registry mirror URL for Containerd: %w", err)
	}
	templateInput := struct {
		URL        string
		CACertPath string
	}{
		URL: formattedURL,
	}
	// CA cert is optional for mirror registry.
	// i.e. registry is using signed certificates. Insecure registry will not be allowed.
	if mirror.CACert != "" {
		templateInput.CACertPath = mirrorCACertPathOnRemote
	}

	var b bytes.Buffer
	err = defaultRegistryMirrorPatchTemplate.Execute(&b, templateInput)
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
	mirror *mirrorConfig,
	globalMirror v1alpha1.GlobalImageRegistryMirror,
) []cabpkv1.File {
	if mirror == nil || mirror.CACert == "" {
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

func formatURLForContainerd(uri string) (string, error) {
	mirrorURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", fmt.Errorf("failed parsing mirror: %w", err)
	}

	mirror := fmt.Sprintf("%s://%s", mirrorURL.Scheme, mirrorURL.Host)
	// assume Containerd expects the following pattern:
	//   scheme://host/v2/path
	mirrorPath := "v2"
	if mirrorURL.Path != "" {
		mirrorPath = path.Join(mirrorPath, mirrorURL.Path)
	}
	// using path.Join on all elements incorrectly drops a "/" from "https://"
	return fmt.Sprintf("%s/%s", mirror, mirrorPath), nil
}

func generateContainerdRegistryConfigDropInFile() []cabpkv1.File {
	return []cabpkv1.File{
		{
			Path:        containerdRegistryConfigDropInFileOnRemote,
			Content:     string(containerdRegistryConfigDropIn),
			Permissions: "0600",
		},
	}
}

func generateContainerdApplyPatchesScript() ([]cabpkv1.File, string, error) {
	t, err := template.New("").Parse(string(containerdApplyConfigPatchesScript))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse go template: %w", err)
	}

	templateInput := struct {
		TOMLMergeImage string
		PatchDir       string
	}{
		TOMLMergeImage: tomlMergeImage,
		PatchDir:       containerdPatchesDirOnRemote,
	}

	var b bytes.Buffer
	err = t.Execute(&b, templateInput)
	if err != nil {
		return nil, "", fmt.Errorf("failed executing template: %w", err)
	}

	return []cabpkv1.File{
		{
			Path:        containerdApplyPatchesScriptOnRemote,
			Content:     b.String(),
			Permissions: "0700",
		},
	}, containerdApplyPatchesScriptOnRemoteCommand, nil
}
