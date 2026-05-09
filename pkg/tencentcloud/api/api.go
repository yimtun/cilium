// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package api

import (
	"context"
	"fmt"
	"time"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	eniTypes "github.com/cilium/cilium/pkg/tencentcloud/eni"
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

func (c *Client) CreateNetworkInterface(ctx context.Context, secondaryPrivateIPCount int, vpcID, subnetID string) (string, *eniTypes.ENI, error) {
	req := vpc.NewCreateNetworkInterfaceRequest()
	req.VpcId = &vpcID
	req.SubnetId = &subnetID
	req.NetworkInterfaceName = strPtr("cilium-eni")
	req.SecondaryPrivateIpAddressCount = uint64Ptr(uint64(secondaryPrivateIPCount))
	resp, err := c.vpcClient.CreateNetworkInterface(req)
	if err != nil {
		return "", nil, err
	}
	ni := resp.Response.NetworkInterface
	eni := &eniTypes.ENI{
		NetworkInterfaceID: *ni.NetworkInterfaceId,
	}

	for _, ip := range ni.PrivateIpAddressSet {
		eni.PrivateIPAddresses = append(eni.PrivateIPAddresses, eniTypes.PrivateIP{
			PrivateIpAddress: *ip.PrivateIpAddress,
			Primary:          *ip.Primary,
		})
	}

	return *ni.NetworkInterfaceId, eni, nil

}

func strPtr(s string) *string    { return &s }
func uint64Ptr(u uint64) *uint64 { return &u }

// AttachNetworkInterface attaches an ENI to a CVM instance
func (c *Client) WaitENIAvailable(ctx context.Context, eniID string) error {
	for i := 0; i < 10; i++ {
		ifaces, err := c.describeNetworkInterfaces(ctx, []string{eniID})
		if err != nil {
			return err
		}
		for _, iface := range ifaces {
			if iface.State != nil && *iface.State == "AVAILABLE" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("ENI %s not available after retries", eniID)
}

func (c *Client) describeNetworkInterfaces(ctx context.Context, eniIDs []string) ([]*vpc.NetworkInterface, error) {
	req := vpc.NewDescribeNetworkInterfacesRequest()
	req.Limit = uint64Ptr(100)
	for _, id := range eniIDs {
		id := id
		req.NetworkInterfaceIds = append(req.NetworkInterfaceIds, &id)
	}

	resp, err := c.vpcClient.DescribeNetworkInterfaces(req)
	if err != nil {
		return nil, err
	}
	return resp.Response.NetworkInterfaceSet, nil
}

func (c *Client) AttachNetworkInterface(ctx context.Context, instanceID, eniID string) error {
	req := vpc.NewAttachNetworkInterfaceRequest()
	req.NetworkInterfaceId = &eniID
	req.InstanceId = &instanceID

	_, err := c.vpcClient.AttachNetworkInterface(req)
	if err != nil {
		return err
	}
	return nil
}

// GetInstance returns the ENIs for a specific CVM instance
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error) {
	instance := &ipamTypes.Instance{
		Interfaces: map[string]ipamTypes.InterfaceRevision{},
	}

	networkInterfaces, err := c.describeNetworkInterfacesByInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	for _, iface := range networkInterfaces {
		if iface.NetworkInterfaceName == nil || *iface.NetworkInterfaceName != "cilium-eni" {
			continue
		}
		ifaceID := *iface.NetworkInterfaceId
		eni := parseENI(iface)
		instance.Interfaces[ifaceID] = ipamTypes.InterfaceRevision{Resource: eni}
	}
	return instance, nil
}

func (c *Client) describeNetworkInterfacesByInstance(ctx context.Context, instanceID string) ([]*vpc.NetworkInterface, error) {
	req := vpc.NewDescribeNetworkInterfacesRequest()
	req.Limit = uint64Ptr(100)
	req.Filters = []*vpc.Filter{
		{
			Name:   strPtr("attachment.instance-id"),
			Values: []*string{&instanceID},
		},
	}

	resp, err := c.vpcClient.DescribeNetworkInterfaces(req)
	if err != nil {
		return nil, err
	}
	return resp.Response.NetworkInterfaceSet, nil
}

func parseENI(iface *vpc.NetworkInterface) *eniTypes.ENI {
	eni := &eniTypes.ENI{}
	if iface.NetworkInterfaceId != nil {
		eni.NetworkInterfaceID = *iface.NetworkInterfaceId
	}
	for _, ip := range iface.PrivateIpAddressSet {
		if ip == nil || ip.PrivateIpAddress == nil {
			continue
		}
		primary := false
		if ip.Primary != nil {
			primary = *ip.Primary
		}
		eni.PrivateIPAddresses = append(eni.PrivateIPAddresses, eniTypes.PrivateIP{
			PrivateIpAddress: *ip.PrivateIpAddress,
			Primary:          primary,
		})
	}
	return eni
}

func (c *Client) WaitENIAttached(ctx context.Context, eniID string) error {
	for i := 0; i < 20; i++ {
		ifaces, err := c.describeNetworkInterfaces(ctx, []string{eniID})
		if err != nil {
			return err
		}
		for _, iface := range ifaces {
			if iface.NetworkInterfaceState != nil && *iface.NetworkInterfaceState == "INUSE" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("ENI %s not in INUSE state after retries", eniID)
}

func (c *Client) GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error) {
	instances := ipamTypes.NewInstanceMap()

	ifaces, err := c.describeNetworkInterfaces(ctx, nil)
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Attachment == nil || iface.Attachment.InstanceId == nil {
			continue
		}
		if iface.NetworkInterfaceName == nil || *iface.NetworkInterfaceName != "cilium-eni" {
			continue
		}
		instanceID := *iface.Attachment.InstanceId
		eni := parseENI(iface)
		instances.Update(instanceID, ipamTypes.InterfaceRevision{Resource: eni})
	}
	return instances, nil
}

func (c *Client) UnassignPrivateIpAddresses(ctx context.Context, eniID string, ips []string) error {
	req := vpc.NewUnassignPrivateIpAddressesRequest()
	req.NetworkInterfaceId = &eniID
	for _, ip := range ips {
		ip := ip
		req.PrivateIpAddresses = append(req.PrivateIpAddresses, &vpc.PrivateIpAddressSpecification{
			PrivateIpAddress: &ip,
		})
	}
	_, err := c.vpcClient.UnassignPrivateIpAddresses(req)
	return err
}

func (c *Client) AssignPrivateIpAddresses(ctx context.Context, eniID string, count int) ([]string, error) {
	req := vpc.NewAssignPrivateIpAddressesRequest()
	req.NetworkInterfaceId = &eniID
	req.SecondaryPrivateIpAddressCount = uint64Ptr(uint64(count))

	resp, err := c.vpcClient.AssignPrivateIpAddresses(req)
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, ip := range resp.Response.PrivateIpAddressSet {
		if ip.PrivateIpAddress != nil {
			ips = append(ips, *ip.PrivateIpAddress)
		}
	}
	return ips, nil
}
