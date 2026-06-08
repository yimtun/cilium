output "tc_public_ip" {
  value = tencentcloud_eip.tc.public_ip
}

output "aws_public_ip" {
  value = aws_eip.aws.public_ip
}

output "ali_public_ip" {
  value = alicloud_eip_address.ali.ip_address
}

# output "tc_security_group_id" {
#   value = tencentcloud_security_group.tc.id
# }
#
# output "aws_security_group_id" {
#   value = aws_security_group.aws.id
# }
#
# output "ali_security_group_id" {
#   value = alicloud_security_group.ali.id
# }
