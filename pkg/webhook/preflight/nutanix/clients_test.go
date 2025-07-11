// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

var _ = client(&mocknclient{})

// mocknclient is a mock implementation of the client interface for testing purposes.
type mocknclient struct {
	user *prismv3.UserIntentResponse
	err  error

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

	GetSubnetByIdFunc func(id *string) (*netv4.GetSubnetApiResponse, error)

	ListSubnetsFunc func(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		expand_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (*netv4.ListSubnetsApiResponse, error)
}

func (m *mocknclient) GetCurrentLoggedInUser(ctx context.Context) (*prismv3.UserIntentResponse, error) {
	return m.user, m.err
}

func (m *mocknclient) GetImageById(uuid *string) (*vmmv4.GetImageApiResponse, error) {
	return m.getImageByIdFunc(uuid)
}

func (m *mocknclient) ListImages(
	page, limit *int,
	filter, orderby, select_ *string,
	args ...map[string]interface{},
) (*vmmv4.ListImagesApiResponse, error) {
	return m.listImagesFunc(page, limit, filter, orderby, select_)
}

func (m *mocknclient) GetClusterById(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
	return m.getClusterByIdFunc(id)
}

func (m *mocknclient) ListClusters(
	page, limit *int,
	filter, orderby, apply, select_ *string,
	args ...map[string]interface{},
) (*clustermgmtv4.ListClustersApiResponse, error) {
	return m.listClustersFunc(page, limit, filter, orderby, apply, select_, args...)
}

func (m *mocknclient) ListStorageContainers(
	page, limit *int,
	filter, orderby, select_ *string,
	args ...map[string]interface{},
) (*clustermgmtv4.ListStorageContainersApiResponse, error) {
	return m.listStorageContainersFunc(page, limit, filter, orderby, select_, args...)
}

func (m *mocknclient) GetSubnetById(id *string) (*netv4.GetSubnetApiResponse, error) {
	return m.GetSubnetByIdFunc(id)
}

func (m *mocknclient) ListSubnets(
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	expand_ *string,
	select_ *string,
	args ...map[string]interface{},
) (*netv4.ListSubnetsApiResponse, error) {
	return m.ListSubnetsFunc(page_, limit_, filter_, orderby_, expand_, select_, args...)
}
