// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

const gcpBase = "http://metadata.google.internal/computeMetadata/v1/instance"

// GCP metadata server requires this header; requests without it get 403.
var gcpHeaders = map[string]string{"Metadata-Flavor": "Google"}

var (
	// "projects/532461681306/zones/asia-northeast3-a" → "asia-northeast3-a"
	gcpZoneRe = regexp.MustCompile(`zones/([^/]+)$`)
	// "projects/532461681306/networks/multicloud-test-gcp" → "multicloud-test-gcp"
	gcpNetworkRe = regexp.MustCompile(`networks/([^/]+)$`)
	// "projects/532461681306/regions/asia-northeast3/subnetworks/multicloud-test-gcp" → "multicloud-test-gcp"
	gcpSubnetRe = regexp.MustCompile(`subnetworks/([^/]+)$`)
)

type gcpProvider struct{}

func (g *gcpProvider) providerName() string { return CloudProviderGCP }

func gcpGet(ctx context.Context, path string) (string, error) {
	return httpGet(ctx, gcpBase+path, gcpHeaders)
}

// detect: GCP instance IDs are plain uint64 numerics.
// metadata.google.internal won't resolve on other clouds, so a successful
// response with a numeric body is an unambiguous GCP signal.
func (g *gcpProvider) detect(ctx context.Context) bool {
	body, err := gcpGet(ctx, "/id")
	if err != nil {
		return false
	}
	_, err = strconv.ParseUint(strings.TrimSpace(body), 10, 64)
	return err == nil
}

func (g *gcpProvider) getInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	instanceID, err := gcpGet(ctx, "/id")
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("instance-id: %w", err)
	}

	// "projects/532461681306/zones/asia-northeast3-a"
	zonePath, err := gcpGet(ctx, "/zone")
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("zone: %w", err)
	}
	m := gcpZoneRe.FindStringSubmatch(zonePath)
	if len(m) < 2 {
		return InstanceMetadata{}, fmt.Errorf("unexpected zone format: %s", zonePath)
	}
	zone := m[1]
	// "asia-northeast3-a" → "asia-northeast3"
	region := zone[:strings.LastIndex(zone, "-")]

	// "projects/532461681306/networks/multicloud-test-gcp"
	networkPath, err := gcpGet(ctx, "/network-interfaces/0/network")
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("network: %w", err)
	}
	m = gcpNetworkRe.FindStringSubmatch(networkPath)
	if len(m) < 2 {
		return InstanceMetadata{}, fmt.Errorf("unexpected network format: %s", networkPath)
	}
	vpcID := m[1]

	// "projects/532461681306/regions/asia-northeast3/subnetworks/multicloud-test-gcp"
	subnetPath, err := gcpGet(ctx, "/network-interfaces/0/subnetwork")
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("subnetwork: %w", err)
	}
	m = gcpSubnetRe.FindStringSubmatch(subnetPath)
	if len(m) < 2 {
		return InstanceMetadata{}, fmt.Errorf("unexpected subnetwork format: %s", subnetPath)
	}
	subnetID := m[1]

	return InstanceMetadata{
		InstanceID: instanceID,
		Region:     region,
		VPCID:      vpcID,
		SubnetID:   subnetID,
	}, nil
}

// gcpLookupIPInfo finds the MAC and subnet CIDR for an IP by iterating GCP NICs.
// An alias IP falls within the subnet of the NIC it is attached to.
func gcpLookupIPInfo(ctx context.Context, ipStr string) (string, string, error) {
	target := net.ParseIP(ipStr)
	if target == nil {
		return "", "", fmt.Errorf("gcp: invalid IP %s", ipStr)
	}
	for idx := 0; idx < 8; idx++ {
		base := fmt.Sprintf("/network-interfaces/%d", idx)
		mac, err := gcpGet(ctx, base+"/mac")
		if err != nil {
			break
		}
		mac = strings.TrimSpace(mac)
		primaryIP, err := gcpGet(ctx, base+"/ip")
		if err != nil {
			continue
		}
		maskStr, err := gcpGet(ctx, base+"/subnetmask")
		if err != nil {
			continue
		}
		parsed := net.ParseIP(strings.TrimSpace(maskStr))
		if parsed == nil {
			continue
		}
		mask := net.IPMask(parsed.To4())
		ip := net.ParseIP(strings.TrimSpace(primaryIP)).To4()
		if ip == nil {
			continue
		}
		subnet := &net.IPNet{IP: ip.Mask(mask), Mask: mask}
		if subnet.Contains(target) {
			return mac, subnet.String(), nil
		}
	}
	return "", "", fmt.Errorf("gcp: IP %s not found on any interface", ipStr)
}

// gcpGetInterfacePrimaryIP returns the primary IP of the interface with the given MAC.
func gcpGetInterfacePrimaryIP(ctx context.Context, mac string) (string, error) {
	mac = strings.ToLower(strings.TrimSpace(mac))
	for idx := 0; idx < 8; idx++ {
		base := fmt.Sprintf("/network-interfaces/%d", idx)
		ifMAC, err := gcpGet(ctx, base+"/mac")
		if err != nil {
			break
		}
		if strings.ToLower(strings.TrimSpace(ifMAC)) == mac {
			ip, err := gcpGet(ctx, base+"/ip")
			return strings.TrimSpace(ip), err
		}
	}
	return "", fmt.Errorf("gcp: no interface with MAC %s", mac)
}
