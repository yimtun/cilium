// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package eni

import ipamTypes "github.com/cilium/cilium/pkg/ipam/types"

type ENI struct {
	NetworkInterfaceID string
	PrivateIPAddresses []PrivateIP
}

type PrivateIP struct {
	PrivateIpAddress string
	Primary          bool
}

func (e *ENI) InterfaceID() string {
	return e.NetworkInterfaceID
}

func (e *ENI) ForeachAddress(instanceID string, fn ipamTypes.AddressIterator) error {
	for _, ip := range e.PrivateIPAddresses {
		if err := fn(instanceID, e.NetworkInterfaceID, ip.PrivateIpAddress, "", ip); err != nil {
			return err
		}
	}
	return nil
}

func (e *ENI) DeepCopyInterface() ipamTypes.Interface {
	copy := *e
	copy.PrivateIPAddresses = make([]PrivateIP, len(e.PrivateIPAddresses))
	copy.PrivateIPAddresses = append(copy.PrivateIPAddresses[:0], e.PrivateIPAddresses...)
	return &copy
}
