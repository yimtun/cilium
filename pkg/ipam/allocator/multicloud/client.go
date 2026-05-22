// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	mcMeta "github.com/cilium/cilium/pkg/multicloud/metadata"
)

// ClientKey uniquely identifies a cloud API client by cloud provider and region.
// VPC is an API-call parameter, not a client boundary.
type ClientKey struct {
	CloudProvider string
	Region        string
}

// CloudAPI is the interface that each cloud provider's API client must implement.
type CloudAPI interface {
	GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error)
	GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error)
	CreateNetworkInterface(ctx context.Context, ipCount int, vpcID, subnetID, securityGroupID string) (string, *ENI, error)
	WaitENIAvailable(ctx context.Context, eniID string) error
	AttachNetworkInterface(ctx context.Context, instanceID, eniID string) error
	WaitENIAttached(ctx context.Context, eniID string) error
	AssignPrivateIpAddresses(ctx context.Context, eniID string, count int) ([]string, error)
	UnassignPrivateIpAddresses(ctx context.Context, eniID string, ips []string) error
}

// newCloudClient creates a CloudAPI implementation for the given key.
// Credentials are read from environment variables per cloud provider.
func newCloudClient(_ context.Context, key ClientKey) (CloudAPI, error) {
	switch key.CloudProvider {
	case mcMeta.CloudProviderTencent:
		return newTencentClient(key)
	case mcMeta.CloudProviderAliyun:
		return newAlibabaClient(key)
	case mcMeta.CloudProviderAWS:
		return newAWSClient(key)
	default:
		return nil, fmt.Errorf("unknown cloud provider %q", key.CloudProvider)
	}
}

func newTencentClient(key ClientKey) (CloudAPI, error) {
	// TODO: initialize real TencentCloud VPC client once SDK is vendored.
	// Credentials: TENCENTCLOUD_SECRET_ID, TENCENTCLOUD_SECRET_KEY from env.
	_ = key
	return &tencentClient{}, nil
}
