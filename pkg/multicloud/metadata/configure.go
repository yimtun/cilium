// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package metadata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// ConfigureSecondaryENIAddresses assigns the primary IP (with /32 mask) to
// each secondary ENI that has no IPv4 address yet. The primary ENI (eth0) is
// skipped because it is managed by NetworkManager/DHCP. /32 avoids creating a
// subnet route that conflicts with eth0's route.
func ConfigureSecondaryENIAddresses(ctx context.Context, logger *slog.Logger) error {
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("list links: %w", err)
	}

	needsConfigure := false
	for _, link := range links {
		if link.Attrs().Flags&net.FlagLoopback != 0 || link.Type() != "device" {
			continue
		}
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
		if len(addrs) == 0 {
			needsConfigure = true
			break
		}
	}
	if !needsConfigure {
		return nil
	}

	cloudProvider, err := DetectCloudProvider(ctx)
	if err != nil {
		return fmt.Errorf("detect cloud provider: %w", err)
	}

	primaryMAC, err := GetPrimaryMAC(ctx, cloudProvider)
	if err != nil {
		return fmt.Errorf("get primary MAC: %w", err)
	}
	primaryMAC = strings.TrimSpace(primaryMAC)

	var errs error
	for _, link := range links {
		if link.Attrs().Flags&net.FlagLoopback != 0 {
			continue
		}
		if link.Type() != "device" {
			continue
		}
		mac := link.Attrs().HardwareAddr.String()
		if strings.EqualFold(mac, primaryMAC) {
			continue
		}

		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("list addrs on %s: %w", link.Attrs().Name, err))
			continue
		}
		if len(addrs) > 0 {
			continue
		}

		primaryIP, err := GetInterfacePrimaryIP(ctx, cloudProvider, mac)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("primary IP for %s: %w", mac, err))
			continue
		}

		addr := &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   net.ParseIP(primaryIP).To4(),
				Mask: net.CIDRMask(32, 32),
			},
		}
		if err := netlink.AddrAdd(link, addr); err != nil && !errors.Is(err, unix.EEXIST) {
			errs = errors.Join(errs, fmt.Errorf("addr add %s on %s: %w", primaryIP, link.Attrs().Name, err))
			continue
		}
		logger.Info("multicloud: assigned primary IP to secondary ENI",
			"interface", link.Attrs().Name,
			"ipAddr", primaryIP,
		)
	}
	return errs
}
