// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cilium/cilium/pkg/ipam"
	"github.com/cilium/cilium/pkg/ipam/allocator"
	ipamMetrics "github.com/cilium/cilium/pkg/ipam/metrics"
	"github.com/cilium/cilium/pkg/logging/logfields"
)

var subsysLogAttr = []any{logfields.LogSubsys, "ipam-allocator-multicloud"}

// AllocatorMultiCloud implements the IPAM allocator for IPAMMultiCloud mode.
// It maintains one CloudAPI client per (cloudProvider, region, vpcID) tuple,
// created lazily as CiliumNodes appear.
type AllocatorMultiCloud struct {
	ParallelAllocWorkers int64

	rootLogger *slog.Logger
	logger     *slog.Logger

	mu      sync.RWMutex
	clients map[ClientKey]CloudAPI
}

// Init initialises logging and the client map. Cloud clients are created lazily
// in getOrCreateClient when the first CiliumNode for a given key appears.
func (a *AllocatorMultiCloud) Init(ctx context.Context, logger *slog.Logger) error {
	a.rootLogger = logger
	a.logger = logger.With(subsysLogAttr...)
	a.clients = make(map[ClientKey]CloudAPI)
	return nil
}

// Start creates the shared InstancesManager and NodeManager.
func (a *AllocatorMultiCloud) Start(ctx context.Context, getterUpdater ipam.CiliumNodeGetterUpdater, metrics *ipamMetrics.Metrics) (allocator.NodeEventHandler, error) {
	a.logger.Info("Starting MultiCloud ENI allocator")

	instancesManager := newInstancesManager(a.rootLogger, a)
	nodeManager, err := ipam.NewNodeManager(
		a.logger,
		instancesManager,
		getterUpdater,
		metrics,
		a.ParallelAllocWorkers,
		true,
		0,
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to initialise MultiCloud node manager: %w", err)
	}

	if err := nodeManager.Start(ctx); err != nil {
		return nil, err
	}
	return nodeManager, nil
}

// getOrCreateClient returns an existing client for the key, or creates one.
// Safe for concurrent use.
func (a *AllocatorMultiCloud) getOrCreateClient(ctx context.Context, key ClientKey) (CloudAPI, error) {
	a.mu.RLock()
	c, ok := a.clients[key]
	a.mu.RUnlock()
	if ok {
		return c, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok = a.clients[key]; ok {
		return c, nil
	}

	c, err := newCloudClient(ctx, key)
	if err != nil {
		return nil, err
	}
	a.clients[key] = c
	a.logger.Info("Created cloud API client",
		"cloudProvider", key.CloudProvider,
		"region", key.Region,
	)
	return c, nil
}
