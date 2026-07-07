# Google Cloud - Seoul (asia-northeast3)
# VPC: 10.204.0.0/16
# nic0 subnet: 10.204.1.0/24  k8s node: 10.204.1.101  WG gateway: 10.204.1.102  WG tunnel: 10.200.0.4
# nic1 subnet: 10.204.2.0/24  k8s nic1: 10.204.2.1  pod alias IPs: 10.204.2.2+
# Billing: on-demand (GCP default, no preemptible)

data "google_compute_image" "rocky9" {
  family  = "rocky-linux-9"
  project = "rocky-linux-cloud"
}

resource "google_compute_network" "gcp" {
  name                    = "multicloud-test-gcp"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "gcp" {
  name          = "multicloud-test-gcp"
  ip_cidr_range = "10.204.1.0/24"
  region        = var.gcp_region
  network       = google_compute_network.gcp.id
}

# Second subnet for nic1 — GCP requires each NIC to be in a different subnet.
# Pod alias IPs are allocated from this range by the multicloud operator.
resource "google_compute_subnetwork" "gcp-pod" {
  name          = "multicloud-test-gcp-pod"
  ip_cidr_range = "10.204.2.0/24"
  region        = var.gcp_region
  network       = google_compute_network.gcp.id
}

resource "google_compute_firewall" "gcp_ssh" {
  name    = "multicloud-test-gcp-ssh"
  network = google_compute_network.gcp.name
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = ["0.0.0.0/0"]
}

resource "google_compute_firewall" "gcp_wg" {
  name    = "multicloud-test-gcp-wg"
  network = google_compute_network.gcp.name
  allow {
    protocol = "udp"
    ports    = ["51820"]
  }
  source_ranges = ["0.0.0.0/0"]
}

resource "google_compute_firewall" "gcp_internal" {
  name    = "multicloud-test-gcp-internal"
  network = google_compute_network.gcp.name
  allow {
    protocol = "all"
  }
  source_ranges = [
    "10.200.0.0/13",
  ]
}

resource "google_compute_firewall" "gcp_cross_cloud" {
  name    = "multicloud-test-gcp-cross-cloud"
  network = google_compute_network.gcp.name
  allow {
    protocol = "all"
  }
  source_ranges = [
    "${tencentcloud_eip.tc.public_ip}/32",
    "${tencentcloud_eip.tc-wg.public_ip}/32",
    "${aws_eip.aws.public_ip}/32",
    "${aws_eip.aws-wg.public_ip}/32",
    "${alicloud_eip_address.ali.ip_address}/32",
    "${alicloud_eip_address.ali-wg.ip_address}/32",
    "${azurerm_public_ip.azure.ip_address}/32",
    "${azurerm_public_ip.azure-wg.ip_address}/32",
  ]
}

resource "google_compute_address" "gcp" {
  name   = "multicloud-test-gcp"
  region = var.gcp_region
}

resource "google_compute_address" "gcp-wg" {
  name   = "multicloud-test-gcp-wg"
  region = var.gcp_region
}

resource "google_compute_instance" "gcp" {
  name           = "multicloud-test-gcp"
  machine_type   = "e2-standard-4"
  zone           = var.gcp_zone
  can_ip_forward = true

  boot_disk {
    initialize_params {
      image = data.google_compute_image.rocky9.self_link
      size  = 50
      type  = "pd-ssd"
    }
  }

  # nic0: node management, WireGuard, k8s control plane
  network_interface {
    subnetwork = google_compute_subnetwork.gcp.name
    network_ip = "10.204.1.101"
    access_config {
      nat_ip = google_compute_address.gcp.address
    }
  }

  # nic1: dedicated to pod IPs; multicloud operator assigns alias IPs here
  network_interface {
    subnetwork = google_compute_subnetwork.gcp-pod.name
    network_ip = "10.204.2.2"
    # no public IP needed
  }

  metadata = {
    ssh-keys = "rocky:${data.local_file.public_key.content}"
  }

  tags = ["multicloud-test"]
}

resource "google_compute_instance" "gcp-wg" {
  name           = "multicloud-test-gcp-wg"
  machine_type   = "e2-small"
  zone           = var.gcp_zone
  can_ip_forward = true

  boot_disk {
    initialize_params {
      image = data.google_compute_image.rocky9.self_link
      size  = 40
      type  = "pd-standard"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.gcp.name
    network_ip = "10.204.1.102"
    access_config {
      nat_ip = google_compute_address.gcp-wg.address
    }
  }

  metadata = {
    ssh-keys = "rocky:${data.local_file.public_key.content}"
  }

  tags = ["multicloud-test"]
}

resource "google_compute_route" "gcp_to_tc" {
  name                   = "multicloud-gcp-to-tc"
  network                = google_compute_network.gcp.name
  dest_range             = "10.202.0.0/16"
  next_hop_instance      = google_compute_instance.gcp-wg.self_link
  next_hop_instance_zone = var.gcp_zone
  priority               = 100
}

resource "google_compute_route" "gcp_to_aws" {
  name                   = "multicloud-gcp-to-aws"
  network                = google_compute_network.gcp.name
  dest_range             = "10.201.0.0/16"
  next_hop_instance      = google_compute_instance.gcp-wg.self_link
  next_hop_instance_zone = var.gcp_zone
  priority               = 100
}

resource "google_compute_route" "gcp_to_ali" {
  name                   = "multicloud-gcp-to-ali"
  network                = google_compute_network.gcp.name
  dest_range             = "10.203.0.0/16"
  next_hop_instance      = google_compute_instance.gcp-wg.self_link
  next_hop_instance_zone = var.gcp_zone
  priority               = 100
}

resource "google_compute_route" "gcp_to_wg_mesh" {
  name                   = "multicloud-gcp-to-wg-mesh"
  network                = google_compute_network.gcp.name
  dest_range             = "10.200.0.0/24"
  next_hop_instance      = google_compute_instance.gcp-wg.self_link
  next_hop_instance_zone = var.gcp_zone
  priority               = 100
}

resource "google_compute_route" "gcp_to_azure" {
  name                   = "multicloud-gcp-to-azure"
  network                = google_compute_network.gcp.name
  dest_range             = "10.205.0.0/16"
  next_hop_instance      = google_compute_instance.gcp-wg.self_link
  next_hop_instance_zone = var.gcp_zone
  priority               = 100
}
