package compute_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
)

type ImageDataSource struct{}

func TestAccDataSourceImage_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurestack_image", "test")
	r := ImageDataSource{}

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("name").Exists(),
				check.That(data.ResourceName).Key("resource_group_name").Exists(),
				check.That(data.ResourceName).Key("os_disk.#").HasValue("1"),
				check.That(data.ResourceName).Key("os_disk.0.caching").HasValue("None"),
				check.That(data.ResourceName).Key("os_disk.0.os_type").HasValue("Linux"),
				check.That(data.ResourceName).Key("os_disk.0.os_state").HasValue("Generalized"),
				check.That(data.ResourceName).Key("os_disk.0.size_gb").HasValue("45"),
				check.That(data.ResourceName).Key("data_disk.#").HasValue("0"),
				check.That(data.ResourceName).Key("tags.%").HasValue("2"),
				check.That(data.ResourceName).Key("tags.environment").HasValue("Dev"),
				check.That(data.ResourceName).Key("tags.cost-center").HasValue("Ops"),
			),
		},
	})
}

func TestAccDataSourceImage_localFilter(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurestack_image", "test1")
	r := ImageDataSource{}

	descDataSourceName := "data.azurestack_image.test2"

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			// We have to create the images first explicitly, then retrieve the data source, because in this case we do not have explicit dependency on the image resources
			Config: r.localFilter_setup(data),
		},
		{
			Config: r.localFilter(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("name").Exists(),
				check.That(data.ResourceName).Key("resource_group_name").Exists(),
				check.That(data.ResourceName).Key("name").HasValue(fmt.Sprintf("def-acctest-%d", data.RandomInteger)),
				acceptance.TestCheckResourceAttrSet(descDataSourceName, "name"),
				acceptance.TestCheckResourceAttrSet(descDataSourceName, "resource_group_name"),
				acceptance.TestCheckResourceAttr(descDataSourceName, "name", fmt.Sprintf("def-acctest-%d", data.RandomInteger)),
			),
		},
	})
}

func (ImageDataSource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test" {
  name                = "acctestvn-%d"
  address_space       = ["10.0.0.0/16"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_subnet" "test" {
  name                 = "internal"
  resource_group_name  = azurestack_resource_group.test.name
  virtual_network_name = azurestack_virtual_network.test.name
  address_prefix       = "10.0.2.0/24"
}

resource "azurestack_public_ip" "test" {
  name                = "acctestpip%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
  allocation_method   = "Dynamic"
  domain_name_label   = "acctestpip%d"
}

resource "azurestack_network_interface" "testsource" {
  name                = "acctestnic-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  ip_configuration {
    name                          = "testconfigurationsource"
    subnet_id                     = azurestack_subnet.test.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurestack_public_ip.test.id
  }
}

resource "azurestack_virtual_machine" "testsource" {
  name                  = "acctestvm-%d"
  location              = azurestack_resource_group.test.location
  resource_group_name   = azurestack_resource_group.test.name
  network_interface_ids = [azurestack_network_interface.testsource.id]
  vm_size               = "Standard_DS1_v2"

  storage_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "16.04-LTS"
    version   = "latest"
  }

  storage_os_disk {
    name          = "myosdisk1"
    caching       = "ReadWrite"
    create_option = "FromImage"
    disk_size_gb  = "45"
  }

  os_profile {
    computer_name  = "acctest-%d"
    admin_username = "tfuser"
    admin_password = "P@ssW0RD7890"
  }

  os_profile_linux_config {
    disable_password_authentication = false
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}

resource "azurestack_image" "test" {
  name                = "acctest-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  os_disk {
    os_type  = "Linux"
    os_state = "Generalized"
    managed_disk_id = azurestack_virtual_machine.testsource.storage_os_disk[0].managed_disk_id
    size_gb  = 45
    caching  = "None"
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}

data "azurestack_image" "test" {
  name                = azurestack_image.test.name
  resource_group_name = azurestack_resource_group.test.name
}

output "location" {
  value = data.azurestack_image.test.location
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger)
}

func (ImageDataSource) localFilter_setup(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test" {
  name                = "acctestvn-%d"
  address_space       = ["10.0.0.0/16"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_subnet" "test" {
  name                 = "internal"
  resource_group_name  = azurestack_resource_group.test.name
  virtual_network_name = azurestack_virtual_network.test.name
  address_prefix       = "10.0.2.0/24"
}

resource "azurestack_public_ip" "test" {
  name                = "acctestpip%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
  allocation_method   = "Dynamic"
  domain_name_label   = "acctestpip%d"
}

resource "azurestack_network_interface" "testsource" {
  name                = "acctestnic-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  ip_configuration {
    name                          = "testconfigurationsource"
    subnet_id                     = azurestack_subnet.test.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurestack_public_ip.test.id
  }
}

resource "azurestack_virtual_machine" "testsource" {
  name                  = "acctestvm-%d"
  location              = azurestack_resource_group.test.location
  resource_group_name   = azurestack_resource_group.test.name
  network_interface_ids = [azurestack_network_interface.testsource.id]
  vm_size               = "Standard_DS1_v2"

  storage_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "16.04-LTS"
    version   = "latest"
  }

  storage_os_disk {
    name          = "myosdisk1"
    caching       = "ReadWrite"
    create_option = "FromImage"
    disk_size_gb  = "45"
  }

  os_profile {
    computer_name  = "acctest-%d"
    admin_username = "tfuser"
    admin_password = "P@ssW0RD7890"
  }

  os_profile_linux_config {
    disable_password_authentication = false
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}

resource "azurestack_image" "abc" {
  name                = "abc-acctest-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  os_disk {
    os_type  = "Linux"
    os_state = "Generalized"
    managed_disk_id = azurestack_virtual_machine.testsource.storage_os_disk[0].managed_disk_id
    size_gb  = 45
    caching  = "None"
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}

resource "azurestack_image" "def" {
  name                = "def-acctest-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  os_disk {
    os_type  = "Linux"
    os_state = "Generalized"
    managed_disk_id = azurestack_virtual_machine.testsource.storage_os_disk[0].managed_disk_id
    size_gb  = 45
    caching  = "None"
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger, data.RandomInteger)
}

func (r ImageDataSource) localFilter(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

data "azurestack_image" "test1" {
  name_regex          = "^def-acctest-\\d+"
  resource_group_name = azurestack_resource_group.test.name
}

data "azurestack_image" "test2" {
  name_regex          = "^[a-z]+-acctest-\\d+"
  sort_descending     = true
  resource_group_name = azurestack_resource_group.test.name
}
`, r.localFilter_setup(data))
}
