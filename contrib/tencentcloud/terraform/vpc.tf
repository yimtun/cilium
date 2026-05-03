


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
    port               = "6443"
    action             = "ACCEPT"
    cidr_block         = "0.0.0.0/0"
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


