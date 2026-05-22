// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

// alibabaClient implements CloudAPI for Alibaba Cloud.
// TODO: initialize real Alibaba Cloud ECS/VPC client.
// Wrap pkg/alibabacloud/api.Client methods here.
type alibabaClient struct{}

func newAlibabaClient(key ClientKey) (CloudAPI, error) {
	// TODO: use key.Region to create a region-scoped vpc+ecs client pair via
	// vpc.NewClientWithProvider(key.Region) and ecs.NewClientWithProvider(key.Region),
	// then wrap with alibabacloudAPI.NewClient(...).
	_ = key
	return &alibabaClient{}, nil
}

func (c *alibabaClient) GetInstances(_ context.Context) (*ipamTypes.InstanceMap, error) {
	return nil, fmt.Errorf("alibabacloud: GetInstances not implemented")
}

func (c *alibabaClient) GetInstance(_ context.Context, _ string) (*ipamTypes.Instance, error) {
	return nil, fmt.Errorf("alibabacloud: GetInstance not implemented")
}

func (c *alibabaClient) CreateNetworkInterface(_ context.Context, _ int, _, _, _ string) (string, *ENI, error) {
	return "", nil, fmt.Errorf("alibabacloud: CreateNetworkInterface not implemented")
}

func (c *alibabaClient) WaitENIAvailable(_ context.Context, _ string) error {
	return fmt.Errorf("alibabacloud: WaitENIAvailable not implemented")
}

func (c *alibabaClient) AttachNetworkInterface(_ context.Context, _, _ string) error {
	return fmt.Errorf("alibabacloud: AttachNetworkInterface not implemented")
}

func (c *alibabaClient) WaitENIAttached(_ context.Context, _ string) error {
	return fmt.Errorf("alibabacloud: WaitENIAttached not implemented")
}

func (c *alibabaClient) AssignPrivateIpAddresses(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, fmt.Errorf("alibabacloud: AssignPrivateIpAddresses not implemented")
}

func (c *alibabaClient) UnassignPrivateIpAddresses(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("alibabacloud: UnassignPrivateIpAddresses not implemented")
}
