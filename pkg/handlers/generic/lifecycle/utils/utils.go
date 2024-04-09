// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

const (
	defaultCRSConfigMapKey = "custom-resources.yaml"
)

var (
	defaultStorageClassKey = "storageclass.kubernetes.io/is-default-class"
	defaultStorageClassMap = map[string]string{
		defaultStorageClassKey: "true",
	}
)

func EnsureCRSForClusterFromObjects(
	ctx context.Context,
	crsName string,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	objects ...runtime.Object,
) error {
	resources := make([]crsv1.ResourceRef, 0, len(objects))
	for _, obj := range objects {
		var name string
		var kind crsv1.ClusterResourceSetResourceKind
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			sec, secOk := obj.(*corev1.Secret)
			if !secOk {
				return fmt.Errorf(
					"cannot create ClusterResourceSet with obj %v only secrets and configmaps are supported",
					obj,
				)
			}
			name = sec.Name
			kind = crsv1.SecretClusterResourceSetResourceKind
		} else {
			name = cm.Name
			kind = crsv1.ConfigMapClusterResourceSetResourceKind
		}
		resources = append(resources, crsv1.ResourceRef{
			Name: name,
			Kind: string(kind),
		})
	}

	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      crsName,
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: resources,
			Strategy:  string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(cluster, crs, c.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err := client.ServerSideApply(ctx, c, crs)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}

	return nil
}

// EnsureNamespace will create the namespece if it does not exist.
func EnsureNamespace(ctx context.Context, c ctrlclient.Client, name string) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	// check if namespace exists and return early if it does
	if err := c.Get(ctx, ctrlclient.ObjectKeyFromObject(ns), ns); err == nil {
		return nil
	}

	err := client.ServerSideApply(ctx, c, ns)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}

	return nil
}

func RetrieveValuesTemplateConfigMap(
	ctx context.Context,
	c ctrlclient.Client,
	configMapName,
	defaultsNamespace string,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      configMapName,
		},
	}
	configMapObjName := ctrlclient.ObjectKeyFromObject(
		configMap,
	)
	err := c.Get(ctx, configMapObjName, configMap)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve installation values template ConfigMap %q: %w",
			configMapObjName,
			err,
		)
	}
	return configMap, nil
}

func CreateConfigMapForCRS(configMapName, configMapNamespace string,
	objs ...runtime.Object,
) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		Data: make(map[string]string),
	}
	l := make([][]byte, 0, len(objs))
	for _, v := range objs {
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(v)
		if err != nil {
			return nil, err
		}
		objYaml, err := utilyaml.FromUnstructured([]unstructured.Unstructured{
			{
				Object: obj,
			},
		})
		if err != nil {
			return nil, err
		}
		l = append(l, objYaml)
	}
	cm.Data[defaultCRSConfigMapKey] = string(utilyaml.JoinYaml(l...))
	return cm, nil
}
