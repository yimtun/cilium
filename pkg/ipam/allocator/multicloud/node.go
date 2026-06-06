// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cilium/cilium/pkg/ipam"
	"github.com/cilium/cilium/pkg/ipam/stats"
	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/lock"
)

const nodeInternalIPType = "InternalIP"

const (
	maxSecondaryIPsPerENI       = 6
	maxSecondaryENIsPerInstance = 4
)

// Node represents a single cloud VM node and implements ipam.NodeOperations.
type Node struct {
	logger     *slog.Logger
	k8sObj     *v2.CiliumNode
	manager    *InstancesManager
	instanceID string
	clientKey  ClientKey
	mutex      lock.RWMutex
}

func (n *Node) UpdatedNode(obj *v2.CiliumNode) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.k8sObj = obj

	// If the clientKey was created before the agent wrote cloud metadata,
	// pick it up now so subsequent operations use the correct cloud client.
	if n.clientKey.CloudProvider == "" && obj.Spec.MultiCloud.CloudProvider != "" {
		newKey := ClientKey{
			CloudProvider: obj.Spec.MultiCloud.CloudProvider,
			Region:        obj.Spec.MultiCloud.Region,
		}
		n.clientKey = newKey
		n.manager.mutex.Lock()
		n.manager.nodeToKey[n.instanceID] = newKey
		n.manager.mutex.Unlock()
		if _, err := n.manager.allocator.getOrCreateClient(context.Background(), newKey); err != nil {
			n.logger.Warn("Failed to initialise cloud client after node update",
				"nodeName", obj.Name,
				"cloudProvider", newKey.CloudProvider,
				"region", newKey.Region,
				"error", err,
			)
		}
	}
}

func (n *Node) PopulateStatusFields(resource *v2.CiliumNode) {}

// getSecurityGroupsFromCache returns security groups from an existing cilium-eni
// in the instance cache, avoiding a cloud API call on subsequent ENI creations.
func (n *Node) getSecurityGroupsFromCache() []string {
	var sgs []string
	n.manager.instances.ForeachInterface(n.instanceID,
		func(_, _ string, rev ipamTypes.InterfaceRevision) error {
			if e, ok := rev.Resource.(*ENI); ok && len(e.SecurityGroups) > 0 {
				sgs = e.SecurityGroups
			}
			return nil
		})
	return sgs
}

// getInternalIP returns the node's primary private IP from the CiliumNode addresses.
func getInternalIP(node *v2.CiliumNode) string {
	for _, addr := range node.Spec.Addresses {
		if addr.Type == nodeInternalIPType {
			return addr.IP
		}
	}
	return ""
}

// CreateInterface creates a new ENI and attaches it to the node.
func (n *Node) CreateInterface(ctx context.Context, allocation *ipam.AllocationAction, scopedLog *slog.Logger) (int, string, error) {
	client, err := n.manager.allocator.getOrCreateClient(ctx, n.clientKey)
	if err != nil {
		return 0, "", fmt.Errorf("get cloud client: %w", err)
	}

	vpcID := n.k8sObj.Spec.MultiCloud.VPCID
	subnetID := n.k8sObj.Spec.MultiCloud.SubnetID

	securityGroups := n.getSecurityGroupsFromCache()
	if len(securityGroups) == 0 {
		internalIP := getInternalIP(n.k8sObj)
		securityGroups, err = client.GetSecurityGroups(ctx, n.instanceID, internalIP)
		if err != nil {
			return 0, "", fmt.Errorf("get security groups: %w", err)
		}
	}

	eniID, eni, err := client.CreateNetworkInterface(ctx, maxSecondaryIPsPerENI, vpcID, subnetID, securityGroups)
	if err != nil {
		return 0, "", err
	}

	if err := client.WaitENIAvailable(ctx, eniID); err != nil {
		return 0, "", err
	}
	if err := client.AttachNetworkInterface(ctx, n.instanceID, eniID); err != nil {
		return 0, "", err
	}
	if err := client.WaitENIAttached(ctx, eniID); err != nil {
		return 0, "", err
	}

	n.manager.updateENI(n.instanceID, eni)
	return maxSecondaryIPsPerENI, eniID, nil
}

// ResyncInterfacesAndIPs retrieves the available secondary IPs from the instance cache.
func (n *Node) ResyncInterfacesAndIPs(ctx context.Context, scopedLog *slog.Logger) (ipamTypes.AllocationMap, stats.InterfaceStats, error) {
	available := ipamTypes.AllocationMap{}
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
	return available, stats.InterfaceStats{}, nil
}

// PrepareIPAllocation returns an AllocationAction describing how many IPs can be allocated.
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
			secondary := 0
			for _, ip := range e.PrivateIPAddresses {
				if !ip.Primary {
					secondary++
				}
			}
			available := maxSecondaryIPsPerENI - secondary
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

// AllocateIPs assigns secondary IPs to an existing ENI.
func (n *Node) AllocateIPs(ctx context.Context, a *ipam.AllocationAction) error {
	client, err := n.manager.allocator.getOrCreateClient(ctx, n.clientKey)
	if err != nil {
		return err
	}
	_, err = client.AssignPrivateIpAddresses(ctx, a.InterfaceID, a.IPv4.MaxIPsToAllocate)
	return err
}

// AllocateStaticIP is not supported.
func (n *Node) AllocateStaticIP(ctx context.Context, tags ipamTypes.Tags) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// PrepareIPRelease selects excess IPs to release.
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
			// Once an ENI is selected, do not collect IPs from other ENIs —
			// ReleaseIPs calls the cloud API with a single interfaceID.
			if r.InterfaceID != "" {
				return nil
			}
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

// ReleaseIPs releases secondary IPs from an ENI.
func (n *Node) ReleaseIPs(ctx context.Context, r *ipam.ReleaseAction) error {
	client, err := n.manager.allocator.getOrCreateClient(ctx, n.clientKey)
	if err != nil {
		return err
	}
	return client.UnassignPrivateIpAddresses(ctx, r.InterfaceID, r.IPsToRelease)
}

func (n *Node) ReleaseIPPrefixes(ctx context.Context, r *ipam.ReleaseAction) error { return nil }

func (n *Node) GetMaximumAllocatableIPv4() int {
	return maxSecondaryENIsPerInstance * maxSecondaryIPsPerENI
}

func (n *Node) GetMinimumAllocatableIPv4() int { return maxSecondaryIPsPerENI }

func (n *Node) IsPrefixDelegated() bool { return false }
