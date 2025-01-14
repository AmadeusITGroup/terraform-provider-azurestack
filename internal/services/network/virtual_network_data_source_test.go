package network_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"

	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
)

type VirtualNetworkDataSource struct{}

func TestAccVirtualNetworkDataSource_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurestack_virtual_network", "test")
	r := VirtualNetworkDataSource{}

	name := fmt.Sprintf("acctestvnet-%d", data.RandomInteger)

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("name").HasValue(name),
				check.That(data.ResourceName).Key("location").HasValue(location.Normalize(data.Locations.Primary)),
				check.That(data.ResourceName).Key("dns_servers.0").HasValue("10.0.0.4"),
				check.That(data.ResourceName).Key("address_space.0").HasValue("10.0.0.0/16"),
				check.That(data.ResourceName).Key("subnets.0").HasValue("subnet1"),
			),
		},
	})
}

func (VirtualNetworkDataSource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test" {
  name                = "acctestvnet-%d"
  address_space       = ["10.0.0.0/16"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
  dns_servers         = ["10.0.0.4"]

  subnet {
    name           = "subnet1"
    address_prefix = "10.0.1.0/24"
  }
}

data "azurestack_virtual_network" "test" {
  resource_group_name = azurestack_resource_group.test.name
  name                = azurestack_virtual_network.test.name
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger)
}

func TestAccVirtualNetworkDataSource_peering(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurestack_virtual_network", "test")
	r := VirtualNetworkDataSource{}

	virtualNetworkName := fmt.Sprintf("acctestvnet-1-%d", data.RandomInteger)

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.peering(data),
		},
		{
			Config: r.peeringWithDataSource(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("name").HasValue(virtualNetworkName),
				check.That(data.ResourceName).Key("address_space.0").HasValue("10.0.1.0/24"),
				check.That(data.ResourceName).Key("vnet_peerings.%").HasValue("1"),
			),
		},
	})
}

func (VirtualNetworkDataSource) peering(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test1" {
  name                = "acctestvnet-1-%d"
  address_space       = ["10.0.1.0/24"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_virtual_network" "test2" {
  name                = "acctestvnet-2-%d"
  address_space       = ["10.0.2.0/24"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_virtual_network_peering" "test1" {
  name                      = "peer-1to2"
  resource_group_name       = azurestack_resource_group.test.name
  virtual_network_name      = azurestack_virtual_network.test1.name
  remote_virtual_network_id = azurestack_virtual_network.test2.id
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}

func (VirtualNetworkDataSource) peeringWithDataSource(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurestack_virtual_network" "test1" {
  name                = "acctestvnet-1-%d"
  address_space       = ["10.0.1.0/24"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_virtual_network" "test2" {
  name                = "acctestvnet-2-%d"
  address_space       = ["10.0.2.0/24"]
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
}

resource "azurestack_virtual_network_peering" "test1" {
  name                      = "peer-1to2"
  resource_group_name       = azurestack_resource_group.test.name
  virtual_network_name      = azurestack_virtual_network.test1.name
  remote_virtual_network_id = azurestack_virtual_network.test2.id
}

data "azurestack_virtual_network" "test" {
  resource_group_name = azurestack_resource_group.test.name
  name                = azurestack_virtual_network.test1.name
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger, data.RandomInteger)
}
