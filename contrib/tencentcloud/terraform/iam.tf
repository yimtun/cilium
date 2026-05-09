resource "tencentcloud_cam_policy" "cilium_operator" {
  name     = "cilium-operator-eni"
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
        "cvm:DescribeInstances",
        "vpc:DescribeSecurityGroups",
        "vpc:DescribeSecurityGroupPolicies"
      ]
      resource = ["*"]
    }]
  })
}

resource "tencentcloud_cam_user" "cilium_operator" {
  name          = "cilium-operator"
  console_login = false
  use_api       = true
  provisioner "local-exec" {
    when    = destroy
    command = <<-EOT
            for KEY_ID in $(tccli cam ListAccessKeys --TargetUin ${self.uin} --output json | python3 -c "import sys,json;
  [print(k['AccessKeyId']) for k in json.load(sys.stdin).get('AccessKeys',[])]"); do
          echo "Deleting access key $KEY_ID for UIN ${self.uin}"
          tccli cam DeleteAccessKey --AccessKeyId "$KEY_ID" --TargetUin ${self.uin}
        done
        sleep 5
      EOT
    }
}

# resource "tencentcloud_cam_user_policy_attachment" "cilium_operator" {
#   user_id   = tencentcloud_cam_user.cilium_operator.id
#   policy_id = tencentcloud_cam_policy.cilium_operator.id
# }
#
# resource "tencentcloud_cam_access_key" "cilium_operator" {
#   target_uin = tencentcloud_cam_user.cilium_operator.id
# }

resource "tencentcloud_cam_user_policy_attachment" "cilium_operator" {
  user_name = tencentcloud_cam_user.cilium_operator.name
  policy_id = tencentcloud_cam_policy.cilium_operator.id
}

resource "tencentcloud_cam_access_key" "cilium_operator" {
  target_uin = tencentcloud_cam_user.cilium_operator.uin
}



