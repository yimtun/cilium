// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cilium/cilium/pkg/safeio"
)

const (
	CloudProviderTencent = "tencentcloud"
	CloudProviderAliyun  = "alibabacloud"
	CloudProviderAWS     = "aws"
	CloudProviderGCP     = "gcp"
	CloudProviderAzure   = "azure"
	CloudProviderUnknown = "unknown"
)

type cloudDetector struct {
	name   string
	detect func(ctx context.Context) bool
}

// DetectCloudProvider probes cloud metadata endpoints directly from the host
// network and returns the cloud provider name.
func DetectCloudProvider(ctx context.Context) (string, error) {
	detectors := []cloudDetector{
		{CloudProviderTencent, detectTencent},
		{CloudProviderAliyun, detectAliyun},
		{CloudProviderAWS, detectAWS},
		{CloudProviderGCP, detectGCP},
		{CloudProviderAzure, detectAzure},
	}

	for _, d := range detectors {
		if d.detect(ctx) {
			return d.name, nil
		}
	}
	return CloudProviderUnknown, fmt.Errorf("unable to detect cloud provider")
}

func detectTencent(ctx context.Context) bool {
	body, err := getMetadata(ctx, "http://metadata.tencentyun.com/latest/meta-data/instance-id", nil)
	if err != nil {
		return false
	}
	return strings.HasPrefix(body, "ins-") && len(body) > 4
}

func detectAliyun(ctx context.Context) bool {
	body, err := getMetadata(ctx, "http://100.100.100.200/latest/meta-data/instance-id", nil)
	if err != nil {
		return false
	}
	return strings.HasPrefix(body, "i-") && len(body) > 2
}

func detectAWS(ctx context.Context) bool {
	resp, err := headMetadata(ctx, "http://169.254.169.254/latest/meta-data/instance-id")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(resp.Header.Get("Server")), "ec2ws")
}

func detectGCP(ctx context.Context) bool {
	body, err := getMetadata(ctx,
		"http://metadata.google.internal/computeMetadata/v1/instance/id",
		map[string]string{"Metadata-Flavor": "Google"},
	)
	if err != nil {
		return false
	}
	_, err = strconv.ParseInt(strings.TrimSpace(body), 10, 64)
	return err == nil
}

func detectAzure(ctx context.Context) bool {
	body, err := getMetadata(ctx,
		"http://169.254.169.254/metadata/instance/compute/vmId?api-version=2021-02-01&format=text",
		map[string]string{"Metadata": "true"},
	)
	if err != nil {
		return false
	}
	body = strings.TrimSpace(body)
	return len(body) == 36 && strings.Count(body, "-") == 4
}

func getMetadata(ctx context.Context, url string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
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
	return string(body), nil
}

func headMetadata(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp, nil
}
