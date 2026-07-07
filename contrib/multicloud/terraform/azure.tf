# Azure - Seoul (koreacentral)
# VNet: 10.205.0.0/16
# Subnet: 10.205.1.0/24  k8s node: 10.205.1.101  WG gateway: 10.205.1.102  WG tunnel: 10.200.0.5
# Pod subnet: 10.205.2.0/24  pod IPs: 10.205.2.x (secondary IP configs on nic1)
# Billing: pay-as-you-go (Azure default)

resource "azurerm_resource_group" "azure" {
  name     = "multicloud-test"
  location = var.azure_region
}

# ── Network ───────────────────────────────────────────────────────────────────

resource "azurerm_virtual_network" "azure" {
  name                = "multicloud-test-azure"
  address_space       = ["10.205.0.0/16"]
  location            = azurerm_resource_group.azure.location
  resource_group_name = azurerm_resource_group.azure.name
}

resource "azurerm_subnet" "azure" {
  name                 = "multicloud-test-azure"
  resource_group_name  = azurerm_resource_group.azure.name
  virtual_network_name = azurerm_virtual_network.azure.name
  address_prefixes     = ["10.205.1.0/24"]
}

# Dedicated subnet for pod IPs — secondary IP configurations are assigned here
# by the multicloud operator.
resource "azurerm_subnet" "azure-pod" {
  name                 = "multicloud-test-azure-pod"
  resource_group_name  = azurerm_resource_group.azure.name
  virtual_network_name = azurerm_virtual_network.azure.name
  address_prefixes     = ["10.205.2.0/24"]
}

# ── NSG ───────────────────────────────────────────────────────────────────────

resource "azurerm_network_security_group" "azure" {
  name                = "multicloud-test-azure"
  location            = azurerm_resource_group.azure.location
  resource_group_name = azurerm_resource_group.azure.name

  security_rule {
    name                       = "allow-ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-wg"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Udp"
    source_port_range          = "*"
    destination_port_range     = "51820"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-tc"
    priority                   = 200
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = tencentcloud_eip.tc.public_ip
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-tc-wg"
    priority                   = 201
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = tencentcloud_eip.tc-wg.public_ip
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-aws"
    priority                   = 202
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = aws_eip.aws.public_ip
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-aws-wg"
    priority                   = 203
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = aws_eip.aws-wg.public_ip
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-ali"
    priority                   = 204
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = alicloud_eip_address.ali.ip_address
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-ali-wg"
    priority                   = 205
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = alicloud_eip_address.ali-wg.ip_address
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-gcp"
    priority                   = 206
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = google_compute_address.gcp.address
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-cross-cloud-gcp-wg"
    priority                   = 207
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = google_compute_address.gcp-wg.address
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-internal"
    priority                   = 300
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "10.200.0.0/13"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "allow-outbound"
    priority                   = 100
    direction                  = "Outbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "azure" {
  subnet_id                 = azurerm_subnet.azure.id
  network_security_group_id = azurerm_network_security_group.azure.id
}

resource "azurerm_subnet_network_security_group_association" "azure-pod" {
  subnet_id                 = azurerm_subnet.azure-pod.id
  network_security_group_id = azurerm_network_security_group.azure.id
}

# ── Route table: cross-cloud via WG ──────────────────────────────────────────

resource "azurerm_route_table" "azure" {
  name                = "multicloud-test-azure"
  location            = azurerm_resource_group.azure.location
  resource_group_name = azurerm_resource_group.azure.name

  route {
    name                   = "to-tc"
    address_prefix         = "10.202.0.0/16"
    next_hop_type          = "VirtualAppliance"
    next_hop_in_ip_address = "10.205.1.102"
  }

  route {
    name                   = "to-ali"
    address_prefix         = "10.203.0.0/16"
    next_hop_type          = "VirtualAppliance"
    next_hop_in_ip_address = "10.205.1.102"
  }

  route {
    name                   = "to-aws"
    address_prefix         = "10.201.0.0/16"
    next_hop_type          = "VirtualAppliance"
    next_hop_in_ip_address = "10.205.1.102"
  }

  route {
    name                   = "to-gcp"
    address_prefix         = "10.204.0.0/16"
    next_hop_type          = "VirtualAppliance"
    next_hop_in_ip_address = "10.205.1.102"
  }

  route {
    name                   = "to-wg-mesh"
    address_prefix         = "10.200.0.0/24"
    next_hop_type          = "VirtualAppliance"
    next_hop_in_ip_address = "10.205.1.102"
  }
}

resource "azurerm_subnet_route_table_association" "azure" {
  subnet_id      = azurerm_subnet.azure.id
  route_table_id = azurerm_route_table.azure.id
}

resource "azurerm_subnet_route_table_association" "azure-pod" {
  subnet_id      = azurerm_subnet.azure-pod.id
  route_table_id = azurerm_route_table.azure.id
}

# ── Public IPs ────────────────────────────────────────────────────────────────

resource "azurerm_public_ip" "azure" {
  name                = "multicloud-test-azure"
  location            = azurerm_resource_group.azure.location
  resource_group_name = azurerm_resource_group.azure.name
  allocation_method   = "Static"
  sku                 = "Standard"
}

resource "azurerm_public_ip" "azure-wg" {
  name                = "multicloud-test-azure-wg"
  location            = azurerm_resource_group.azure.location
  resource_group_name = azurerm_resource_group.azure.name
  allocation_method   = "Static"
  sku                 = "Standard"
}

# ── k8s node NICs ─────────────────────────────────────────────────────────────

resource "azurerm_network_interface" "azure" {
  name                  = "multicloud-test-azure"
  location              = azurerm_resource_group.azure.location
  resource_group_name   = azurerm_resource_group.azure.name
  ip_forwarding_enabled = true

  ip_configuration {
    name                          = "primary"
    subnet_id                     = azurerm_subnet.azure.id
    private_ip_address_allocation = "Static"
    private_ip_address            = "10.205.1.101"
    public_ip_address_id          = azurerm_public_ip.azure.id
  }
}

# nic1: dedicated to pod IPs; multicloud operator assigns secondary IP configs here
resource "azurerm_network_interface" "azure-pod" {
  name                  = "multicloud-test-azure-pod"
  location              = azurerm_resource_group.azure.location
  resource_group_name   = azurerm_resource_group.azure.name
  ip_forwarding_enabled = true

  ip_configuration {
    name                          = "primary"
    subnet_id                     = azurerm_subnet.azure-pod.id
    private_ip_address_allocation = "Static"
    private_ip_address            = "10.205.2.4"
    primary                       = true
  }
}

# ── k8s node VM ───────────────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine" "azure" {
  name                  = "multicloud-test-azure"
  location              = azurerm_resource_group.azure.location
  resource_group_name   = azurerm_resource_group.azure.name
  size                  = "Standard_D4s_v3"
  admin_username        = "almalinux"
  network_interface_ids = [
    azurerm_network_interface.azure.id,
    azurerm_network_interface.azure-pod.id,
  ]

  admin_ssh_key {
    username   = "almalinux"
    public_key = data.local_file.public_key.content
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
    disk_size_gb         = 50
  }

  # AlmaLinux 9 (RHEL 9 compatible, free community image, available for CN subscriptions)
  source_image_reference {
    publisher = "almalinux"
    offer     = "almalinux-x86_64"
    sku       = "9-gen1"
    version   = "latest"
  }
}

# ── WireGuard gateway NIC ─────────────────────────────────────────────────────

resource "azurerm_network_interface" "azure-wg" {
  name                  = "multicloud-test-azure-wg"
  location              = azurerm_resource_group.azure.location
  resource_group_name   = azurerm_resource_group.azure.name
  ip_forwarding_enabled = true

  ip_configuration {
    name                          = "primary"
    subnet_id                     = azurerm_subnet.azure.id
    private_ip_address_allocation = "Static"
    private_ip_address            = "10.205.1.102"
    public_ip_address_id          = azurerm_public_ip.azure-wg.id
  }
}

# ── WireGuard gateway VM ──────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine" "azure-wg" {
  name                  = "multicloud-test-azure-wg"
  location              = azurerm_resource_group.azure.location
  resource_group_name   = azurerm_resource_group.azure.name
  size                  = "Standard_D2s_v3"
  admin_username        = "almalinux"
  network_interface_ids = [azurerm_network_interface.azure-wg.id]

  admin_ssh_key {
    username   = "almalinux"
    public_key = data.local_file.public_key.content
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
    disk_size_gb         = 40
  }

  source_image_reference {
    publisher = "almalinux"
    offer     = "almalinux-x86_64"
    sku       = "9-gen1"
    version   = "latest"
  }
}
