// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package api

import (
	"context"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
)

// Client is a TencentCloud VPC API client
type Client struct {
	vpcClient *vpc.Client
}

// NewClient creates a new TencentCloud API client
func NewClient(vpcClient *vpc.Client) *Client {
	return &Client{
		vpcClient: vpcClient,
	}
}

func (c *Client) CreateNetworkInterface(ctx context.Context, secondaryPrivateIPCount int, vpcID, subnetID string) (string, error) {
	req := vpc.NewCreateNetworkInterfaceRequest()
	req.VpcId = &vpcID
	req.SubnetId = &subnetID
	req.NetworkInterfaceName = strPtr("cilium-eni")
	req.SecondaryPrivateIpAddressCount = uint64Ptr(uint64(secondaryPrivateIPCount))
	resp, err := c.vpcClient.CreateNetworkInterface(req)
	if err != nil {
		return "", err
	}

	return *resp.Response.NetworkInterface.NetworkInterfaceName, nil
}

func strPtr(s string) *string    { return &s }
func uint64Ptr(u uint64) *uint64 { return &u }
