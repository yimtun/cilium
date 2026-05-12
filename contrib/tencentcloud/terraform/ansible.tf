resource "local_file" "ansible_hosts" {
  filename = "${path.module}/../ansible/hosts.ini"
  content  = templatefile("${path.module}/hosts.ini.tpl", {
    k8s_master_eip = tencentcloud_eip.k8s_master_eip.public_ip
    k8s_node01_eip   = tencentcloud_eip.k8s_node01.public_ip
  })
}

resource "local_file" "operator_secret" {
  filename = "${path.module}/../ansible/files/08-operator-secret.yaml"
  content  = templatefile("${path.module}/operator-secret.yaml.tpl", {
    secret_id  = tencentcloud_cam_access_key.cilium_operator.access_key
    secret_key = tencentcloud_cam_access_key.cilium_operator.secret_access_key
  })
}

resource "local_file" "cilium_operator_deploy" {
  filename = "${path.module}/../ansible/files/09-cilium-operator.yaml"
  content  = templatefile("${path.module}/cilium-operator.yaml.tpl", {
    region    = "ap-seoul"
    vpc_id    = tencentcloud_vpc.test-vpc.id
    subnet_id = tencentcloud_subnet.pod-subnet.id
    pod_security_group_id = tencentcloud_security_group.test-sg.id
  })
}