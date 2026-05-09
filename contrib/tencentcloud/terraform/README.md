**English** | [中文](./README.zh-CN.md)

# Terraform: TencentCloud lab for Cilium ENI

Provisions a minimal TencentCloud lab to develop and test the Cilium TencentCloud ENI integration.

## Install `tccli`

```bash
# Install via pip (Python 3.6+)
pip3 install tccli
```

## Setup

1. **Generate an SSH key** (used for CVM login by Ansible):
   ```bash
   ssh-keygen -t rsa -b 2048 -f my_key -N ""
   ```

2. **Export TencentCloud credentials**:
   ```bash
   export TENCENTCLOUD_SECRET_ID="..."
   export TENCENTCLOUD_SECRET_KEY="..."
   ```

3. **Set required variables** (see `variables.tf`):
   ```bash
   export TF_VAR_rocky95img="img-xxxxxxxx"   # Rocky Linux 9.5 image ID in ap-seoul
   export TF_VAR_ifOn="true"                 # placeholder toggle (kept for legacy)
   ```

4. **Apply**:
   ```bash
   terraform init
   terraform apply
   ```

## Tear down

```bash
terraform destroy
```
