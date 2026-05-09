[English](./README.md) | **中文**

# Terraform：用于 Cilium ENI 的腾讯云实验环境

部署一套最小化的腾讯云实验环境，用于开发和测试 Cilium 的腾讯云 ENI 集成。

## 安装 `tccli`

```bash
# 通过 pip 安装（Python 3.6+）
pip3 install tccli
```

## 配置步骤

1. **生成 SSH 密钥**（Ansible 登录 CVM 用）：
   ```bash
   ssh-keygen -t rsa -b 2048 -f my_key -N ""
   ```

2. **导出腾讯云凭证**：
   ```bash
   export TENCENTCLOUD_SECRET_ID="..."
   export TENCENTCLOUD_SECRET_KEY="..."
   ```

3. **设置必需的变量**（见 `variables.tf`）：
   ```bash
   export TF_VAR_rocky95img="img-xxxxxxxx"   # ap-seoul 区域的 Rocky Linux 9.5 镜像 ID
   export TF_VAR_ifOn="true"                 # 占位开关（历史遗留）
   ```

4. **执行 apply**：
   ```bash
   terraform init
   terraform apply
   ```

## 清理环境

```bash
terraform destroy
```
