// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cilium/cilium/pkg/safeio"
)

const (
	awsBase     = "http://169.254.169.254/latest/meta-data"
	awsTokenURL = "http://169.254.169.254/latest/api/token"
	awsTokenTTL = "21600"
)

// awsInstanceIDRe matches AWS instance IDs: i- followed by 8 or 17 lowercase hex chars
var awsInstanceIDRe = regexp.MustCompile(`^i-[0-9a-f]{8}([0-9a-f]{9})?$`)

type awsProvider struct{}

func (a *awsProvider) providerName() string { return CloudProviderAWS }

// detect uses a lightweight HEAD request and checks the Server header (IMDSv1),
// consistent with the cursor01apiserver detection approach.
func (a *awsProvider) detect(ctx context.Context) bool {
	resp, err := httpHead(ctx, awsBase+"/instance-id")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(resp.Header.Get("Server")), "ec2ws")
}

// getIMDSv2Token fetches an IMDSv2 session token. Falls back gracefully if the
// instance only supports IMDSv1 (token request fails).
func getIMDSv2Token(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, awsTokenURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", awsTokenTTL)
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := safeio.ReadAllLimit(resp.Body, safeio.MB)
	if err != nil {
		return ""
	}
	return string(body)
}

// awsGet fetches a metadata URL. Uses IMDSv2 token if available, falls back to IMDSv1.
func awsGet(ctx context.Context, url, token string) (string, error) {
	var headers map[string]string
	if token != "" {
		headers = map[string]string{"X-aws-ec2-metadata-token": token}
	}
	return httpGet(ctx, url, headers)
}

func (a *awsProvider) getInstanceMetadata(ctx context.Context) (InstanceMetadata, error) {
	// Fetch token once; empty string means IMDSv1 fallback.
	token := getIMDSv2Token(ctx)

	instanceID, err := awsGet(ctx, awsBase+"/instance-id", token)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("instance-id: %w", err)
	}
	if !awsInstanceIDRe.MatchString(instanceID) {
		return InstanceMetadata{}, fmt.Errorf("unexpected instance-id format: %s", instanceID)
	}
	region, err := awsGet(ctx, awsBase+"/placement/region", token)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("region: %w", err)
	}
	mac, err := awsGet(ctx, awsBase+"/mac", token)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("mac: %w", err)
	}
	vpcID, err := awsGet(ctx, fmt.Sprintf("%s/network/interfaces/macs/%s/vpc-id", awsBase, mac), token)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("vpc-id: %w", err)
	}
	subnetID, err := awsGet(ctx, fmt.Sprintf("%s/network/interfaces/macs/%s/subnet-id", awsBase, mac), token)
	if err != nil {
		return InstanceMetadata{}, fmt.Errorf("subnet-id: %w", err)
	}
	return InstanceMetadata{
		InstanceID: instanceID,
		Region:     region,
		VPCID:      vpcID,
		SubnetID:   subnetID,
	}, nil
}
