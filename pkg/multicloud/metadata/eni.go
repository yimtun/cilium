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

// LookupIPInfo returns the MAC address and subnet CIDR for the given IP on this instance.
func LookupIPInfo(ctx context.Context, cloudProvider, ipStr string) (mac, subnetCIDR string, err error) {
	switch cloudProvider {
	case CloudProviderTencent:
		return tencentLookupIPInfo(ctx, ipStr)
	case CloudProviderAWS:
		return awsLookupIPInfo(ctx, ipStr)
	case CloudProviderAliyun:
		return aliLookupIPInfo(ctx, ipStr)
	case CloudProviderGCP:
		return gcpLookupIPInfo(ctx, ipStr)
	default:
		return "", "", fmt.Errorf("unsupported cloud provider: %s", cloudProvider)
	}
}

func tencentLookupIPInfo(ctx context.Context, ipStr string) (string, string, error) {
	raw, err := httpGet(ctx, tcBase+"/network/interfaces/macs/", nil)
	if err != nil {
		return "", "", fmt.Errorf("list MACs: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		mac := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if mac == "" {
			continue
		}
		ipsRaw, err := httpGet(ctx, tcBase+"/network/interfaces/macs/"+mac+"/local-ipv4s", nil)
		if err != nil {
			continue
		}
		for _, ipLine := range strings.Split(strings.TrimSpace(ipsRaw), "\n") {
			if strings.TrimSuffix(strings.TrimSpace(ipLine), "/") != ipStr {
				continue
			}
			maskStr, err := httpGet(ctx, tcBase+"/network/interfaces/macs/"+mac+"/local-ipv4s/"+ipStr+"/subnet-mask", nil)
			if err != nil {
				return "", "", fmt.Errorf("subnet-mask: %w", err)
			}
			parsed := net.ParseIP(strings.TrimSpace(maskStr))
			if parsed == nil {
				return "", "", fmt.Errorf("invalid subnet mask: %s", maskStr)
			}
			mask := net.IPMask(parsed.To4())
			ip := net.ParseIP(ipStr).To4()
			subnet := &net.IPNet{IP: ip.Mask(mask), Mask: mask}
			return mac, subnet.String(), nil
		}
	}
	return "", "", fmt.Errorf("IP %s not found on any TC interface", ipStr)
}

func awsLookupIPInfo(ctx context.Context, ipStr string) (string, string, error) {
	token := getIMDSv2Token(ctx)
	raw, err := awsGet(ctx, awsBase+"/network/interfaces/macs/", token)
	if err != nil {
		return "", "", fmt.Errorf("list MACs: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		mac := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if mac == "" {
			continue
		}
		ipsRaw, err := awsGet(ctx, awsBase+"/network/interfaces/macs/"+mac+"/local-ipv4s", token)
		if err != nil {
			continue
		}
		for _, ipLine := range strings.Split(strings.TrimSpace(ipsRaw), "\n") {
			if strings.TrimSuffix(strings.TrimSpace(ipLine), "/") != ipStr {
				continue
			}
			cidrBlock, err := awsGet(ctx, awsBase+"/network/interfaces/macs/"+mac+"/subnet-ipv4-cidr-block", token)
			if err != nil {
				return "", "", fmt.Errorf("subnet-ipv4-cidr-block: %w", err)
			}
			return mac, strings.TrimSpace(cidrBlock), nil
		}
	}
	return "", "", fmt.Errorf("IP %s not found on any AWS interface", ipStr)
}

func aliLookupIPInfo(ctx context.Context, ipStr string) (string, string, error) {
	raw, err := httpGet(ctx, aliBase+"/network/interfaces/macs/", nil)
	if err != nil {
		return "", "", fmt.Errorf("list MACs: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		mac := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if mac == "" {
			continue
		}
		// private-ipv4s returns a JSON array: ["10.203.1.101","10.203.1.5"]
		ipsRaw, err := httpGet(ctx, aliBase+"/network/interfaces/macs/"+mac+"/private-ipv4s", nil)
		if err != nil {
			continue
		}
		var ips []string
		if err := json.Unmarshal([]byte(ipsRaw), &ips); err != nil {
			continue
		}
		for _, entry := range ips {
			if strings.TrimSpace(entry) != ipStr {
				continue
			}
			cidrBlock, err := httpGet(ctx, aliBase+"/network/interfaces/macs/"+mac+"/vswitch-cidr-block", nil)
			if err != nil {
				return "", "", fmt.Errorf("vswitch-cidr-block: %w", err)
			}
			return mac, strings.TrimSpace(cidrBlock), nil
		}
	}
	return "", "", fmt.Errorf("IP %s not found on any Alibaba interface", ipStr)
}

// GetInterfacePrimaryIP returns the primary private IPv4 of the ENI identified by mac.
func GetInterfacePrimaryIP(ctx context.Context, cloudProvider, mac string) (string, error) {
	switch cloudProvider {
	case CloudProviderTencent:
		raw, err := httpGet(ctx, tcBase+"/network/interfaces/macs/"+mac+"/primary-local-ipv4", nil)
		return strings.TrimSpace(raw), err
	case CloudProviderAWS:
		token := getIMDSv2Token(ctx)
		raw, err := awsGet(ctx, awsBase+"/network/interfaces/macs/"+mac+"/local-ipv4s", token)
		if err != nil {
			return "", err
		}
		lines := strings.Split(strings.TrimSpace(raw), "\n")
		return strings.TrimSuffix(strings.TrimSpace(lines[0]), "/"), nil
	case CloudProviderAliyun:
		raw, err := httpGet(ctx, aliBase+"/network/interfaces/macs/"+mac+"/private-ipv4s", nil)
		if err != nil {
			return "", err
		}
		var ips []string
		if err := json.Unmarshal([]byte(raw), &ips); err != nil {
			return "", fmt.Errorf("parse private-ipv4s: %w", err)
		}
		if len(ips) == 0 {
			return "", fmt.Errorf("no IPs for MAC %s", mac)
		}
		return strings.TrimSpace(ips[0]), nil
	case CloudProviderGCP:
		return gcpGetInterfacePrimaryIP(ctx, mac)
	default:
		return "", fmt.Errorf("unsupported cloud provider: %s", cloudProvider)
	}
}

// GetPrimaryMAC returns the MAC address of the primary network interface (eth0).
func GetPrimaryMAC(ctx context.Context, cloudProvider string) (string, error) {
	switch cloudProvider {
	case CloudProviderTencent:
		raw, err := httpGet(ctx, tcBase+"/mac", nil)
		return strings.TrimSpace(raw), err
	case CloudProviderAWS:
		token := getIMDSv2Token(ctx)
		raw, err := awsGet(ctx, awsBase+"/mac", token)
		return strings.TrimSpace(raw), err
	case CloudProviderAliyun:
		raw, err := httpGet(ctx, aliBase+"/mac", nil)
		return strings.TrimSpace(raw), err
	case CloudProviderGCP:
		raw, err := httpGet(ctx, gcpBase+"/network-interfaces/0/mac", gcpHeaders)
		return strings.TrimSpace(raw), err
	default:
		return "", fmt.Errorf("unsupported cloud provider: %s", cloudProvider)
	}
}
