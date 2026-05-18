// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"strings"
)

const tcBase = "http://metadata.tencentyun.com/latest/meta-data"

type tencentProvider struct{}

func (t *tencentProvider) providerName() string { return CloudProviderTencent }

func (t *tencentProvider) detect(ctx context.Context) bool {
	body, err := httpGet(ctx, tcBase+"/instance-id", nil)
	if err != nil {
		return false
	}
	return strings.HasPrefix(body, "ins-") && len(body) > 4
}

func (t *tencentProvider) getInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	instanceID, err := httpGet(ctx, tcBase+"/instance-id", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("instance-id: %w", err)
	}
	region, err := httpGet(ctx, tcBase+"/placement/region", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("region: %w", err)
	}
	mac, err := httpGet(ctx, tcBase+"/mac", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("mac: %w", err)
	}
	vpcID, err := httpGet(ctx, fmt.Sprintf("%s/network/interfaces/macs/%s/vpc-id", tcBase, mac), nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("vpc-id: %w", err)
	}
	subnetID, err := httpGet(ctx, fmt.Sprintf("%s/network/interfaces/macs/%s/subnet-id", tcBase, mac), nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("subnet-id: %w", err)
	}
	return InstanceMetadata{
		InstanceID: instanceID,
		Region:     region,
		VPCID:      vpcID,
		SubnetID:   subnetID,
	}, nil
}
