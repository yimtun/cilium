variable "tc_region"  { default = "ap-seoul" }
variable "aws_region" { default = "ap-northeast-2" }
variable "ali_region" { default = "ap-northeast-2" }
variable "gcp_region" { default = "asia-northeast3" }
variable "gcp_zone"   { default = "asia-northeast3-a" }
variable "gcp_project" { default = "valid-flow-457210-p7" }

variable "tc_image_id"  {default = "img-ccs1km2l"}
variable "aws_ami_id" { default = "ami-042d2540f68d273fe" }
variable "ali_image_id" {default = "rockylinux_9_5_x64_20G_alibase_20250526.vhd"}

variable "azure_region"          { default = "koreacentral" }
variable "azure_subscription_id" { default = "" }
