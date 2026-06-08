# ===== AWS IAM =====

resource "aws_iam_user" "cilium_eni" {
  name = "cilium-multicloud-eni"
}

resource "aws_iam_user_policy" "cilium_eni" {
  name = "cilium-multicloud-eni"
  user = aws_iam_user.cilium_eni.name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ec2:CreateNetworkInterface",
        "ec2:DeleteNetworkInterface",
        "ec2:DescribeNetworkInterfaces",
        "ec2:AttachNetworkInterface",
        "ec2:DetachNetworkInterface",
        "ec2:ModifyNetworkInterfaceAttribute",
        "ec2:AssignPrivateIpAddresses",
        "ec2:UnassignPrivateIpAddresses",
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeInstances",
      ]
      Resource = "*"
    }]
  })
}

resource "aws_iam_access_key" "cilium_eni" {
  user = aws_iam_user.cilium_eni.name
}

# ===== Alibaba Cloud RAM =====

resource "alicloud_ram_user" "cilium_eni" {
  name = "cilium-multicloud-eni"
}

resource "alicloud_ram_policy" "cilium_eni" {
  policy_name     = "cilium-multicloud-eni"
  policy_document = jsonencode({
    Version = "1"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ecs:CreateNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:DescribeNetworkInterfaces",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:AssignPrivateIpAddresses",
        "ecs:UnassignPrivateIpAddresses",
        "ecs:DescribeInstances",
        "vpc:DescribeVpcs",
        "vpc:DescribeVSwitches",
        "vpc:DescribeSecurityGroups",
      ]
      Resource = ["*"]
    }]
  })
}

resource "alicloud_ram_user_policy_attachment" "cilium_eni" {
  policy_name = alicloud_ram_policy.cilium_eni.policy_name
  policy_type = alicloud_ram_policy.cilium_eni.type
  user_name   = alicloud_ram_user.cilium_eni.name
}

resource "alicloud_ram_access_key" "cilium_eni" {
  user_name = alicloud_ram_user.cilium_eni.name
}

# ===== Tencent Cloud CAM =====

resource "tencentcloud_cam_policy" "cilium_eni" {
  name     = "cilium-multicloud-eni"
  document = jsonencode({
    version = "2.0"
    statement = [{
      effect = "allow"
      action = [
        "vpc:CreateNetworkInterface",
        "vpc:DeleteNetworkInterface",
        "vpc:AttachNetworkInterface",
        "vpc:DetachNetworkInterface",
        "vpc:DescribeNetworkInterfaces",
        "vpc:AssignPrivateIpAddresses",
        "vpc:UnassignPrivateIpAddresses",
        "vpc:DescribeSecurityGroups",
        "vpc:DescribeSecurityGroupPolicies",
        "cvm:DescribeInstances",
      ]
      resource = ["*"]
    }]
  })
}

resource "tencentcloud_cam_user" "cilium_eni" {
  name          = "cilium-multicloud-eni"
  console_login = false
  use_api       = true

  provisioner "local-exec" {
    when    = destroy
    command = <<-EOT
      for KEY_ID in $(tccli cam ListAccessKeys --TargetUin ${self.uin} --output json | \
        python3 -c "import sys,json; [print(k['AccessKeyId']) for k in json.load(sys.stdin).get('AccessKeys',[])]"); do
        echo "Deleting access key $KEY_ID for UIN ${self.uin}"
        tccli cam DeleteAccessKey --AccessKeyId "$KEY_ID" --TargetUin ${self.uin}
      done
    EOT
  }
}

resource "tencentcloud_cam_user_policy_attachment" "cilium_eni" {
  user_name = tencentcloud_cam_user.cilium_eni.name
  policy_id = tencentcloud_cam_policy.cilium_eni.id
}

resource "tencentcloud_cam_access_key" "cilium_eni" {
  target_uin = tencentcloud_cam_user.cilium_eni.uin
}
