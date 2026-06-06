// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	ec2sdk "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	ec2shim "github.com/cilium/cilium/pkg/aws/ec2"
	awsTypes "github.com/cilium/cilium/pkg/aws/eni/types"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

type awsClient struct {
	client *ec2shim.Client
	rawEC2 *ec2sdk.Client
}

type awsNoopMetrics struct{}

func (awsNoopMetrics) ObserveRateLimit(_ string, _ time.Duration) {}
func (awsNoopMetrics) ObserveAPICall(_, _ string, _ float64)      {}

func newAWSClient(ctx context.Context, key ClientKey) (CloudAPI, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(key.Region))
	if err != nil {
		return nil, fmt.Errorf("aws: load config: %w", err)
	}
	rawEC2 := ec2sdk.NewFromConfig(cfg)
	shimClient := ec2shim.NewClient(
		slog.Default(),
		rawEC2,
		awsNoopMetrics{},
		4.0, 20,
		nil, nil,
		nil,
		false,
		0,
	)
	return &awsClient{client: shimClient, rawEC2: rawEC2}, nil
}

func (c *awsClient) GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error) {
	raw, err := c.client.GetInstances(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	dst := ipamTypes.NewInstanceMap()
	raw.ForeachInterface("", func(instanceID, _ string, rev ipamTypes.InterfaceRevision) error {
		awsENI, ok := rev.Resource.(*awsTypes.ENI)
		if !ok || awsENI.Number == 0 {
			return nil
		}
		dst.Update(instanceID, ipamTypes.InterfaceRevision{Resource: convertAWSENI(awsENI)})
		return nil
	})
	return dst, nil
}

func (c *awsClient) GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error) {
	raw, err := c.client.GetInstance(ctx, nil, nil, instanceID)
	if err != nil {
		return nil, err
	}
	dst := &ipamTypes.Instance{Interfaces: make(map[string]ipamTypes.InterfaceRevision)}
	for id, rev := range raw.Interfaces {
		awsENI, ok := rev.Resource.(*awsTypes.ENI)
		if !ok || awsENI.Number == 0 {
			continue
		}
		dst.Interfaces[id] = ipamTypes.InterfaceRevision{Resource: convertAWSENI(awsENI)}
	}
	return dst, nil
}

func (c *awsClient) GetSecurityGroups(ctx context.Context, instanceID, primaryIP string) ([]string, error) {
	out, err := c.rawEC2.DescribeInstances(ctx, &ec2sdk.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("aws: instance %s not found", instanceID)
	}
	for _, iface := range out.Reservations[0].Instances[0].NetworkInterfaces {
		if iface.Attachment == nil || iface.Attachment.DeviceIndex == nil || *iface.Attachment.DeviceIndex != 0 {
			continue
		}
		var sgs []string
		for _, sg := range iface.Groups {
			if sg.GroupId != nil {
				sgs = append(sgs, *sg.GroupId)
			}
		}
		return sgs, nil
	}
	return nil, fmt.Errorf("aws: primary ENI not found for instance %s", instanceID)
}

func (c *awsClient) CreateNetworkInterface(ctx context.Context, ipCount int, _, subnetID string, securityGroupIDs []string) (string, *ENI, error) {
	eniID, awsENI, err := c.client.CreateNetworkInterface(ctx, int32(ipCount), subnetID, "cilium-multicloud", securityGroupIDs, false)
	if err != nil {
		return "", nil, err
	}
	return eniID, convertAWSENI(awsENI), nil
}

func (c *awsClient) WaitENIAvailable(_ context.Context, _ string) error {
	// AWS ENIs are immediately available after creation.
	return nil
}

func (c *awsClient) AttachNetworkInterface(ctx context.Context, instanceID, eniID string) error {
	index, err := c.nextDeviceIndex(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("aws: find next device index: %w", err)
	}
	_, err = c.client.AttachNetworkInterface(ctx, index, instanceID, eniID)
	return err
}

func (c *awsClient) WaitENIAttached(ctx context.Context, eniID string) error {
	for i := 0; i < 30; i++ {
		out, err := c.rawEC2.DescribeNetworkInterfaces(ctx, &ec2sdk.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{eniID},
		})
		if err != nil {
			return err
		}
		if len(out.NetworkInterfaces) > 0 && out.NetworkInterfaces[0].Status == ec2Types.NetworkInterfaceStatusInUse {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("aws: ENI %s not in use after retries", eniID)
}

func (c *awsClient) AssignPrivateIpAddresses(ctx context.Context, eniID string, count int) ([]string, error) {
	return c.client.AssignPrivateIpAddresses(ctx, eniID, int32(count))
}

func (c *awsClient) UnassignPrivateIpAddresses(ctx context.Context, eniID string, ips []string) error {
	return c.client.UnassignPrivateIpAddresses(ctx, eniID, ips)
}

// nextDeviceIndex describes the instance's current ENIs and returns the first
// unoccupied device index starting from 1 (index 0 is the primary interface).
func (c *awsClient) nextDeviceIndex(ctx context.Context, instanceID string) (int32, error) {
	out, err := c.rawEC2.DescribeInstances(ctx, &ec2sdk.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return 0, err
	}
	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		return 1, nil
	}
	used := make(map[int32]bool)
	for _, iface := range out.Reservations[0].Instances[0].NetworkInterfaces {
		if iface.Attachment != nil && iface.Attachment.DeviceIndex != nil {
			used[*iface.Attachment.DeviceIndex] = true
		}
	}
	for i := int32(1); ; i++ {
		if !used[i] {
			return i, nil
		}
	}
}

func convertAWSENI(src *awsTypes.ENI) *ENI {
	if src == nil {
		return nil
	}
	ips := []PrivateIP{{PrivateIpAddress: src.IP, Primary: true}}
	for _, addr := range src.Addresses {
		ips = append(ips, PrivateIP{PrivateIpAddress: addr, Primary: false})
	}
	return &ENI{
		NetworkInterfaceID: src.ID,
		PrivateIPAddresses: ips,
	}
}

