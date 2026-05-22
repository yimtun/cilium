// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"log/slog"

	"github.com/cilium/cilium/pkg/ipam"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/lock"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/time"
)

// InstancesManager maintains the list of cloud instances and their ENIs across
// all registered cloud providers. It implements ipam.AllocationImplementation.
type InstancesManager struct {
	logger    *slog.Logger
	allocator *AllocatorMultiCloud

	resyncLock lock.RWMutex

	mutex     lock.RWMutex
	instances *ipamTypes.InstanceMap
	// nodeToKey maps instanceID to the ClientKey that manages it, so that
	// InstanceSync can look up the right cloud client.
	nodeToKey map[string]ClientKey
}

func newInstancesManager(logger *slog.Logger, alloc *AllocatorMultiCloud) *InstancesManager {
	return &InstancesManager{
		logger:    logger.With(subsysLogAttr...),
		allocator: alloc,
		instances: ipamTypes.NewInstanceMap(),
		nodeToKey: make(map[string]ClientKey),
	}
}

// CreateNode returns a NodeOperations implementation for the given CiliumNode.
// The cloud API client for this node's (provider, region, vpc) key is created
// eagerly here so that Resync() can use it before any per-node operation fires.
func (m *InstancesManager) CreateNode(obj *v2.CiliumNode, node *ipam.Node) ipam.NodeOperations {
	spec := obj.Spec.MultiCloud
	key := ClientKey{
		CloudProvider: spec.CloudProvider,
		Region:        spec.Region,
	}

	// Eagerly initialise the cloud client for this key so Resync() has it ready.
	if _, err := m.allocator.getOrCreateClient(context.Background(), key); err != nil {
		m.logger.Warn("Failed to initialise cloud client for node",
			logfields.NodeName, obj.Name,
			"cloudProvider", key.CloudProvider,
			"region", key.Region,
			logfields.Error, err,
		)
	}

	m.mutex.Lock()
	m.nodeToKey[obj.InstanceID()] = key
	m.mutex.Unlock()

	return &Node{
		logger:     m.logger,
		k8sObj:     obj,
		manager:    m,
		instanceID: obj.InstanceID(),
		clientKey:  key,
	}
}

// GetPoolQuota returns available IPs per subnet pool (not used in ENI mode).
func (m *InstancesManager) GetPoolQuota() ipamTypes.PoolQuotaMap {
	return ipamTypes.PoolQuotaMap{}
}

// Resync fetches all instances from every registered cloud client.
func (m *InstancesManager) Resync(ctx context.Context) (time.Time, error) {
	m.resyncLock.Lock()
	defer m.resyncLock.Unlock()

	resyncStart := time.Now()

	m.allocator.mu.RLock()
	keys := make([]ClientKey, 0, len(m.allocator.clients))
	clients := make(map[ClientKey]CloudAPI, len(m.allocator.clients))
	for k, c := range m.allocator.clients {
		keys = append(keys, k)
		clients[k] = c
	}
	m.allocator.mu.RUnlock()

	merged := ipamTypes.NewInstanceMap()
	for _, key := range keys {
		instances, err := clients[key].GetInstances(ctx)
		if err != nil {
			m.logger.Warn("Unable to synchronize ENI list",
				"cloudProvider", key.CloudProvider,
				"region", key.Region,
				logfields.Error, err,
			)
			continue
		}
		instances.ForeachInterface("", func(instanceID, interfaceID string, rev ipamTypes.InterfaceRevision) error {
			merged.Update(instanceID, rev)
			return nil
		})
		m.logger.Info("Synchronized ENI information",
			"cloudProvider", key.CloudProvider,
			"region", key.Region,
			logfields.NumInstances, instances.NumInstances(),
		)
	}

	m.mutex.Lock()
	m.instances = merged
	m.mutex.Unlock()

	return resyncStart, nil
}

// InstanceSync re-fetches a single instance using its registered cloud client.
func (m *InstancesManager) InstanceSync(ctx context.Context, instanceID string) (time.Time, error) {
	m.resyncLock.RLock()
	defer m.resyncLock.RUnlock()

	m.mutex.RLock()
	key, ok := m.nodeToKey[instanceID]
	m.mutex.RUnlock()

	if !ok {
		return time.Time{}, nil
	}

	client, err := m.allocator.getOrCreateClient(ctx, key)
	if err != nil {
		return time.Time{}, err
	}

	resyncStart := time.Now()
	instance, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		return time.Time{}, err
	}

	m.mutex.Lock()
	m.instances.UpdateInstance(instanceID, instance)
	m.mutex.Unlock()

	m.logger.Info("Synchronized ENI information for instance",
		logfields.InstanceID, instanceID,
	)
	return resyncStart, nil
}

// HasInstance returns whether the instance is known.
func (m *InstancesManager) HasInstance(instanceID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.instances.Exists(instanceID)
}

// DeleteInstance removes an instance from the cache.
func (m *InstancesManager) DeleteInstance(instanceID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.instances.Delete(instanceID)
	delete(m.nodeToKey, instanceID)
}

func (m *InstancesManager) updateENI(instanceID string, eni *ENI) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.instances.Update(instanceID, ipamTypes.InterfaceRevision{Resource: eni})
}
