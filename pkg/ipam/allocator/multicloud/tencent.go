// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	tcProfile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcVpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

type tencentClient struct {
	vpcClient *tcVpc.Client
}

func newTencentClient(key ClientKey) (CloudAPI, error) {
	secretID := os.Getenv("TENCENTCLOUD_SECRET_ID")
	secretKey := os.Getenv("TENCENTCLOUD_SECRET_KEY")
	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("tencentcloud: TENCENTCLOUD_SECRET_ID and TENCENTCLOUD_SECRET_KEY must be set")
	}

	credential := common.NewCredential(secretID, secretKey)
	prof := tcProfile.NewClientProfile()
	prof.HttpProfile.Endpoint = fmt.Sprintf("vpc.%s.tencentcloudapi.com", key.Region)

	vpcClient, err := tcVpc.NewClient(credential, key.Region, prof)
	if err != nil {
		return nil, fmt.Errorf("tencentcloud: create vpc client: %w", err)
	}
	return &tencentClient{vpcClient: vpcClient}, nil
}

func (c *tencentClient) GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error) {
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
		instances.Update(instanceID, ipamTypes.InterfaceRevision{Resource: parseTencentENI(iface)})
	}
	return instances, nil
}

func (c *tencentClient) GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error) {
	instance := &ipamTypes.Instance{Interfaces: map[string]ipamTypes.InterfaceRevision{}}
	ifaces, err := c.describeNetworkInterfacesByInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.NetworkInterfaceName == nil || *iface.NetworkInterfaceName != "cilium-eni" {
			continue
		}
		eni := parseTencentENI(iface)
		instance.Interfaces[eni.NetworkInterfaceID] = ipamTypes.InterfaceRevision{Resource: eni}
	}
	return instance, nil
}

func (c *tencentClient) CreateNetworkInterface(ctx context.Context, ipCount int, vpcID, subnetID, securityGroupID string) (string, *ENI, error) {
	req := tcVpc.NewCreateNetworkInterfaceRequest()
	req.VpcId = &vpcID
	req.SubnetId = &subnetID
	req.NetworkInterfaceName = strPtr("cilium-eni")
	req.SecondaryPrivateIpAddressCount = uint64Ptr(uint64(ipCount))
	req.SecurityGroupIds = []*string{&securityGroupID}

	resp, err := c.vpcClient.CreateNetworkInterface(req)
	if err != nil {
		return "", nil, err
	}
	ni := resp.Response.NetworkInterface
	eni := &ENI{NetworkInterfaceID: *ni.NetworkInterfaceId}
	for _, ip := range ni.PrivateIpAddressSet {
		eni.PrivateIPAddresses = append(eni.PrivateIPAddresses, PrivateIP{
			PrivateIpAddress: *ip.PrivateIpAddress,
			Primary:          *ip.Primary,
		})
	}
	return *ni.NetworkInterfaceId, eni, nil
}

func (c *tencentClient) WaitENIAvailable(ctx context.Context, eniID string) error {
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
	return fmt.Errorf("tencentcloud: ENI %s not available after retries", eniID)
}

func (c *tencentClient) AttachNetworkInterface(ctx context.Context, instanceID, eniID string) error {
	req := tcVpc.NewAttachNetworkInterfaceRequest()
	req.NetworkInterfaceId = &eniID
	req.InstanceId = &instanceID
	_, err := c.vpcClient.AttachNetworkInterface(req)
	return err
}

func (c *tencentClient) WaitENIAttached(ctx context.Context, eniID string) error {
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
	return fmt.Errorf("tencentcloud: ENI %s not in INUSE state after retries", eniID)
}

func (c *tencentClient) AssignPrivateIpAddresses(ctx context.Context, eniID string, count int) ([]string, error) {
	req := tcVpc.NewAssignPrivateIpAddressesRequest()
	req.NetworkInterfaceId = &eniID
	req.SecondaryPrivateIpAddressCount = uint64Ptr(uint64(count))
	for i := 0; i < 10; i++ {
		resp, err := c.vpcClient.AssignPrivateIpAddresses(req)
		if err == nil {
			var ips []string
			for _, ip := range resp.Response.PrivateIpAddressSet {
				if ip.PrivateIpAddress != nil {
					ips = append(ips, *ip.PrivateIpAddress)
				}
			}
			return ips, nil
		}
		var sdkErr *tcErrors.TencentCloudSDKError
		if errors.As(err, &sdkErr) && sdkErr.GetCode() == "UnsupportedOperation.MutexOperationTaskRunning" {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("tencentcloud: AssignPrivateIpAddresses still blocked after retries on %s", eniID)
}

func (c *tencentClient) UnassignPrivateIpAddresses(ctx context.Context, eniID string, ips []string) error {
	req := tcVpc.NewUnassignPrivateIpAddressesRequest()
	req.NetworkInterfaceId = &eniID
	for i := range ips {
		req.PrivateIpAddresses = append(req.PrivateIpAddresses, &tcVpc.PrivateIpAddressSpecification{
			PrivateIpAddress: &ips[i],
		})
	}
	for i := 0; i < 10; i++ {
		_, err := c.vpcClient.UnassignPrivateIpAddresses(req)
		if err == nil {
			return nil
		}
		var sdkErr *tcErrors.TencentCloudSDKError
		if errors.As(err, &sdkErr) && sdkErr.GetCode() == "UnsupportedOperation.MutexOperationTaskRunning" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}
		return err
	}
	return fmt.Errorf("tencentcloud: UnassignPrivateIpAddresses still blocked after retries on %s", eniID)
}

func (c *tencentClient) describeNetworkInterfaces(ctx context.Context, eniIDs []string) ([]*tcVpc.NetworkInterface, error) {
	req := tcVpc.NewDescribeNetworkInterfacesRequest()
	req.Limit = uint64Ptr(100)
	for i := range eniIDs {
		req.NetworkInterfaceIds = append(req.NetworkInterfaceIds, &eniIDs[i])
	}
	resp, err := c.vpcClient.DescribeNetworkInterfaces(req)
	if err != nil {
		return nil, err
	}
	return resp.Response.NetworkInterfaceSet, nil
}

func (c *tencentClient) describeNetworkInterfacesByInstance(ctx context.Context, instanceID string) ([]*tcVpc.NetworkInterface, error) {
	req := tcVpc.NewDescribeNetworkInterfacesRequest()
	req.Limit = uint64Ptr(100)
	req.Filters = []*tcVpc.Filter{
		{Name: strPtr("attachment.instance-id"), Values: []*string{&instanceID}},
	}
	resp, err := c.vpcClient.DescribeNetworkInterfaces(req)
	if err != nil {
		return nil, err
	}
	return resp.Response.NetworkInterfaceSet, nil
}

func parseTencentENI(iface *tcVpc.NetworkInterface) *ENI {
	eni := &ENI{}
	if iface.NetworkInterfaceId != nil {
		eni.NetworkInterfaceID = *iface.NetworkInterfaceId
	}
	for _, ip := range iface.PrivateIpAddressSet {
		if ip == nil || ip.PrivateIpAddress == nil {
			continue
		}
		primary := ip.Primary != nil && *ip.Primary
		eni.PrivateIPAddresses = append(eni.PrivateIPAddresses, PrivateIP{
			PrivateIpAddress: *ip.PrivateIpAddress,
			Primary:          primary,
		})
	}
	return eni
}

func strPtr(s string) *string    { return &s }
func uint64Ptr(u uint64) *uint64 { return &u }
