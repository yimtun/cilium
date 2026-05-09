**English** | [中文](./README.zh-CN.md)

# Cilium on TencentCloud (ENI mode)

Use TencentCloud VPC ENI as Cilium IPAM, allocating pod IPs directly from VPC subnets — same model as AWS ENI / Alibaba ENI.

## Architecture
- cilium-operator-tencentcloud: ENI lifecycle (attach/detach, IP alloc/release)
- cilium-agent: BPF datapath, policy routing on host side
- IPAM mode: `tencentcloud`

## Prerequisites
- TencentCloud account with CVM/VPC/CAM permissions
- Kubernetes 1.x+
- Linux kernel 5.x+ (for BPF features)

## Quick start
1. Provision infra: see [terraform/](./terraform/)
2. Deploy Cilium: see [ansible/](./ansible/)

## Limitations / Known issues
- KPR=false hybrid mode: ...
- Multi-ENI NodePort: ...