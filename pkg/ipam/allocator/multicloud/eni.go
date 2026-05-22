// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import ipamTypes "github.com/cilium/cilium/pkg/ipam/types"

// ENI represents a cloud network interface with its allocated private IPs.
type ENI struct {
	NetworkInterfaceID string
	PrivateIPAddresses []PrivateIP
}

// PrivateIP holds a private IP address and whether it is the primary IP.
type PrivateIP struct {
	PrivateIpAddress string
	Primary          bool
}

func (e *ENI) InterfaceID() string { return e.NetworkInterfaceID }

func (e *ENI) ForeachAddress(instanceID string, fn ipamTypes.AddressIterator) error {
	for _, ip := range e.PrivateIPAddresses {
		if err := fn(instanceID, e.NetworkInterfaceID, ip.PrivateIpAddress, "", ip); err != nil {
			return err
		}
	}
	return nil
}

func (e *ENI) DeepCopyInterface() ipamTypes.Interface {
	cp := *e
	cp.PrivateIPAddresses = make([]PrivateIP, len(e.PrivateIPAddresses))
	copy(cp.PrivateIPAddresses, e.PrivateIPAddresses)
	return &cp
}
