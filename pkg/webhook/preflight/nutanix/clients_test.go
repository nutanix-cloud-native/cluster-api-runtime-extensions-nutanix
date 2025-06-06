// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

type mockv3client struct {
	user *prismv3.UserIntentResponse
	err  error
}

func (m *mockv3client) GetCurrentLoggedInUser(ctx context.Context) (*prismv3.UserIntentResponse, error) {
	return m.user, m.err
}

type mockv4client struct {
	getImageByIdFunc func(
		uuid *string,
	) (
		*vmmv4.GetImageApiResponse, error,
	)

	listImagesFunc func(
		page,
		limit *int,
		filter,
		orderby,
		select_ *string,
		args ...map[string]interface{},
	) (
		*vmmv4.ListImagesApiResponse,
		error,
	)

	getClusterByIdFunc func(id *string) (*clustermgmtv4.GetClusterApiResponse, error)

	listClustersFunc func(
		page,
		limit *int,
		filter,
		orderby,
		apply,
		select_ *string,
		args ...map[string]interface{},
	) (*clustermgmtv4.ListClustersApiResponse, error)

	listStorageContainersFunc func(
		page,
		limit *int,
		filter,
		orderby,
		select_ *string,
		args ...map[string]interface{},
	) (*clustermgmtv4.ListStorageContainersApiResponse, error)
}

func (m *mockv4client) GetImageById(uuid *string) (*vmmv4.GetImageApiResponse, error) {
	return m.getImageByIdFunc(uuid)
}

func (m *mockv4client) ListImages(
	page, limit *int,
	filter, orderby, select_ *string,
	args ...map[string]interface{},
) (*vmmv4.ListImagesApiResponse, error) {
	return m.listImagesFunc(page, limit, filter, orderby, select_)
}

func (m *mockv4client) GetClusterById(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
	return m.getClusterByIdFunc(id)
}

func (m *mockv4client) ListClusters(
	page, limit *int,
	filter, orderby, apply, select_ *string,
	args ...map[string]interface{},
) (*clustermgmtv4.ListClustersApiResponse, error) {
	return m.listClustersFunc(page, limit, filter, orderby, apply, select_, args...)
}

func (m *mockv4client) ListStorageContainers(
	page, limit *int,
	filter, orderby, select_ *string,
	args ...map[string]interface{},
) (*clustermgmtv4.ListStorageContainersApiResponse, error) {
	return m.listStorageContainersFunc(page, limit, filter, orderby, select_, args...)
}
