// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"strings"
)

const aliBase = "http://100.100.100.200/latest/meta-data"

type alibabaProvider struct{}

func (a *alibabaProvider) providerName() string { return CloudProviderAliyun }

func (a *alibabaProvider) detect(ctx context.Context) bool {
	body, err := httpGet(ctx, aliBase+"/instance-id", nil)
	if err != nil {
		return false
	}
	// Alibaba instance IDs start with "i-"; endpoint 100.100.100.200 is Ali-unique
	return strings.HasPrefix(body, "i-") && len(body) > 2
}

func (a *alibabaProvider) getInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	instanceID, err := httpGet(ctx, aliBase+"/instance-id", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("instance-id: %w", err)
	}
	region, err := httpGet(ctx, aliBase+"/region-id", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("region-id: %w", err)
	}
	vpcID, err := httpGet(ctx, aliBase+"/vpc-id", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("vpc-id: %w", err)
	}
	vswitchID, err := httpGet(ctx, aliBase+"/vswitch-id", nil)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("vswitch-id: %w", err)
	}
	return InstanceMetadata{
		InstanceID: instanceID,
		Region:     region,
		VPCID:      vpcID,
		SubnetID:   vswitchID,
	}, nil
}
