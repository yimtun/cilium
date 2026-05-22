// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

//go:build ipam_provider_multicloud

package ipam

import (
	"fmt"
	"log/slog"

	"github.com/cilium/hive/cell"
	"github.com/cilium/hive/job"

	"github.com/cilium/cilium/pkg/ipam/allocator/multicloud"
	ipamMetrics "github.com/cilium/cilium/pkg/ipam/metrics"
	ipamOption "github.com/cilium/cilium/pkg/ipam/option"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/option"
)

func init() {
	allocators = append(allocators, cell.Module(
		"multicloud-ipam-allocator",
		"MultiCloud IP Allocator",

		cell.Invoke(startMultiCloudAllocator),
	))
}

type multiCloudParams struct {
	cell.In

	Logger             *slog.Logger
	Lifecycle          cell.Lifecycle
	JobGroup           job.Group
	Clientset          k8sClient.Clientset
	IPAMMetrics        *ipamMetrics.Metrics
	DaemonCfg          *option.DaemonConfig
	NodeWatcherFactory nodeWatcherJobFactory

	Cfg Config
}

func startMultiCloudAllocator(p multiCloudParams) {
	if p.DaemonCfg.IPAM != ipamOption.IPAMMultiCloud {
		return
	}

	alloc := &multicloud.AllocatorMultiCloud{
		ParallelAllocWorkers: p.Cfg.ParallelAllocWorkers,
	}

	p.Lifecycle.Append(
		cell.Hook{
			OnStart: func(ctx cell.HookContext) error {
				if err := alloc.Init(ctx, p.Logger); err != nil {
					return fmt.Errorf("unable to init MultiCloud allocator: %w", err)
				}

				nm, err := alloc.Start(ctx, &ciliumNodeUpdateImplementation{p.Clientset}, p.IPAMMetrics)
				if err != nil {
					return fmt.Errorf("unable to start MultiCloud allocator: %w", err)
				}

				p.JobGroup.Add(p.NodeWatcherFactory(nm))
				return nil
			},
		},
	)
}
