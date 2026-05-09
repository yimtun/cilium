[English](./README.md) | **中文**

# Ansible：部署 K8s 集群和 Cilium

使用 Terraform 生成的 inventory（`hosts.ini`），分三步部署 Kubernetes 集群和 Cilium。

## 部署步骤

1. **部署 K8s 集群**：
   ```bash
   ansible-playbook -i ./hosts.ini ./install_k8s.yaml
   ```

2. **部署 cilium-operator**：
   ```bash
   ansible-playbook -i ./hosts.ini ./install_cilium_operator.yaml
   ```

3. **部署 cilium-agent**：
   ```bash
   ansible-playbook -i ./hosts.ini ./install_cilium_agent.yaml
   ```