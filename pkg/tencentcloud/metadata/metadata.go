// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cilium/cilium/pkg/safeio"
	"github.com/cilium/cilium/pkg/time"
)

const (
	// metadataURL is the TencentCloud instance metadata endpoint
	// https://cloud.tencent.com/document/product/213/4934
	metadataURL = "http://metadata.tencentyun.com/latest/meta-data"
)

// GetInstanceID returns the instance ID from metadata
func GetInstanceID(ctx context.Context) (string, error) {
	return getMetadata(ctx, "instance-id")
}

func getMetadata(ctx context.Context, path string) (string, error) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	url := fmt.Sprintf("%s/%s", metadataURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata service returned status code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	respBytes, err := safeio.ReadAllLimit(resp.Body, safeio.MB)
	if err != nil {
		return "", err
	}

	return string(respBytes), nil
}

// GetNetworkInterfaceMACs returns all MAC addresses of attached network interfaces.
func GetNetworkInterfaceMACs(ctx context.Context) ([]string, error) {
	raw, err := getMetadata(ctx, "network/interfaces/macs/")
	if err != nil {
		return nil, err
	}
	var macs []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		mac := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if mac != "" {
			macs = append(macs, mac)
		}
	}
	return macs, nil
}

// GetNetworkInterfaceLocalIPs returns all local IPv4 addresses on the given MAC.
func GetNetworkInterfaceLocalIPs(ctx context.Context, mac string) ([]string, error) {
	raw, err := getMetadata(ctx, "network/interfaces/macs/"+mac+"/local-ipv4s")
	if err != nil {
		return nil, err
	}
	var ips []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		ip := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

// GetIPSubnetMask returns the subnet mask for a specific IP on the given MAC.
func GetIPSubnetMask(ctx context.Context, mac, ip string) (string, error) {
	return getMetadata(ctx, "network/interfaces/macs/"+mac+"/local-ipv4s/"+ip+"/subnet-mask")
}
