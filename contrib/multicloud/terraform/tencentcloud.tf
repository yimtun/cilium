data "local_file" "public_key" {
  filename = "./my_key.pub"
}

resource "tencentcloud_key_pair" "cilium_test" {
  key_name   = "cilium"
  public_key = data.local_file.public_key.content
}

resource "tencentcloud_vpc" "tc" {
  name       = "multicloud-test-tc"
  cidr_block = "10.202.0.0/16"
  is_multicast = false
}

resource "tencentcloud_subnet" "tc" {
  name              = "multicloud-test-tc"
  cidr_block        = "10.202.1.0/24"
  availability_zone = "${var.tc_region}-1"
  vpc_id            = tencentcloud_vpc.tc.id
  is_multicast      = false

  provisioner "local-exec" {
    when    = destroy
    command = <<-EOT
      SUBNET_ID="${self.id}"
      REGION="${replace(self.availability_zone, "/-[0-9]+$/", "")}"
      list_enis() {
        tccli vpc DescribeNetworkInterfaces --region "$REGION" \
          --Filters '[{"Name":"subnet-id","Values":["'"$SUBNET_ID"'"]}]' \
          --Limit 100 --output json 2>/dev/null | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    for e in d.get('NetworkInterfaceSet', []):
        if (e.get('NetworkInterfaceName') or '') != 'cilium-eni':
            continue
        att = (e.get('Attachment') or {}).get('InstanceId') or ''
        print(e['NetworkInterfaceId'], att)
except Exception: pass"
      }
      echo ">> cleaning cilium-eni in subnet $SUBNET_ID"
      for i in 1 2 3; do
        ENIS=$(list_enis)
        if [ -z "$ENIS" ]; then echo "   none left"; exit 0; fi
        echo "$ENIS" | while read ENI INST; do
          [ -z "$ENI" ] && continue
          if [ -n "$INST" ]; then
            echo "   detach $ENI from $INST"
            tccli vpc DetachNetworkInterface --region "$REGION" \
              --NetworkInterfaceId "$ENI" --InstanceId "$INST" 2>/dev/null || true
          fi
          echo "   delete $ENI"
          tccli vpc DeleteNetworkInterface --region "$REGION" \
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

resource "tencentcloud_security_group" "tc" {
  name = "multicloud-test-tc"
}

resource "tencentcloud_security_group_rule_set" "tc" {
  security_group_id = tencentcloud_security_group.tc.id
  ingress {
    protocol   = "TCP"
    port       = "22"
    action     = "ACCEPT"
    cidr_block = "0.0.0.0/0"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${aws_eip.aws.public_ip}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${alicloud_eip_address.ali.ip_address}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${aws_eip.aws-wg.public_ip}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${alicloud_eip_address.ali-wg.ip_address}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${google_compute_address.gcp.address}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${google_compute_address.gcp-wg.address}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "10.202.0.0/16"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "10.203.0.0/16"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "10.201.0.0/16"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "10.204.0.0/16"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${azurerm_public_ip.azure.ip_address}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "${azurerm_public_ip.azure-wg.ip_address}/32"
  }
  ingress {
    protocol   = "ALL"
    action     = "ACCEPT"
    cidr_block = "10.205.0.0/16"
  }
  egress {
    action     = "ACCEPT"
    cidr_block = "0.0.0.0/0"
  }
}

resource "tencentcloud_instance" "tc" {
  instance_name           = "multicloud-test-tc"
  availability_zone       = "${var.tc_region}-1"
  instance_type           = "SA4.LARGE8"
  image_id                = var.tc_image_id
  vpc_id                  = tencentcloud_vpc.tc.id
  subnet_id               = tencentcloud_subnet.tc.id
  key_ids                 = [tencentcloud_key_pair.cilium_test.id]
  instance_charge_type    = "POSTPAID_BY_HOUR"
  system_disk_size        = 50
  orderly_security_groups = [tencentcloud_security_group.tc.id]
  private_ip              = "10.202.1.101"
}

resource "tencentcloud_eip" "tc" {
  name                       = "multicloud-test-tc"
  internet_charge_type       = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 10
}

resource "tencentcloud_eip_association" "tc" {
  eip_id      = tencentcloud_eip.tc.id
  instance_id = tencentcloud_instance.tc.id
}

resource "tencentcloud_route_table" "tc" {
  vpc_id = tencentcloud_vpc.tc.id
  name   = "multicloud-test-tc"
}

resource "tencentcloud_route_table_association" "tc" {
  route_table_id = tencentcloud_route_table.tc.id
  subnet_id      = tencentcloud_subnet.tc.id
}

resource "tencentcloud_route_table_entry" "tc_to_aws" {
  route_table_id         = tencentcloud_route_table.tc.id
  destination_cidr_block = "10.201.0.0/16"
  next_type              = "NORMAL_CVM"
  next_hub               = tencentcloud_instance.tc-wg.private_ip
}

resource "tencentcloud_route_table_entry" "tc_to_ali" {
  route_table_id         = tencentcloud_route_table.tc.id
  destination_cidr_block = "10.203.0.0/16"
  next_type              = "NORMAL_CVM"
  next_hub               = tencentcloud_instance.tc-wg.private_ip
}

resource "tencentcloud_route_table_entry" "tc_to_gcp" {
  route_table_id         = tencentcloud_route_table.tc.id
  destination_cidr_block = "10.204.0.0/16"
  next_type              = "NORMAL_CVM"
  next_hub               = tencentcloud_instance.tc-wg.private_ip
}

resource "tencentcloud_route_table_entry" "tc_to_azure" {
  route_table_id         = tencentcloud_route_table.tc.id
  destination_cidr_block = "10.205.0.0/16"
  next_type              = "NORMAL_CVM"
  next_hub               = tencentcloud_instance.tc-wg.private_ip
}

