terraform {
  required_providers {
    tencentcloud = {
      source = "tencentcloudstack/tencentcloud"
      version = "1.82.39"
    }

    local = {
      source = "hashicorp/local"
      version = "2.5.3"

    }
  }
}

provider "tencentcloud" {
  region = "ap-seoul"
}
