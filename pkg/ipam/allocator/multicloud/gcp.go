// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package multicloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2/google"

	ipamTypes "github.com/cilium/cilium/pkg/ipam/types"
)

const gcpComputeScope = "https://www.googleapis.com/auth/compute"

type gcpClient struct {
	project string
	zone    string
	httpCli *http.Client
}

// newGCPClient builds a GCP client from GOOGLE_CREDENTIALS_JSON (service account JSON).
// Zone defaults to <region>-a; override with GCP_ZONE env var.
func newGCPClient(key ClientKey) (CloudAPI, error) {
	credJSON := os.Getenv("GOOGLE_CREDENTIALS_JSON")
	if credJSON == "" {
		return nil, fmt.Errorf("gcp: GOOGLE_CREDENTIALS_JSON not set")
	}

	var meta struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(credJSON), &meta); err != nil {
		return nil, fmt.Errorf("gcp: parse project_id: %w", err)
	}

	conf, err := google.JWTConfigFromJSON([]byte(credJSON), gcpComputeScope)
	if err != nil {
		return nil, fmt.Errorf("gcp: parse credentials: %w", err)
	}

	zone := os.Getenv("GCP_ZONE")
	if zone == "" {
		zone = key.Region + "-a"
	}

	// conf.Client returns an *http.Client that automatically fetches and refreshes OAuth2 tokens.
	httpCli := conf.Client(context.Background())
	httpCli.Timeout = 30 * time.Second

	return &gcpClient{
		project: meta.ProjectID,
		zone:    zone,
		httpCli: httpCli,
	}, nil
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

const gcpComputeBase = "https://compute.googleapis.com/compute/v1"

func (c *gcpClient) doGet(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gcpComputeBase+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gcp: GET %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ── GCP Compute API types ─────────────────────────────────────────────────────

type gcpInstanceListResp struct {
	Items []gcpInstance `json:"items"`
}

type gcpInstance struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	NetworkInterfaces []gcpNIC `json:"networkInterfaces"`
}

type gcpNIC struct {
	Name          string     `json:"name"`
	NetworkIP     string     `json:"networkIP"`
	Subnetwork    string     `json:"subnetwork"`
	Fingerprint   string     `json:"fingerprint"`
	AliasIpRanges []gcpAlias `json:"aliasIpRanges"`
}

type gcpAlias struct {
	IPCidrRange string `json:"ipCidrRange"`
}

type gcpOperation struct {
	Name   string      `json:"name"`
	Status string      `json:"status"`
	Error  *gcpOpError `json:"error"`
}

type gcpOpError struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ── NIC selection ─────────────────────────────────────────────────────────────

// podNICs returns all NICs available for pod IP allocation.
// Non-nic0 NICs (nic1, nic2, …) are preferred; nic0 is the fallback.
func podNICs(nics []gcpNIC) []gcpNIC {
	var secondary []gcpNIC
	var nic0 *gcpNIC
	for i := range nics {
		if nics[i].Name == "nic0" {
			nic0 = &nics[i]
		} else {
			secondary = append(secondary, nics[i])
		}
	}
	if len(secondary) > 0 {
		return secondary
	}
	if nic0 != nil {
		return []gcpNIC{*nic0}
	}
	return nil
}

// ── ENI ID encoding ───────────────────────────────────────────────────────────

// ENI ID format: "<numericInstanceID>:<nicName>", e.g. "3256678776495443166:nic1"
func gcpMakeENIID(instanceID, nicName string) string { return instanceID + ":" + nicName }

func gcpParseENIID(id string) (instanceID, nicName string, err error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("gcp: malformed ENI ID %q", id)
	}
	return parts[0], parts[1], nil
}

func gcpBuildENI(instanceID string, nic *gcpNIC) *ENI {
	ips := []PrivateIP{{PrivateIpAddress: nic.NetworkIP, Primary: true}}
	for _, alias := range nic.AliasIpRanges {
		ip := strings.TrimSuffix(alias.IPCidrRange, "/32")
		if !strings.Contains(ip, "/") {
			ips = append(ips, PrivateIP{PrivateIpAddress: ip})
		}
	}
	return &ENI{
		NetworkInterfaceID: gcpMakeENIID(instanceID, nic.Name),
		PrivateIPAddresses: ips,
	}
}

// ── CloudAPI implementation ───────────────────────────────────────────────────

func (c *gcpClient) GetInstances(ctx context.Context) (*ipamTypes.InstanceMap, error) {
	var resp gcpInstanceListResp
	if err := c.doGet(ctx, fmt.Sprintf("/projects/%s/zones/%s/instances", c.project, c.zone), &resp); err != nil {
		return nil, err
	}
	result := ipamTypes.NewInstanceMap()
	for _, inst := range resp.Items {
		for _, nic := range podNICs(inst.NetworkInterfaces) {
			nic := nic
			result.Update(inst.ID, ipamTypes.InterfaceRevision{Resource: gcpBuildENI(inst.ID, &nic)})
		}
	}
	return result, nil
}

func (c *gcpClient) GetInstance(ctx context.Context, instanceID string) (*ipamTypes.Instance, error) {
	var inst gcpInstance
	if err := c.doGet(ctx,
		fmt.Sprintf("/projects/%s/zones/%s/instances/%s", c.project, c.zone, instanceID),
		&inst); err != nil {
		return nil, err
	}
	nics := podNICs(inst.NetworkInterfaces)
	if len(nics) == 0 {
		return nil, fmt.Errorf("gcp: no usable NIC on instance %s", instanceID)
	}
	ifaces := make(map[string]ipamTypes.InterfaceRevision, len(nics))
	for _, nic := range nics {
		nic := nic
		id := gcpMakeENIID(inst.ID, nic.Name)
		ifaces[id] = ipamTypes.InterfaceRevision{Resource: gcpBuildENI(inst.ID, &nic)}
	}
	return &ipamTypes.Instance{Interfaces: ifaces}, nil
}

// GetSecurityGroups returns empty: GCP uses VPC-level firewall rules.
func (c *gcpClient) GetSecurityGroups(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

// CreateNetworkInterface is not supported on GCP — NICs must be pre-created in Terraform.
func (c *gcpClient) CreateNetworkInterface(_ context.Context, _ int, _, _ string, _ []string) (string, *ENI, error) {
	return "", nil, fmt.Errorf("gcp: dynamic NIC creation is not supported; pre-create NICs in Terraform")
}

func (c *gcpClient) WaitENIAvailable(_ context.Context, _ string) error              { return nil }
func (c *gcpClient) AttachNetworkInterface(_ context.Context, _, _ string) error     { return nil }
func (c *gcpClient) WaitENIAttached(_ context.Context, _ string) error               { return nil }

func (c *gcpClient) AssignPrivateIpAddresses(ctx context.Context, eid string, count int) ([]string, error) {
	instanceID, nicName, err := gcpParseENIID(eid)
	if err != nil {
		return nil, err
	}
	_, nic, err := c.fetchInstance(ctx, instanceID, nicName)
	if err != nil {
		return nil, err
	}

	usedOnNIC := map[string]bool{nic.NetworkIP: true}
	for _, alias := range nic.AliasIpRanges {
		usedOnNIC[strings.TrimSuffix(alias.IPCidrRange, "/32")] = true
	}

	subnetCIDR, err := c.getSubnetCIDR(ctx, nic.Subnetwork)
	if err != nil {
		return nil, err
	}
	newIPs, err := c.findFreeIPs(ctx, subnetCIDR, count, usedOnNIC)
	if err != nil {
		return nil, err
	}

	aliases := make([]gcpAlias, len(nic.AliasIpRanges), len(nic.AliasIpRanges)+len(newIPs))
	copy(aliases, nic.AliasIpRanges)
	for _, ip := range newIPs {
		aliases = append(aliases, gcpAlias{IPCidrRange: ip + "/32"})
	}
	if err := c.patchAliasIPs(ctx, instanceID, nicName, nic.Fingerprint, aliases); err != nil {
		return nil, err
	}
	return newIPs, nil
}

func (c *gcpClient) UnassignPrivateIpAddresses(ctx context.Context, eid string, ips []string) error {
	instanceID, nicName, err := gcpParseENIID(eid)
	if err != nil {
		return err
	}
	_, nic, err := c.fetchInstance(ctx, instanceID, nicName)
	if err != nil {
		return err
	}

	removeSet := make(map[string]bool, len(ips)*2)
	for _, ip := range ips {
		removeSet[ip] = true
		removeSet[ip+"/32"] = true
	}
	kept := make([]gcpAlias, 0, len(nic.AliasIpRanges))
	for _, alias := range nic.AliasIpRanges {
		if !removeSet[alias.IPCidrRange] && !removeSet[strings.TrimSuffix(alias.IPCidrRange, "/32")] {
			kept = append(kept, alias)
		}
	}
	return c.patchAliasIPs(ctx, instanceID, nicName, nic.Fingerprint, kept)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (c *gcpClient) fetchInstance(ctx context.Context, instanceID, nicName string) (*gcpInstance, *gcpNIC, error) {
	var inst gcpInstance
	if err := c.doGet(ctx,
		fmt.Sprintf("/projects/%s/zones/%s/instances/%s", c.project, c.zone, instanceID),
		&inst); err != nil {
		return nil, nil, err
	}
	for i := range inst.NetworkInterfaces {
		if inst.NetworkInterfaces[i].Name == nicName {
			return &inst, &inst.NetworkInterfaces[i], nil
		}
	}
	return nil, nil, fmt.Errorf("gcp: NIC %q not found on instance %s", nicName, instanceID)
}

func (c *gcpClient) getSubnetCIDR(ctx context.Context, subnetRef string) (string, error) {
	var path string
	if idx := strings.Index(subnetRef, "/projects/"); idx >= 0 {
		path = subnetRef[idx:]
	} else if strings.HasPrefix(subnetRef, "projects/") {
		path = "/" + subnetRef
	} else {
		return "", fmt.Errorf("gcp: unexpected subnetwork reference: %s", subnetRef)
	}
	var sub struct {
		IPCidrRange string `json:"ipCidrRange"`
	}
	if err := c.doGet(ctx, path, &sub); err != nil {
		return "", fmt.Errorf("gcp: get subnet CIDR: %w", err)
	}
	return sub.IPCidrRange, nil
}

func (c *gcpClient) findFreeIPs(ctx context.Context, cidr string, count int, alreadyUsed map[string]bool) ([]string, error) {
	var resp gcpInstanceListResp
	if err := c.doGet(ctx, fmt.Sprintf("/projects/%s/zones/%s/instances", c.project, c.zone), &resp); err != nil {
		return nil, err
	}
	exclude := make(map[string]bool, len(alreadyUsed))
	for k := range alreadyUsed {
		exclude[k] = true
	}
	for _, inst := range resp.Items {
		for _, nic := range inst.NetworkInterfaces {
			exclude[nic.NetworkIP] = true
			for _, alias := range nic.AliasIpRanges {
				exclude[strings.TrimSuffix(alias.IPCidrRange, "/32")] = true
			}
		}
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("gcp: parse CIDR %s: %w", cidr, err)
	}
	base := network.IP.To4()
	if base == nil {
		return nil, fmt.Errorf("gcp: only IPv4 subnets supported")
	}

	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = base[i] | ^network.Mask[i]
	}

	// GCP-reserved: network, gateway (.1), second-to-last, broadcast
	exclude[base.String()] = true
	exclude[ip4Inc(base).String()] = true
	exclude[ip4Dec(broadcast).String()] = true
	exclude[broadcast.String()] = true

	var result []string
	for cur := ip4Inc(ip4Inc(base)); !cur.Equal(broadcast); cur = ip4Inc(cur) {
		if !exclude[cur.String()] {
			result = append(result, cur.String())
			if len(result) == count {
				return result, nil
			}
		}
	}
	return nil, fmt.Errorf("gcp: not enough free IPs in %s (need %d, found %d)", cidr, count, len(result))
}

func ip4Inc(ip net.IP) net.IP {
	next := make(net.IP, 4)
	copy(next, ip.To4())
	for i := 3; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func ip4Dec(ip net.IP) net.IP {
	prev := make(net.IP, 4)
	copy(prev, ip.To4())
	for i := 3; i >= 0; i-- {
		if prev[i] > 0 {
			prev[i]--
			break
		}
		prev[i] = 255
	}
	return prev
}

func (c *gcpClient) patchAliasIPs(ctx context.Context, instanceID, nicName, fingerprint string, aliases []gcpAlias) error {
	if aliases == nil {
		aliases = []gcpAlias{}
	}
	body, err := json.Marshal(map[string]interface{}{
		"fingerprint":   fingerprint,
		"aliasIpRanges": aliases,
	})
	if err != nil {
		return err
	}
	apiURL := fmt.Sprintf(
		"%s/projects/%s/zones/%s/instances/%s/updateNetworkInterface?networkInterface=%s",
		gcpComputeBase, c.project, c.zone, instanceID, nicName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("gcp: updateNetworkInterface: %w", err)
	}
	defer resp.Body.Close()

	var op gcpOperation
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return fmt.Errorf("gcp: decode operation response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := ""
		if op.Error != nil && len(op.Error.Errors) > 0 {
			msg = op.Error.Errors[0].Message
		}
		return fmt.Errorf("gcp: updateNetworkInterface status %d: %s", resp.StatusCode, msg)
	}
	return c.waitOperation(ctx, op.Name)
}

func (c *gcpClient) waitOperation(ctx context.Context, opName string) error {
	for {
		var op gcpOperation
		if err := c.doGet(ctx,
			fmt.Sprintf("/projects/%s/zones/%s/operations/%s", c.project, c.zone, opName),
			&op); err != nil {
			return fmt.Errorf("gcp: poll operation %s: %w", opName, err)
		}
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("gcp: operation failed: %s", op.Error.Errors[0].Message)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
