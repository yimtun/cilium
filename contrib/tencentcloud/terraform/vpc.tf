


resource "tencentcloud_security_group" "test-sg" {
  name        = "test"
  description = "test"
}


resource "tencentcloud_security_group_rule_set" "test" {
  security_group_id = tencentcloud_security_group.test-sg.id
  ingress {
    protocol           = "TCP"
    port               = "22"
    action             = "ACCEPT"
    cidr_block         = "0.0.0.0/0"

  }


  ingress {
    protocol           = "TCP"
    port               = "80"
    action             = "ACCEPT"
    cidr_block         = "0.0.0.0/0"

  }

  ingress {
    protocol           = "ALL"
    action             = "ACCEPT"
    cidr_block         = "10.202.0.0/16"
  }

  egress {
    action = "ACCEPT"
    cidr_block = "0.0.0.0/0"
  }
  depends_on = [tencentcloud_security_group.test-sg]
}


resource "tencentcloud_vpc" "test-vpc" {
  name       = "test-vpc"
  cidr_block = "10.202.0.0/16"
  is_multicast       = false
}

resource "tencentcloud_subnet" "test-subnet" {
  name              = "test-subnet"
  cidr_block        = "10.202.11.0/24"
  availability_zone = "ap-seoul-1"
  vpc_id            = tencentcloud_vpc.test-vpc.id
  route_table_id    = tencentcloud_route_table.test-rt.id
  is_multicast       = false
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


resource "tencentcloud_subnet" "pod-subnet" {
  name              = "pod-subnet"
  cidr_block        = "10.202.12.0/24"
  availability_zone = "ap-seoul-1"
  vpc_id            = tencentcloud_vpc.test-vpc.id
  route_table_id    = tencentcloud_route_table.test-rt.id
  is_multicast       = false
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


resource "tencentcloud_subnet" "out-of-k8s-subnet" {
  name              = "out-of-k8s-subnet"
  cidr_block        = "10.202.13.0/24"
  availability_zone = "ap-seoul-1"
  vpc_id            = tencentcloud_vpc.test-vpc.id
  route_table_id    = tencentcloud_route_table.test-rt.id
  is_multicast       = false
}




resource "tencentcloud_eip"  "k8s_master_eip" {
  name = "eip-1"
  internet_charge_type = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 50
  internet_service_provider = "BGP"
  type = "EIP"
}

resource "tencentcloud_eip"  "k8s_node01" {
  name = "k8s_node01"
  internet_charge_type = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 50
  internet_service_provider = "BGP"
  type = "EIP"
}






resource "tencentcloud_eip"  "nat-eip" {
  name = "nat-eip"
  internet_charge_type = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 1
  internet_service_provider = "BGP"
  type = "EIP"
}



resource "tencentcloud_nat_gateway" "cilium-nat" {
  tags = {
    "env-usage" = "cilium-nat"
  }
  name             = "cilium-nat"
  vpc_id           = tencentcloud_vpc.test-vpc.id
  bandwidth        = 20
  max_concurrent   = 1000000
  assigned_eip_set = [
    tencentcloud_eip.nat-eip.public_ip
  ]
}


resource "tencentcloud_route_table" "test-rt" {
  vpc_id = tencentcloud_vpc.test-vpc.id
  name   = "test-rt"
}

resource "tencentcloud_route_table_entry" "eip2first" {
  route_table_id         = tencentcloud_route_table.test-rt.id
  destination_cidr_block = "0.0.0.0/0"
  next_type              = "EIP"
  next_hub               = "0"
  description            = "eip2first"
  disabled = false
}



resource "tencentcloud_route_table_entry" "default2nat" {
  route_table_id         = tencentcloud_route_table.test-rt.id
  destination_cidr_block = "0.0.0.0/0"
  next_type              = "NAT"
  next_hub               = tencentcloud_nat_gateway.cilium-nat.id
  description            = "default2nat"
  disabled = false
}


resource "tencentcloud_eip"  "out_of_k8s_node01_eip" {
  name = "out_of_k8s_node01_eip"
  internet_charge_type = "TRAFFIC_POSTPAID_BY_HOUR"
  internet_max_bandwidth_out = 1
  internet_service_provider = "BGP"
  type = "EIP"
}