// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

// awsClient implements CloudAPI for AWS.
// TODO: initialize real AWS EC2 client (pkg/aws/ec2.Client) with region-scoped
// config and wrap its methods here.
type awsClient struct{}

func newAWSClient(key ClientKey) (CloudAPI, error) {
	// TODO: use key.Region to load aws.Config with config.LoadDefaultConfig(ctx,
	// config.WithRegion(key.Region)), then create ec2.NewFromConfig(cfg) and
	// wrap with ec2shim.NewClient(...).
	_ = key
	return &awsClient{}, nil
}

func (c *awsClient) GetInstances(_ context.Context) (*ipamTypes.InstanceMap, error) {
	return nil, fmt.Errorf("aws: GetInstances not implemented")
}

func (c *awsClient) GetInstance(_ context.Context, _ string) (*ipamTypes.Instance, error) {
	return nil, fmt.Errorf("aws: GetInstance not implemented")
}

func (c *awsClient) CreateNetworkInterface(_ context.Context, _ int, _, _, _ string) (string, *ENI, error) {
	return "", nil, fmt.Errorf("aws: CreateNetworkInterface not implemented")
}

func (c *awsClient) WaitENIAvailable(_ context.Context, _ string) error {
	return fmt.Errorf("aws: WaitENIAvailable not implemented")
}

func (c *awsClient) AttachNetworkInterface(_ context.Context, _, _ string) error {
	return fmt.Errorf("aws: AttachNetworkInterface not implemented")
}

func (c *awsClient) WaitENIAttached(_ context.Context, _ string) error {
	return fmt.Errorf("aws: WaitENIAttached not implemented")
}

func (c *awsClient) AssignPrivateIpAddresses(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, fmt.Errorf("aws: AssignPrivateIpAddresses not implemented")
}

func (c *awsClient) UnassignPrivateIpAddresses(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("aws: UnassignPrivateIpAddresses not implemented")
}
