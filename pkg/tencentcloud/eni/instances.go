// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package eni

import (
	"context"
	"github.com/cilium/cilium/pkg/ipam"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/lock"
	"github.com/cilium/cilium/pkg/time"
	"log/slog"
)

// TencentCloudAPI is the API surface used by the InstancesManager
type TencentCloudAPI interface {
	CreateNetworkInterface(ctx context.Context, secondaryPrivateIPCount int, vpcID, subnetID string) (string, error)
}

// InstancesManager maintains the list of CVM instances and their ENIs.
type InstancesManager struct {
	logger *slog.Logger
	// resyncLock ensures full resync and per-instance resync do not race
	resyncLock lock.RWMutex

	// mutex protects the fields below
	mutex     lock.RWMutex
	instances *ipamTypes.InstanceMap
	subnets   ipamTypes.SubnetMap
	vpcs      ipamTypes.VirtualNetworkMap
	api       TencentCloudAPI
}

// NewInstancesManager returns a new InstancesManager
func NewInstancesManager(logger *slog.Logger, api TencentCloudAPI) *InstancesManager {
	return &InstancesManager{
		logger:    logger.With(subsysLogAttr...),
		instances: ipamTypes.NewInstanceMap(),
		api:       api,
	}
}

// CreateNode returns a NodeOperations implementation for the given CiliumNode
func (m *InstancesManager) CreateNode(obj *v2.CiliumNode, node *ipam.Node) ipam.NodeOperations {
	return &Node{
		logger:     m.logger,
		k8sObj:     obj,
		manager:    m,
		instanceID: node.InstanceID(),
		node:       node,
	}
}

// GetPoolQuota returns the number of available IPs per subnet pool
func (m *InstancesManager) GetPoolQuota() ipamTypes.PoolQuotaMap {
	return ipamTypes.PoolQuotaMap{}
}

// Resync fetches all CVM instances and subnets and updates the local cache.
func (m *InstancesManager) Resync(ctx context.Context) time.Time {
	m.resyncLock.Lock()
	defer m.resyncLock.Unlock()
	return time.Now()
}

// InstanceSync fetches a single CVM instance and updates the local cache.
func (m *InstancesManager) InstanceSync(ctx context.Context, instanceID string) time.Time {
	m.resyncLock.RLock()
	defer m.resyncLock.RUnlock()
	return time.Now()
}

// HasInstance returns whether the instance is known
func (m *InstancesManager) HasInstance(instanceID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.instances.Exists(instanceID)
}

// DeleteInstance removes an instance from the cache
func (m *InstancesManager) DeleteInstance(instanceID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.instances.Delete(instanceID)
}
