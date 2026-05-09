[English](./README.md) | **中文**

# 腾讯云上的 Cilium（ENI 模式）

使用腾讯云 VPC ENI 作为 Cilium 的 IPAM，pod IP 直接从 VPC 子网分配——与 AWS ENI / Alibaba ENI 同模式。

## 架构
- cilium-operator-tencentcloud：ENI 生命周期管理（挂载/解绑、IP 分配/释放）
- cilium-agent：BPF 数据面，宿主机侧策略路由
- IPAM 模式：`tencentcloud`

## 前置条件
- 拥有 CVM / VPC / CAM 权限的腾讯云账号
- Kubernetes 1.x+
- Linux 内核 5.x+（BPF 功能依赖）

## 快速开始
1. 创建基础设施：参见 [terraform/](./terraform/)
2. 部署 Cilium：参见 [ansible/](./ansible/)

## 限制 / 已知问题
- KPR=false 混合模式：...
- 多 ENI NodePort：...