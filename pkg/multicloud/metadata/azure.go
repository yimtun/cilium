// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

const (
	azureIMDS    = "http://169.254.169.254/metadata/instance"
	azureAPIVer  = "?api-version=2021-02-01"
	azureNetBase = azureIMDS + "/network" + azureAPIVer
)

var azureHeaders = map[string]string{"Metadata": "true"}

// ── IMDS response types ───────────────────────────────────────────────────────

type azureInstanceResponse struct {
	Compute azureCompute `json:"compute"`
	Network azureNetwork `json:"network"`
}

type azureCompute struct {
	Name           string `json:"name"`
	Location       string `json:"location"`
	SubscriptionID string `json:"subscriptionId"`
	ResourceGroup  string `json:"resourceGroupName"`
}

type azureNetwork struct {
	Interfaces []azureNetworkInterface `json:"interface"`
}

type azureNetworkInterface struct {
	MACAddress string          `json:"macAddress"`
	IPv4       azureIPv4Config `json:"ipv4"`
}

type azureIPv4Config struct {
	IPAddresses []azureIPAddress `json:"ipAddress"`
	Subnets     []azureSubnet    `json:"subnet"`
}

type azureIPAddress struct {
	PrivateIP string `json:"privateIpAddress"`
}

type azureSubnet struct {
	Address string `json:"address"`
	Prefix  string `json:"prefix"`
}

// ── provider ──────────────────────────────────────────────────────────────────

type azureProvider struct{}

func (p *azureProvider) providerName() string { return CloudProviderAzure }

func (p *azureProvider) detect(ctx context.Context) bool {
	_, err := httpGet(ctx, azureIMDS+azureAPIVer, azureHeaders)
	return err == nil
}

func (p *azureProvider) getInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	raw, err := httpGet(ctx, azureIMDS+azureAPIVer, azureHeaders)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("azure imds: %w", err)
	}
	var resp azureInstanceResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return InstanceMetadata{}, fmt.Errorf("azure imds parse: %w", err)
	}
	subnetCIDR := ""
	if len(resp.Network.Interfaces) > 0 && len(resp.Network.Interfaces[0].IPv4.Subnets) > 0 {
		s := resp.Network.Interfaces[0].IPv4.Subnets[0]
		subnetCIDR = s.Address + "/" + s.Prefix
	}
	return InstanceMetadata{
		InstanceID: resp.Compute.Name,
		Region:     resp.Compute.Location,
		VPCID:      resp.Compute.SubscriptionID + "/" + resp.Compute.ResourceGroup,
		SubnetID:   subnetCIDR,
	}, nil
}

// ── ENI helpers ───────────────────────────────────────────────────────────────

func azureGetNetwork(ctx context.Context) (azureNetwork, error) {
	raw, err := httpGet(ctx, azureNetBase, azureHeaders)
	if err != nil {
		return azureNetwork{}, err
	}
	var n azureNetwork
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return azureNetwork{}, fmt.Errorf("azure network parse: %w", err)
	}
	return n, nil
}

// azureNormMAC normalises Azure MAC (e.g. "000D3A123456" → "00:0d:3a:12:34:56").
func azureNormMAC(raw string) string {
	raw = strings.ToLower(strings.ReplaceAll(raw, ":", ""))
	if len(raw) != 12 {
		return raw
	}
	return strings.Join([]string{raw[0:2], raw[2:4], raw[4:6], raw[6:8], raw[8:10], raw[10:12]}, ":")
}

func azureLookupIPInfo(ctx context.Context, ipStr string) (string, string, error) {
	net_, err := azureGetNetwork(ctx)
	if err != nil {
		return "", "", fmt.Errorf("azure: get network: %w", err)
	}
	for _, iface := range net_.Interfaces {
		for _, addr := range iface.IPv4.IPAddresses {
			if addr.PrivateIP != ipStr {
				continue
			}
			if len(iface.IPv4.Subnets) == 0 {
				return "", "", fmt.Errorf("azure: no subnet for interface MAC %s", iface.MACAddress)
			}
			s := iface.IPv4.Subnets[0]
			_, ipNet, err := net.ParseCIDR(s.Address + "/" + s.Prefix)
			if err != nil {
				return "", "", fmt.Errorf("azure: parse subnet: %w", err)
			}
			return azureNormMAC(iface.MACAddress), ipNet.String(), nil
		}
	}
	return "", "", fmt.Errorf("azure: IP %s not found on any interface", ipStr)
}

func azureGetInterfacePrimaryIP(ctx context.Context, mac string) (string, error) {
	net_, err := azureGetNetwork(ctx)
	if err != nil {
		return "", fmt.Errorf("azure: get network: %w", err)
	}
	mac = strings.ToLower(strings.ReplaceAll(mac, ":", ""))
	for _, iface := range net_.Interfaces {
		ifMAC := strings.ToLower(strings.ReplaceAll(iface.MACAddress, ":", ""))
		if ifMAC != mac {
			continue
		}
		if len(iface.IPv4.IPAddresses) == 0 {
			return "", fmt.Errorf("azure: no IPs for MAC %s", mac)
		}
		return iface.IPv4.IPAddresses[0].PrivateIP, nil
	}
	return "", fmt.Errorf("azure: no interface with MAC %s", mac)
}

func azureGetPrimaryMAC(ctx context.Context) (string, error) {
	net_, err := azureGetNetwork(ctx)
	if err != nil {
		return "", fmt.Errorf("azure: get network: %w", err)
	}
	if len(net_.Interfaces) == 0 {
		return "", fmt.Errorf("azure: no interfaces")
	}
	return azureNormMAC(net_.Interfaces[0].MACAddress), nil
}
