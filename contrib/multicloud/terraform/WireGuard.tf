resource "alicloud_instance" "ali-wg" {
  instance_name              = "ali-wg"
  image_id                   = var.ali_image_id
  instance_type              = "ecs.e-c1m1.large"
  instance_charge_type       = "PostPaid"
  vswitch_id                 = alicloud_vswitch.ali.id
  security_groups            = [alicloud_security_group.ali.id]
  key_name                   = alicloud_key_pair.cilium_test.key_pair_name
  system_disk_category       = "cloud_essd_entry"
  system_disk_size           = 40
  internet_max_bandwidth_out = 0
  private_ip = "10.203.1.102"
}

resource "null_resource" "ali_wg_disable_src_dst_check" {
  triggers = {
    instance_id = alicloud_instance.ali-wg.id
  }

  depends_on = [alicloud_instance.ali-wg]

  provisioner "local-exec" {
    command = <<-EOT
      ENI_ID=$(aliyun ecs DescribeNetworkInterfaces \
        --access-key-id "$ALICLOUD_ACCESS_KEY" \
        --access-key-secret "$ALICLOUD_SECRET_KEY" \
        --region "${var.ali_region}" \
        --InstanceId "${alicloud_instance.ali-wg.id}" \
        | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['NetworkInterfaceSets']['NetworkInterfaceSet'][0]['NetworkInterfaceId'])")
      echo "ali-wg ENI: $ENI_ID -> disabling SourceDestCheck"
      aliyun ecs ModifyNetworkInterfaceAttribute \
        --access-key-id "$ALICLOUD_ACCESS_KEY" \
        --access-key-secret "$ALICLOUD_SECRET_KEY" \
        --region "${var.ali_region}" \
        --NetworkInterfaceId "$ENI_ID" \
        --SourceDestCheck false
    EOT
  }
}

resource "alicloud_eip_address" "ali-wg" {
  bandwidth            = "10"
  internet_charge_type = "PayByTraffic"
}

resource "alicloud_eip_association" "ali-wg" {
  allocation_id = alicloud_eip_address.ali-wg.id
  instance_id   = alicloud_instance.ali-wg.id
}

resource "aws_instance" "aws-wg" {
  ami                         = var.aws_ami_id
  instance_type               = "t3.small"
  subnet_id                   = aws_subnet.aws.id
  key_name                    = aws_key_pair.cilium_test.key_name
  vpc_security_group_ids      = [aws_security_group.aws.id]
  tags                        = { Name = "multicloud-test-aws" }
  private_ip                  = "10.201.1.102"
  source_dest_check           = false
}

resource "aws_eip" "aws-wg" {
  domain = "vpc"
}

resource "aws_eip_association" "aws-wg" {
  instance_id   = aws_instance.aws-wg.id
  allocation_id = aws_eip.aws-wg.id
}


resource "tencentcloud_instance" "tc-wg" {
  instance_name           = "tc-wg"
  availability_zone       = "${var.tc_region}-1"
  instance_type           = "SA5.MEDIUM2"
  image_id                = var.tc_image_id
  vpc_id                  = tencentcloud_vpc.tc.id
  subnet_id               = tencentcloud_subnet.tc.id
  key_ids                 = [tencentcloud_key_pair.cilium_test.id]
  instance_charge_type    = "POSTPAID_BY_HOUR"
  system_disk_size        = 50
  orderly_security_groups = [tencentcloud_security_group.tc.id]
  private_ip              = "10.202.1.102"
}

resource "tencentcloud_eip" "tc-wg" {
  name                       = "tc-wg"
  internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 10
}

resource "tencentcloud_eip_association" "tc-wg" {
  eip_id      = tencentcloud_eip.tc-wg.id
  instance_id = tencentcloud_instance.tc-wg.id
}