output "eip-nat-gw" {
  value = tencentcloud_eip.nat-eip.public_ip
}
output "k8s_master_eip" {
  value = tencentcloud_eip.k8s_master_eip.public_ip
}

output "k8s_node01-eip" {
  value = tencentcloud_eip.k8s_node01.public_ip
}

output "out_of_k8s_node01_eip" {
  value = tencentcloud_eip.out_of_k8s_node01_eip.public_ip
}


