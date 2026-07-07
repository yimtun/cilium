resource "alicloud_key_pair" "cilium_test" {
  key_pair_name = "cilium-multicloud-test"
  public_key    = data.local_file.public_key.content
}

resource "alicloud_vpc" "ali" {
  vpc_name   = "multicloud-test-ali"
  cidr_block = "10.203.0.0/16"
}

resource "alicloud_vswitch" "ali" {
  vswitch_name = "multicloud-test-ali"
  cidr_block   = "10.203.1.0/24"
  vpc_id       = alicloud_vpc.ali.id
  zone_id      = "ap-northeast-2a"
  # brew install aliyun-cli macbook
  provisioner "local-exec" {
    when    = destroy
    command = <<-EOT
      VSWITCH_ID="${self.id}"
      REGION="${replace(self.zone_id, "/[a-z]$/", "")}"
      list_enis() {
        aliyun ecs DescribeNetworkInterfaces \
          --access-key-id "$ALICLOUD_ACCESS_KEY" \
          --access-key-secret "$ALICLOUD_SECRET_KEY" \
          --region "$REGION" \
          --VSwitchId "$VSWITCH_ID" \
          --Type Secondary \
          2>/dev/null | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    for e in d.get('NetworkInterfaceSets', {}).get('NetworkInterfaceSet', []):
        att = (e.get('Attachment') or {}).get('InstanceId') or ''
        print(e['NetworkInterfaceId'], att)
except Exception: pass"
      }
      echo ">> cleaning cilium ENI in vswitch $VSWITCH_ID"
      for i in 1 2 3; do
        ENIS=$(list_enis)
        if [ -z "$ENIS" ]; then echo "   none left"; exit 0; fi
        echo "$ENIS" | while read ENI INST; do
          [ -z "$ENI" ] && continue
          if [ -n "$INST" ]; then
            echo "   detach $ENI from $INST"
            aliyun ecs DetachNetworkInterface \
              --access-key-id "$ALICLOUD_ACCESS_KEY" \
              --access-key-secret "$ALICLOUD_SECRET_KEY" \
              --region "$REGION" \
              --NetworkInterfaceId "$ENI" \
              --InstanceId "$INST" 2>/dev/null || true
          fi
          echo "   delete $ENI"
          aliyun ecs DeleteNetworkInterface \
            --access-key-id "$ALICLOUD_ACCESS_KEY" \
            --access-key-secret "$ALICLOUD_SECRET_KEY" \
            --region "$REGION" \
            --NetworkInterfaceId "$ENI" 2>/dev/null || true
        done
        sleep 5
      done
      REMAIN=$(list_enis)
      if [ -n "$REMAIN" ]; then
        echo "   ERROR: still present:"; echo "$REMAIN"; exit 1
      fi
    EOT
  }
}

resource "alicloud_security_group" "ali" {
  security_group_name = "multicloud-test-ali"
  vpc_id = alicloud_vpc.ali.id
}

resource "alicloud_security_group_rule" "ssh" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "22/22"
  cidr_ip           = "0.0.0.0/0"
}


resource "alicloud_instance" "ali" {
  instance_name              = "multicloud-test-ali"
  image_id                   = var.ali_image_id
  instance_type              = "ecs.c9i.xlarge"
  instance_charge_type       = "PostPaid"
  vswitch_id                 = alicloud_vswitch.ali.id
  security_groups            = [alicloud_security_group.ali.id]
  key_name                   = alicloud_key_pair.cilium_test.key_pair_name
  system_disk_category       = "cloud_essd"
  system_disk_size           = 40
  internet_max_bandwidth_out = 0
  private_ip                 = "10.203.1.101"
}

resource "alicloud_eip_address" "ali" {
  bandwidth            = "10"
  internet_charge_type = "PayByTraffic"
}

resource "alicloud_eip_association" "ali" {
  allocation_id = alicloud_eip_address.ali.id
  instance_id   = alicloud_instance.ali.id
}

resource "alicloud_security_group_rule" "from_tc" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${tencentcloud_eip.tc.public_ip}/32"
}

resource "alicloud_security_group_rule" "from_aws" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${aws_eip.aws.public_ip}/32"
}


resource "alicloud_security_group_rule" "inner" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  port_range        = "-1/-1"
  ip_protocol       = "all"
  cidr_ip           = "10.203.0.0/16"
}

resource "alicloud_security_group_rule" "from_tc_wg" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${tencentcloud_eip.tc-wg.public_ip}/32"
}

resource "alicloud_security_group_rule" "from_aws_wg" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${aws_eip.aws-wg.public_ip}/32"
}

resource "alicloud_security_group_rule" "from_gcp" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${google_compute_address.gcp.address}/32"
}

resource "alicloud_security_group_rule" "from_gcp_wg" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${google_compute_address.gcp-wg.address}/32"
}

resource "alicloud_security_group_rule" "from_gcp_vpc" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "10.204.0.0/16"
}

resource "alicloud_security_group_rule" "from_tc_vpc" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "10.202.0.0/16"
}

resource "alicloud_security_group_rule" "from_aws_vpc" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "10.201.0.0/16"
}

resource "alicloud_security_group_rule" "from_azure" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${azurerm_public_ip.azure.ip_address}/32"
}

resource "alicloud_security_group_rule" "from_azure_wg" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "${azurerm_public_ip.azure-wg.ip_address}/32"
}

resource "alicloud_security_group_rule" "from_azure_vpc" {
  security_group_id = alicloud_security_group.ali.id
  type              = "ingress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "10.205.0.0/16"
}

resource "alicloud_route_entry" "ali_to_tc" {
  route_table_id        = alicloud_vpc.ali.route_table_id
  destination_cidrblock = "10.202.0.0/16"
  nexthop_type          = "Instance"
  nexthop_id            = alicloud_instance.ali-wg.id
}

resource "alicloud_route_entry" "ali_to_aws" {
  route_table_id        = alicloud_vpc.ali.route_table_id
  destination_cidrblock = "10.201.0.0/16"
  nexthop_type          = "Instance"
  nexthop_id            = alicloud_instance.ali-wg.id
}

resource "alicloud_route_entry" "ali_to_gcp" {
  route_table_id        = alicloud_vpc.ali.route_table_id
  destination_cidrblock = "10.204.0.0/16"
  nexthop_type          = "Instance"
  nexthop_id            = alicloud_instance.ali-wg.id
}

resource "alicloud_route_entry" "ali_to_azure" {
  route_table_id        = alicloud_vpc.ali.route_table_id
  destination_cidrblock = "10.205.0.0/16"
  nexthop_type          = "Instance"
  nexthop_id            = alicloud_instance.ali-wg.id
}

