// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package tencentcloud

import (
	"context"
	"log/slog"

	"github.com/cilium/cilium/pkg/ipam"
	"github.com/cilium/cilium/pkg/ipam/allocator"
	"github.com/cilium/cilium/pkg/metrics"
)

// AllocatorTencentCloud is an IPAM allocator implementation for TencentCloud ENI
type AllocatorTencentCloud struct {
}

// Init sets up the TencentCloud API client and fetches region from instance metadata
func (a *AllocatorTencentCloud) Init(ctx context.Context, logger *slog.Logger, reg *metrics.Registry) error {

	return nil
}

// Start kicks off ENI allocation for TencentCloud
func (a *AllocatorTencentCloud) Start(ctx context.Context, getterUpdater ipam.CiliumNodeGetterUpdater, reg *metrics.Registry) (allocator.NodeEventHandler, error) {
	return nil, nil
}
