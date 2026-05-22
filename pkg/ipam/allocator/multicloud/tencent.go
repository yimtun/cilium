// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

// tencentClient implements CloudAPI for TencentCloud.
// TODO: replace stub methods with real TencentCloud VPC SDK calls once
// github.com/tencentcloud/tencentcloud-sdk-go is vendored.
type tencentClient struct{}

func (c *tencentClient) GetInstances(_ context.Context) (*ipamTypes.InstanceMap, error) {
	return nil, fmt.Errorf("tencentcloud: GetInstances not implemented")
}

func (c *tencentClient) GetInstance(_ context.Context, _ string) (*ipamTypes.Instance, error) {
	return nil, fmt.Errorf("tencentcloud: GetInstance not implemented")
}

func (c *tencentClient) CreateNetworkInterface(_ context.Context, _ int, _, _, _ string) (string, *ENI, error) {
	return "", nil, fmt.Errorf("tencentcloud: CreateNetworkInterface not implemented")
}

func (c *tencentClient) WaitENIAvailable(_ context.Context, _ string) error {
	return fmt.Errorf("tencentcloud: WaitENIAvailable not implemented")
}

func (c *tencentClient) AttachNetworkInterface(_ context.Context, _, _ string) error {
	return fmt.Errorf("tencentcloud: AttachNetworkInterface not implemented")
}

func (c *tencentClient) WaitENIAttached(_ context.Context, _ string) error {
	return fmt.Errorf("tencentcloud: WaitENIAttached not implemented")
}

func (c *tencentClient) AssignPrivateIpAddresses(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, fmt.Errorf("tencentcloud: AssignPrivateIpAddresses not implemented")
}

func (c *tencentClient) UnassignPrivateIpAddresses(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("tencentcloud: UnassignPrivateIpAddresses not implemented")
}
