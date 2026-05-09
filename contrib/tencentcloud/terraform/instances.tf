resource "tencentcloud_instance" "k8s_master" {
  running_flag= true
  stopped_mode = "STOP_CHARGING"
  availability_zone = "ap-seoul-1"
  instance_name        = "k8s_master"
  vpc_id = tencentcloud_vpc.test-vpc.id
  subnet_id = tencentcloud_subnet.test-subnet.id
  #instance_type = "SA5.MEDIUM2"  # 2C2G
  instance_type = "SA5.MEDIUM4" #2C4G
  key_ids = [tencentcloud_key_pair.cilium_test.id]
  instance_charge_type = "POSTPAID_BY_HOUR"
  image_id             = var.rocky95img
  system_disk_size     = 50
  private_ip = "10.202.11.101"
  orderly_security_groups = [tencentcloud_security_group.test-sg.id]
  depends_on = [tencentcloud_subnet.pod-subnet]
}


resource "tencentcloud_eip_association" "k8s_master" {
  eip_id               = tencentcloud_eip.k8s_master_eip.id
  instance_id = tencentcloud_instance.k8s_master.id
}


resource "tencentcloud_instance" "k8s_node01" {
  running_flag= true

  stopped_mode = "STOP_CHARGING"
  availability_zone = "ap-seoul-1"
  instance_name        = "k8s_node01"
  vpc_id = tencentcloud_vpc.test-vpc.id
  subnet_id = tencentcloud_subnet.test-subnet.id
  #instance_type = "SA5.MEDIUM2"  # 2C2G
  instance_type = "SA5.MEDIUM4" #2C4G
  key_ids = [tencentcloud_key_pair.cilium_test.id]
  instance_charge_type = "POSTPAID_BY_HOUR"
  image_id             = var.rocky95img
  system_disk_size     = 50
  private_ip = "10.202.11.102"
  orderly_security_groups = [tencentcloud_security_group.test-sg.id]
  depends_on = [tencentcloud_subnet.pod-subnet]
}



resource "tencentcloud_instance" "out_of_k8s_node01" {
  running_flag= true

  stopped_mode = "STOP_CHARGING"
  availability_zone = "ap-seoul-1"
  instance_name        = "out_of_k8s_node01"
  vpc_id = tencentcloud_vpc.test-vpc.id
  subnet_id = tencentcloud_subnet.out-of-k8s-subnet.id
  instance_type = "SA5.MEDIUM2"  # 2C2G
  key_ids = [tencentcloud_key_pair.cilium_test.id]
  instance_charge_type = "POSTPAID_BY_HOUR"
  image_id             = var.rocky95img
  system_disk_size     = 50
  private_ip = "10.202.13.101"
  orderly_security_groups = [tencentcloud_security_group.test-sg.id]

}







resource "tencentcloud_eip_association" "k8s_node01" {
  eip_id               = tencentcloud_eip.k8s_node01.id
  instance_id = tencentcloud_instance.k8s_node01.id
}


data "local_file" "public_key" {
  filename = "./my_key.pub"
}


resource "tencentcloud_key_pair" "cilium_test" {
  key_name   = "cilium_test"
  public_key = data.local_file.public_key.content
  tags = {
    Name        = "cilium_test"
  }
}

resource "tencentcloud_eip_association" "out_of_k8s_node01_eip" {
  eip_id               = tencentcloud_eip.out_of_k8s_node01_eip.id
  instance_id = tencentcloud_instance.out_of_k8s_node01.id
}