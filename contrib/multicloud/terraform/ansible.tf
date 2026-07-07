resource "local_file" "ansible_hosts" {
  filename = "${path.module}/../ansible/hosts.ini"
  content  = templatefile("${path.module}/hosts.ini.tpl", {
    k8s_tc_eip    = tencentcloud_eip.tc.public_ip
    k8s_ali_eip   = alicloud_eip_address.ali.ip_address
    k8s_aws_eip   = aws_eip.aws.public_ip
    k8s_gcp_eip   = google_compute_address.gcp.address
    k8s_azure_eip = azurerm_public_ip.azure.ip_address
    wg_tc_eip     = tencentcloud_eip.tc-wg.public_ip
    wg_ali_eip    = alicloud_eip_address.ali-wg.ip_address
    wg_aws_eip    = aws_eip.aws-wg.public_ip
    wg_gcp_eip    = google_compute_address.gcp-wg.address
    wg_azure_eip  = azurerm_public_ip.azure-wg.ip_address
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
    google_credentials_json         = base64decode(google_service_account_key.cilium_eni.private_key)
    azure_credentials_json          = jsonencode({
      tenantId       = data.azurerm_client_config.current.tenant_id
      clientId       = azuread_application.cilium_eni.client_id
      clientSecret   = azuread_service_principal_password.cilium_eni.value
      subscriptionId = data.azurerm_client_config.current.subscription_id
    })
  })
}
