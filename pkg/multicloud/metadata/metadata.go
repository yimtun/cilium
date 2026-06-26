// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cilium/cilium/pkg/safeio"
	"github.com/cilium/cilium/pkg/time"
)

const (
	detectMaxRetries    = 3
	detectRetryInterval = 5 * time.Second
)

const (
	CloudProviderTencent = "tencentcloud"
	CloudProviderAliyun  = "alibabacloud"
	CloudProviderAWS     = "aws"
	CloudProviderGCP     = "gcp"
	CloudProviderUnknown = "unknown"
)

// InstanceMetadata holds cloud instance information needed for multicloud IPAM.
type InstanceMetadata struct {
	CloudProvider string
	InstanceID    string
	Region        string
	VPCID         string
	SubnetID      string // eth0 subnet; vswitch-id for Alibaba
}

type cloudProvider interface {
	detect(ctx context.Context) bool
	getInstanceMetadata(ctx context.Context) (InstanceMetadata, error)
	providerName() string
}

var providers = []cloudProvider{
	&tencentProvider{},
	&alibabaProvider{},
	&awsProvider{},
	&gcpProvider{},
}

// GetInstanceMetadata detects the cloud provider and returns instance metadata.
// Each provider is retried up to detectMaxRetries times with detectRetryInterval between attempts.
func GetInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	for _, p := range providers {
		for attempt := 1; attempt <= detectMaxRetries; attempt++ {
			if p.detect(ctx) {
				meta, err := p.getInstanceMetadata(ctx)
				if err != nil {
					return InstanceMetadata{}, fmt.Errorf("get metadata for %s: %w", p.providerName(), err)
				}
				meta.CloudProvider = p.providerName()
				return meta, nil
			}
			if attempt < detectMaxRetries {
				select {
				case <-ctx.Done():
					return InstanceMetadata{CloudProvider: CloudProviderUnknown}, ctx.Err()
				case <-time.After(detectRetryInterval):
				}
			}
		}
	}
	return InstanceMetadata{CloudProvider: CloudProviderUnknown},
		fmt.Errorf("unable to detect cloud provider")
}

var (
	cachedCloudProvider string
	cacheMu             sync.RWMutex
)

func WarmCloudProviderCache(provider string) {
	cacheMu.Lock()
	cachedCloudProvider = provider
	cacheMu.Unlock()
}

// DetectCloudProvider returns the cloud provider name.
func DetectCloudProvider(ctx context.Context) (string, error) {
	cacheMu.RLock()
	provider := cachedCloudProvider
	cacheMu.RUnlock()
	if provider != "" {
		return provider, nil
	}
	meta, err := GetInstanceMetadata(ctx)
	if err != nil {
		return "", err
	}
	WarmCloudProviderCache(meta.CloudProvider)
	return meta.CloudProvider, nil
}

var httpClient = &http.Client{Timeout: time.Second * 10}

func httpGet(ctx context.Context, url string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := safeio.ReadAllLimit(resp.Body, safeio.MB)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func httpHead(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp, nil
}
