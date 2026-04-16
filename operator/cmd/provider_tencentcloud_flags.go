// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

//go:build ipam_provider_tencentcloud

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	FlagsHooks = append(FlagsHooks, &tencentCloudFlagsHooks{})
}

type tencentCloudFlagsHooks struct{}

func (hook *tencentCloudFlagsHooks) RegisterProviderFlag(cmd *cobra.Command, vp *viper.Viper) {
	flags := cmd.Flags()
	vp.BindPFlags(flags)
}
