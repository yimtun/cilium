// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package tencentcloud

import (
	//
	"context"
	"fmt"
	"log/slog"
	"os"

	// cilium
	operatorOption "github.com/cilium/cilium/operator/option"
	"github.com/cilium/cilium/pkg/ipam"
	"github.com/cilium/cilium/pkg/ipam/allocator"
	ipamMetrics "github.com/cilium/cilium/pkg/ipam/metrics"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/metrics"
	tcApi "github.com/cilium/cilium/pkg/tencentcloud/api"
	"github.com/cilium/cilium/pkg/tencentcloud/eni"

	// 3 part
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcProfile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcVpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
)

var subsysLogAttr = []any{logfields.LogSubsys, "ipam-allocator-tencentcloud"}

// implementation AllocatorProvider  interface
type AllocatorTencentCloud struct {
	rootLogger *slog.Logger
	logger     *slog.Logger
	client     *tcApi.Client
}

// Init sets up the TencentCloud API client and fetches region from instance metadata
func (a *AllocatorTencentCloud) Init(ctx context.Context, logger *slog.Logger, reg *metrics.Registry) error {
	a.rootLogger = logger
	a.logger = logger.With(subsysLogAttr...)

	secretID := os.Getenv("TENCENTCLOUD_SECRET_ID")
	secretKey := os.Getenv("TENCENTCLOUD_SECRET_KEY")
	region := os.Getenv("TENCENTCLOUD_REGION")

	if secretID == "" || secretKey == "" || region == "" {
		return fmt.Errorf("TENCENTCLOUD_SECRET_ID, TENCENTCLOUD_SECRET_KEY and TENCENTCLOUD_REGION must be set")
	}

	credential := common.NewCredential(secretID, secretKey)
	prof := tcProfile.NewClientProfile()
	prof.HttpProfile.Endpoint = fmt.Sprintf("vpc.%s.tencentcloudapi.com", region)

	vpcClient, err := tcVpc.NewClient(credential, region, prof)
	if err != nil {
		return fmt.Errorf("unable to create TencentCloud VPC client: %w", err)
	}

	a.client = tcApi.NewClient(vpcClient)

	return nil
}

// Start kicks off ENI allocation for TencentCloud
func (a *AllocatorTencentCloud) Start(ctx context.Context, getterUpdater ipam.CiliumNodeGetterUpdater, reg *metrics.Registry) (allocator.NodeEventHandler, error) {
	a.logger.Info("Starting TencentCloud ENI allocator...")
	instancesManager := eni.NewInstancesManager(a.rootLogger, a.client)
	nodeManager, err := ipam.NewNodeManager(a.logger, instancesManager, getterUpdater, &ipamMetrics.NoOpMetrics{},
		operatorOption.Config.ParallelAllocWorkers,
		false,
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize TencentCloud node manager: %w", err)
	}

	if err := nodeManager.Start(ctx); err != nil {
		return nil, err
	}

	return nodeManager, nil
}
