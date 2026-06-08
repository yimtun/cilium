terraform {
  required_providers {
    tencentcloud = {
      source  = "tencentcloudstack/tencentcloud"
      version = "1.82.39"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.220"
    }
    local = {
      source  = "hashicorp/local"
      version = "2.5.3"
    }
  }
}

provider "tencentcloud" {
  region = var.tc_region
}

provider "aws" {
  region = var.aws_region
}

provider "alicloud" {
  region = var.ali_region
}
