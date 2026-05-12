// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package eni

import (
	"context"
	"fmt"
	"os"

	"github.com/cilium/cilium/pkg/ipam"
	"github.com/cilium/cilium/pkg/ipam/stats"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/lock"
	"log/slog"
)

type ipamNodeActions interface {
	InstanceID() string
}

// Node represents a CVM node and implements ipam.NodeOperations
type Node struct {
	logger *slog.Logger

	node ipamNodeActions

	mutex lock.RWMutex

	// k8sObj is the CiliumNode custom resource representing the node
	k8sObj *v2.CiliumNode

	// manager is the InstancesManager responsible for this node
	manager *InstancesManager

	// instanceID of the node
	instanceID string
}

// UpdatedNode is called when an update to the CiliumNode is received.
func (n *Node) UpdatedNode(obj *v2.CiliumNode) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.k8sObj = obj
}

// PopulateStatusFields fills in the TencentCloud ENI status of the CiliumNode
func (n *Node) PopulateStatusFields(resource *v2.CiliumNode) {
}

// CreateInterface creates and attaches a new ENI to the node
func (n *Node) CreateInterface(ctx context.Context, allocation *ipam.AllocationAction, scopedLog *slog.Logger) (int, string,
	error) {
	scopedLog.Info("TencentCloud CreateInterface called")

	vpcID := os.Getenv("TENCENTCLOUD_VPC_ID")
	subnetID := os.Getenv("TENCENTCLOUD_SUBNET")
	securityGroupId := os.Getenv("POD_SECURITY_GROUP_ID")

	ipCount := maxSecondaryIPsPerENI

	eniID, eni, err := n.manager.api.CreateNetworkInterface(ctx, ipCount, vpcID, subnetID, securityGroupId)
	if err != nil {
		return 0, "", err
	}

	err = n.manager.api.WaitENIAvailable(ctx, eniID)
	if err != nil {
		return 0, "", err
	}

	instanceID := n.node.InstanceID()

	err = n.manager.api.AttachNetworkInterface(ctx, instanceID, eniID)
	if err != nil {
		return 0, "", err
	}

	err = n.manager.api.WaitENIAttached(ctx, eniID)
	if err != nil {
		return 0, "", err
	}
	n.manager.updateENI(instanceID, eni)

	return ipCount, eniID, nil
}

// ResyncInterfacesAndIPs retrieves ENIs and IPs from the API cache
func (n *Node) ResyncInterfacesAndIPs(ctx context.Context, scopedLog *slog.Logger) (available ipamTypes.AllocationMap, s stats.InterfaceStats, err error) {
	available = ipamTypes.AllocationMap{}
	n.manager.instances.ForeachInterface(n.instanceID,
		func(instanceID, interfaceID string, rev ipamTypes.InterfaceRevision) error {
			e, ok := rev.Resource.(*ENI)
			if !ok {
				return nil
			}
			for _, ip := range e.PrivateIPAddresses {
				if !ip.Primary {
					available[ip.PrivateIpAddress] = ipamTypes.AllocationIP{Resource: interfaceID}
				}
			}
			return nil
		})
	return available, s, nil
}

// PrepareIPAllocation returns the number of ENI IPs and interfaces that can be allocated
func (n *Node) PrepareIPAllocation(scopedLog *slog.Logger) (*ipam.AllocationAction, error) {
	a := &ipam.AllocationAction{}
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	eniCount := 0
	n.manager.instances.ForeachInterface(n.instanceID,
		func(instanceID, interfaceID string, rev ipamTypes.InterfaceRevision) error {
			e, ok := rev.Resource.(*ENI)
			if !ok {
				return nil
			}
			eniCount++
			secondaryCount := 0
			for _, ip := range e.PrivateIPAddresses {
				if !ip.Primary {
					secondaryCount++
				}
			}
			available := maxSecondaryIPsPerENI - secondaryCount
			if available > 0 && a.InterfaceID == "" {
				a.InterfaceID = interfaceID
				a.IPv4.AvailableForAllocation = available
			}
			return nil
		})

	if a.InterfaceID == "" && eniCount < maxSecondaryENIsPerInstance {
		a.EmptyInterfaceSlots = 1
	}
	return a, nil
}

// AllocateIPs assigns secondary IPs to an ENI
func (n *Node) AllocateIPs(ctx context.Context, a *ipam.AllocationAction) error {
	_, err := n.manager.api.AssignPrivateIpAddresses(ctx, a.InterfaceID, a.IPv4.MaxIPsToAllocate)
	return err
}

// AllocateStaticIP is not implemented for TencentCloud
func (n *Node) AllocateStaticIP(ctx context.Context, staticIPTags ipamTypes.Tags) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// PrepareIPRelease selects IPs to release
func (n *Node) PrepareIPRelease(excessIPs int, scopedLog *slog.Logger) *ipam.ReleaseAction {
	r := &ipam.ReleaseAction{}
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	usedIPs := make(map[string]struct{})
	for ip := range n.k8sObj.Status.IPAM.Used {
		usedIPs[ip] = struct{}{}
	}

	n.manager.instances.ForeachInterface(n.instanceID,
		func(instanceID, interfaceID string, rev ipamTypes.InterfaceRevision) error {
			if len(r.IPsToRelease) >= excessIPs {
				return nil
			}
			e, ok := rev.Resource.(*ENI)
			if !ok {
				return nil
			}
			for _, ip := range e.PrivateIPAddresses {
				if len(r.IPsToRelease) >= excessIPs {
					break
				}
				if !ip.Primary {
					if _, used := usedIPs[ip.PrivateIpAddress]; !used {
						r.InterfaceID = interfaceID
						r.IPsToRelease = append(r.IPsToRelease, ip.PrivateIpAddress)
					}
				}
			}
			return nil
		})

	return r
}

// ReleaseIPPrefixes is a no-op (prefix delegation not supported)
func (n *Node) ReleaseIPPrefixes(ctx context.Context, r *ipam.ReleaseAction) error {
	return nil
}

// ReleaseIPs releases secondary IPs from an ENI
func (n *Node) ReleaseIPs(ctx context.Context, r *ipam.ReleaseAction) error {
	return n.manager.api.UnassignPrivateIpAddresses(ctx, r.InterfaceID, r.IPsToRelease)
}

// GetMaximumAllocatableIPv4 returns the maximum number of IPv4 addresses allocatable on this instance
func (n *Node) GetMaximumAllocatableIPv4() int {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return maxSecondaryENIsPerInstance * maxSecondaryIPsPerENI
}

// GetMinimumAllocatableIPv4 returns the minimum number of IPv4 addresses that must be allocated
func (n *Node) GetMinimumAllocatableIPv4() int {
	return maxSecondaryIPsPerENI
}

// IsPrefixDelegated returns false; TencentCloud ENIs do not support prefix delegation
func (n *Node) IsPrefixDelegated() bool {
	return false
}

const (
	maxSecondaryIPsPerENI       = 6
	maxSecondaryENIsPerInstance = 4
)
