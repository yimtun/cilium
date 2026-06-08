resource "aws_key_pair" "cilium_test" {
  key_name   = "cilium-multicloud-test"
  public_key = data.local_file.public_key.content
}

resource "aws_vpc" "aws" {
  cidr_block = "10.201.0.0/16"
  tags       = { Name = "multicloud-test-aws" }
}

resource "aws_internet_gateway" "aws" {
  vpc_id = aws_vpc.aws.id
  tags   = { Name = "multicloud-test-aws" }
}

resource "aws_subnet" "aws" {
  vpc_id            = aws_vpc.aws.id
  cidr_block        = "10.201.1.0/24"
  availability_zone = "${var.aws_region}a"
  tags              = { Name = "multicloud-test-aws" }
  map_public_ip_on_launch = false

  provisioner "local-exec" {
    when    = destroy
    command = <<-EOT
      SUBNET_ID="${self.id}"
      REGION="${replace(self.availability_zone, "/[a-z]$/", "")}"
      list_enis() {
        aws ec2 describe-network-interfaces \
          --filters "Name=subnet-id,Values=$SUBNET_ID" \
                    "Name=description,Values=cilium-multicloud" \
          --region "$REGION" \
          --query 'NetworkInterfaces[*].NetworkInterfaceId' \
          --output text 2>/dev/null
      }
      echo ">> cleaning cilium-multicloud ENI in subnet $SUBNET_ID"
      for i in 1 2 3; do
        ENIS=$(list_enis)
        if [ -z "$ENIS" ]; then echo "   none left"; exit 0; fi
        for ENI in $ENIS; do
          echo "   delete $ENI"
          aws ec2 delete-network-interface --network-interface-id "$ENI" \
            --region "$REGION" 2>/dev/null || true
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

resource "aws_route_table" "aws" {
  vpc_id = aws_vpc.aws.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.aws.id
  }
  route {
    cidr_block           = "10.202.0.0/16"
    network_interface_id = aws_instance.aws-wg.primary_network_interface_id
  }
  route {
    cidr_block           = "10.203.0.0/16"
    network_interface_id = aws_instance.aws-wg.primary_network_interface_id
  }
  tags = { Name = "multicloud-test-aws" }
}

resource "aws_route_table_association" "aws" {
  subnet_id      = aws_subnet.aws.id
  route_table_id = aws_route_table.aws.id
}

resource "aws_security_group" "aws" {
  name   = "multicloud-test-aws"
  vpc_id = aws_vpc.aws.id
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["${tencentcloud_eip.tc.public_ip}/32"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["${alicloud_eip_address.ali.ip_address}/32"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["${tencentcloud_eip.tc-wg.public_ip}/32"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["${alicloud_eip_address.ali-wg.ip_address}/32"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.202.0.0/16"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.203.0.0/16"]
  }
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.201.0.0/16"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "aws" {
  ami                         = var.aws_ami_id
  instance_type               = "t3.large"
  subnet_id                   = aws_subnet.aws.id
  key_name                    = aws_key_pair.cilium_test.key_name
  vpc_security_group_ids      = [aws_security_group.aws.id]
  tags                        = { Name = "multicloud-test-aws" }
  private_ip                  = "10.201.1.101"
}

resource "aws_eip" "aws" {
  domain = "vpc"
}

resource "aws_eip_association" "aws" {
  instance_id   = aws_instance.aws.id
  allocation_id = aws_eip.aws.id
}


