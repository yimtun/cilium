// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"

	alibabaAPI "github.com/cilium/cilium/pkg/alibabacloud/api"
	eniTypes "github.com/cilium/cilium/pkg/alibabacloud/eni/types"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

type alibabaClient struct {
	client *alibabaAPI.Client
}

type alibabaNoopMetrics struct{}

func (alibabaNoopMetrics) ObserveRateLimit(_ string, _ time.Duration) {}
func (alibabaNoopMetrics) ObserveAPICall(_, _ string, _ float64)      {}

func newAlibabaClient(key ClientKey) (CloudAPI, error) {
	vpcClient, err := vpc.NewClientWithProvider(key.Region)
	if err != nil {
		return nil, fmt.Errorf("alibaba: create vpc client: %w", err)
	}
	ecsClient, err := ecs.NewClientWithProvider(key.Region)
	if err != nil {
		return nil, fmt.Errorf("alibaba: create ecs client: %w", err)
	}
	vpcClient.Network = "vpc"
	ecsClient.Network = "vpc"
	vpcClient.GetConfig().WithScheme("HTTPS")
	ecsClient.GetConfig().WithScheme("HTTPS")

	return &alibabaClient{
		client: alibabaAPI.NewClient(vpcClient, ecsClient, alibabaNoopMetrics{}, 4.0, 20, nil),
	}, nil
}

func (c *alibabaClient) GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error) {
	vpcs, err := c.client.GetVPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("alibaba: list VPCs: %w", err)
	}
	subnets, err := c.client.GetVSwitches(ctx)
	if err != nil {
		return nil, fmt.Errorf("alibaba: list VSwitches: %w", err)
	}
	raw, err := c.client.GetInstances(ctx, vpcs, subnets)
	if err != nil {
		return nil, err
	}
	dst := ipamTypes.NewInstanceMap()
	raw.ForeachInterface("", func(instanceID, _ string, rev ipamTypes.InterfaceRevision) error {
		aliENI, ok := rev.Resource.(*eniTypes.ENI)
		if !ok || aliENI.Type == eniTypes.ENITypePrimary {
			return nil
		}
		dst.Update(instanceID, ipamTypes.InterfaceRevision{Resource: convertAlibabaENI(aliENI)})
		return nil
	})
	return dst, nil
}

func (c *alibabaClient) GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error) {
	vpcs, err := c.client.GetVPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("alibaba: list VPCs: %w", err)
	}
	subnets, err := c.client.GetVSwitches(ctx)
	if err != nil {
		return nil, fmt.Errorf("alibaba: list VSwitches: %w", err)
	}
	raw, err := c.client.GetInstance(ctx, vpcs, subnets, instanceID)
	if err != nil {
		return nil, err
	}
	dst := &ipamTypes.Instance{Interfaces: make(map[string]ipamTypes.InterfaceRevision)}
	for id, rev := range raw.Interfaces {
		aliENI, ok := rev.Resource.(*eniTypes.ENI)
		if !ok || aliENI.Type == eniTypes.ENITypePrimary {
			continue
		}
		dst.Interfaces[id] = ipamTypes.InterfaceRevision{Resource: convertAlibabaENI(aliENI)}
	}
	return dst, nil
}

func (c *alibabaClient) CreateNetworkInterface(ctx context.Context, ipCount int, _, subnetID, securityGroupID string) (string, *ENI, error) {
	eniID, aliENI, err := c.client.CreateNetworkInterface(ctx, ipCount, subnetID, []string{securityGroupID}, nil)
	if err != nil {
		return "", nil, err
	}
	return eniID, convertAlibabaENI(aliENI), nil
}

func (c *alibabaClient) WaitENIAvailable(_ context.Context, _ string) error {
	// Alibaba ENIs are available immediately after creation.
	return nil
}

func (c *alibabaClient) AttachNetworkInterface(ctx context.Context, instanceID, eniID string) error {
	return c.client.AttachNetworkInterface(ctx, instanceID, eniID)
}

func (c *alibabaClient) WaitENIAttached(ctx context.Context, eniID string) error {
	_, err := c.client.WaitENIAttached(ctx, eniID)
	return err
}

func (c *alibabaClient) AssignPrivateIpAddresses(ctx context.Context, eniID string, count int) ([]string, error) {
	return c.client.AssignPrivateIPAddresses(ctx, eniID, count)
}

func (c *alibabaClient) UnassignPrivateIpAddresses(ctx context.Context, eniID string, ips []string) error {
	return c.client.UnassignPrivateIPAddresses(ctx, eniID, ips)
}

func convertAlibabaENI(src *eniTypes.ENI) *ENI {
	if src == nil {
		return nil
	}
	ips := make([]PrivateIP, 0, len(src.PrivateIPSets))
	for _, p := range src.PrivateIPSets {
		ips = append(ips, PrivateIP{
			PrivateIpAddress: p.PrivateIpAddress,
			Primary:          p.Primary,
		})
	}
	return &ENI{
		NetworkInterfaceID: src.NetworkInterfaceID,
		PrivateIPAddresses: ips,
	}
}

