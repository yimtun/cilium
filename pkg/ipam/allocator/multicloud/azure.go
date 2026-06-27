// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v9"
	"k8s.io/apimachinery/pkg/util/rand"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

// azureClient implements CloudAPI for Azure.
// Pod IPs are secondary IP configurations added to the pre-created pod NIC (nic1/eth1).
// Dynamic NIC attachment is not supported on Azure — NICs must be pre-created in Terraform.
type azureClient struct {
	subscriptionID string
	resourceGroup  string
	interfaces     *armnetwork.InterfacesClient
}

// azureCredentials matches the JSON format of AZURE_CREDENTIALS_JSON.
type azureCredentials struct {
	TenantID       string `json:"tenantId"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	SubscriptionID string `json:"subscriptionId"`
}

// newAzureClient builds an Azure client from two environment variables:
//
//	AZURE_CREDENTIALS_JSON  – JSON with tenantId/clientId/clientSecret/subscriptionId
//	AZURE_RESOURCE_GROUP    – resource group containing the multicloud NICs
func newAzureClient(_ ClientKey) (CloudAPI, error) {
	raw := os.Getenv("AZURE_CREDENTIALS_JSON")
	if raw == "" {
		return nil, fmt.Errorf("azure: AZURE_CREDENTIALS_JSON not set")
	}
	var creds azureCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("azure: parse AZURE_CREDENTIALS_JSON: %w", err)
	}

	rg := os.Getenv("AZURE_RESOURCE_GROUP")
	if rg == "" {
		return nil, fmt.Errorf("azure: AZURE_RESOURCE_GROUP not set")
	}

	cred, err := azidentity.NewClientSecretCredential(creds.TenantID, creds.ClientID, creds.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: create credential: %w", err)
	}

	ifaceClient, err := armnetwork.NewInterfacesClient(creds.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: create interfaces client: %w", err)
	}

	return &azureClient{
		subscriptionID: creds.SubscriptionID,
		resourceGroup:  rg,
		interfaces:     ifaceClient,
	}, nil
}

// ── ENI ID ────────────────────────────────────────────────────────────────────

// ENI ID is the Azure NIC name, e.g. "multicloud-test-azure-pod".
func azureMakeENIID(nicName string) string { return nicName }

// azureVMName extracts the VM name from an ARM resource ID.
// e.g. "/subscriptions/.../virtualMachines/multicloud-test-azure" → "multicloud-test-azure"
func azureVMName(armID string) string {
	parts := strings.Split(armID, "/")
	return parts[len(parts)-1]
}

// ── CloudAPI implementation ───────────────────────────────────────────────────

// GetInstances lists all NICs in the resource group and groups non-primary NICs
// by VM instance. The non-primary NIC (pod NIC / eth1) is what the operator
// manages for pod IP allocation.
func (c *azureClient) GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error) {
	result := ipamTypes.NewInstanceMap()

	pager := c.interfaces.NewListPager(c.resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azure: list interfaces: %w", err)
		}
		for _, iface := range page.Value {
			if iface.Properties == nil || iface.Properties.VirtualMachine == nil || iface.Properties.VirtualMachine.ID == nil {
				continue
			}
			// Skip primary NIC (eth0); only manage the pod NIC (eth1+).
			if iface.Properties.Primary != nil && *iface.Properties.Primary {
				continue
			}
			instanceID := azureVMName(*iface.Properties.VirtualMachine.ID)
			eni := azureBuildENI(iface)
			result.Update(instanceID, ipamTypes.InterfaceRevision{Resource: eni})
		}
	}
	return result, nil
}

// GetInstance returns the pod NIC of a specific VM instance.
func (c *azureClient) GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error) {
	pager := c.interfaces.NewListPager(c.resourceGroup, nil)
	ifaces := make(map[string]ipamTypes.InterfaceRevision)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azure: list interfaces: %w", err)
		}
		for _, iface := range page.Value {
			if iface.Properties == nil || iface.Properties.VirtualMachine == nil || iface.Properties.VirtualMachine.ID == nil {
				continue
			}
			if !strings.EqualFold(azureVMName(*iface.Properties.VirtualMachine.ID), instanceID) {
				continue
			}
			if iface.Properties.Primary != nil && *iface.Properties.Primary {
				continue
			}
			eni := azureBuildENI(iface)
			ifaces[eni.NetworkInterfaceID] = ipamTypes.InterfaceRevision{Resource: eni}
		}
	}
	if len(ifaces) == 0 {
		return nil, fmt.Errorf("azure: no pod NIC found for instance %s", instanceID)
	}
	return &ipamTypes.Instance{Interfaces: ifaces}, nil
}

// GetSecurityGroups returns empty: Azure uses NSG attached to subnet/NIC.
func (c *azureClient) GetSecurityGroups(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

// CreateNetworkInterface is not supported on Azure — NICs must be pre-created in Terraform.
func (c *azureClient) CreateNetworkInterface(_ context.Context, _ int, _, _ string, _ []string) (string, *ENI, error) {
	return "", nil, fmt.Errorf("azure: dynamic NIC creation is not supported; pre-create NICs in Terraform")
}

func (c *azureClient) WaitENIAvailable(_ context.Context, _ string) error          { return nil }
func (c *azureClient) AttachNetworkInterface(_ context.Context, _, _ string) error  { return nil }
func (c *azureClient) WaitENIAttached(_ context.Context, _ string) error            { return nil }

// AssignPrivateIpAddresses adds `count` secondary IP configurations to the pod NIC.
// Azure dynamically assigns IPs from the subnet; we re-fetch after update to get actual IPs.
func (c *azureClient) AssignPrivateIpAddresses(ctx context.Context, eid string, count int) ([]string, error) {
	nicName := eid

	iface, err := c.interfaces.Get(ctx, c.resourceGroup, nicName, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: get NIC %s: %w", nicName, err)
	}

	// Record existing IPs so we can identify newly assigned ones after update.
	before := azureIPSet(iface.Interface)

	// Append new secondary IP configurations with dynamic allocation.
	newConfigs := make([]*armnetwork.InterfaceIPConfiguration, count)
	for i := range count {
		newConfigs[i] = &armnetwork.InterfaceIPConfiguration{
			Name: to.Ptr("cilium-" + rand.String(8)),
			Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
				Subnet:                    iface.Properties.IPConfigurations[0].Properties.Subnet,
			},
		}
	}
	iface.Properties.IPConfigurations = append(iface.Properties.IPConfigurations, newConfigs...)

	poller, err := c.interfaces.BeginCreateOrUpdate(ctx, c.resourceGroup, nicName, iface.Interface, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: update NIC %s: %w", nicName, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return nil, fmt.Errorf("azure: wait NIC update %s: %w", nicName, err)
	}

	// Re-fetch to get actual assigned IPs.
	updated, err := c.interfaces.Get(ctx, c.resourceGroup, nicName, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: re-fetch NIC %s: %w", nicName, err)
	}
	after := azureIPSet(updated.Interface)

	var newIPs []string
	for ip := range after {
		if !before[ip] {
			newIPs = append(newIPs, ip)
		}
	}
	return newIPs, nil
}

// UnassignPrivateIpAddresses removes specific secondary IP configurations from the pod NIC.
func (c *azureClient) UnassignPrivateIpAddresses(ctx context.Context, eid string, ips []string) error {
	nicName := eid

	iface, err := c.interfaces.Get(ctx, c.resourceGroup, nicName, nil)
	if err != nil {
		return fmt.Errorf("azure: get NIC %s: %w", nicName, err)
	}

	removeSet := make(map[string]bool, len(ips))
	for _, ip := range ips {
		removeSet[ip] = true
	}

	kept := make([]*armnetwork.InterfaceIPConfiguration, 0, len(iface.Properties.IPConfigurations))
	for _, cfg := range iface.Properties.IPConfigurations {
		if cfg.Properties != nil && cfg.Properties.PrivateIPAddress != nil && removeSet[*cfg.Properties.PrivateIPAddress] {
			continue
		}
		kept = append(kept, cfg)
	}
	iface.Properties.IPConfigurations = kept

	poller, err := c.interfaces.BeginCreateOrUpdate(ctx, c.resourceGroup, nicName, iface.Interface, nil)
	if err != nil {
		return fmt.Errorf("azure: update NIC %s: %w", nicName, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("azure: wait NIC update %s: %w", nicName, err)
	}
	return nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// azureBuildENI converts an Azure NIC into the internal ENI type.
// Primary IP (eth1's own IP) is marked as Primary=true; secondary IPs are pod IPs.
func azureBuildENI(iface *armnetwork.Interface) *ENI {
	if iface == nil || iface.Name == nil {
		return nil
	}
	eni := &ENI{NetworkInterfaceID: azureMakeENIID(*iface.Name)}
	if iface.Properties == nil {
		return eni
	}
	for _, cfg := range iface.Properties.IPConfigurations {
		if cfg.Properties == nil || cfg.Properties.PrivateIPAddress == nil {
			continue
		}
		primary := cfg.Properties.Primary != nil && *cfg.Properties.Primary
		eni.PrivateIPAddresses = append(eni.PrivateIPAddresses, PrivateIP{
			PrivateIpAddress: *cfg.Properties.PrivateIPAddress,
			Primary:          primary,
		})
	}
	return eni
}

// azureIPSet returns the set of private IPs on a NIC (excluding primary).
func azureIPSet(iface armnetwork.Interface) map[string]bool {
	set := make(map[string]bool)
	if iface.Properties == nil {
		return set
	}
	for _, cfg := range iface.Properties.IPConfigurations {
		if cfg.Properties == nil || cfg.Properties.PrivateIPAddress == nil {
			continue
		}
		if cfg.Properties.Primary != nil && *cfg.Properties.Primary {
			continue
		}
		set[*cfg.Properties.PrivateIPAddress] = true
	}
	return set
}

