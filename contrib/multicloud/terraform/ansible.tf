resource "local_file" "ansible_hosts" {
  filename = "${path.module}/../ansible/hosts.ini"
  content  = templatefile("${path.module}/hosts.ini.tpl", {
    k8s_tc_eip  = tencentcloud_eip.tc.public_ip
    k8s_ali_eip = alicloud_eip_address.ali.ip_address
    k8s_aws_eip = aws_eip.aws.public_ip
    wg_tc_eip   = tencentcloud_eip.tc-wg.public_ip
    wg_ali_eip  = alicloud_eip_address.ali-wg.ip_address
    wg_aws_eip  = aws_eip.aws-wg.public_ip
  })
}

resource "local_sensitive_file" "cloud_credentials_yaml" {
  filename = "${path.module}/../ansible/files/cloud-credentials.yaml"
  content  = templatefile("${path.module}/cloud-credentials.tpl", {
    aws_access_key_id               = aws_iam_access_key.cilium_eni.id
    aws_secret_access_key           = aws_iam_access_key.cilium_eni.secret
    alibaba_cloud_access_key_id     = alicloud_ram_access_key.cilium_eni.id
    alibaba_cloud_access_key_secret = alicloud_ram_access_key.cilium_eni.secret
    tencentcloud_secret_id          = tencentcloud_cam_access_key.cilium_eni.access_key
    tencentcloud_secret_key         = tencentcloud_cam_access_key.cilium_eni.secret_access_key
  })
}
