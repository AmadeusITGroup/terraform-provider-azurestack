package compute_test

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/compute/parse"
	networkParse "github.com/hashicorp/terraform-provider-azurestack/internal/services/network/parse"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/ssh"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

type ImageDataSource struct{}

const SupportedImageDataSourceStorageTier = "Standard"

func TestAccDataSourceImage_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurestack_image", "test")
	r := ImageDataSource{}

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.setupUnmanagedDisks(data, SupportedImageDataSourceStorageTier),
			Check: acceptance.ComposeTestCheckFunc(
				data.CheckWithClientForResource(r.virtualMachineExists, "azurestack_virtual_machine.testsource"),
				data.CheckWithClientForResource(r.generalizeVirtualMachine(data), "azurestack_virtual_machine.testsource"),
			),
		},
		{
			Config: r.basicImage(data, SupportedImageDataSourceStorageTier),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("name").Exists(),
				check.That(data.ResourceName).Key("resource_group_name").Exists(),
				check.That(data.ResourceName).Key("os_disk.#").HasValue("1"),
				check.That(data.ResourceName).Key("os_disk.0.blob_uri").Exists(),
				check.That(data.ResourceName).Key("os_disk.0.caching").HasValue("None"),
				check.That(data.ResourceName).Key("os_disk.0.os_type").HasValue("Linux"),
				check.That(data.ResourceName).Key("os_disk.0.os_state").HasValue("Generalized"),
				check.That(data.ResourceName).Key("os_disk.0.size_gb").HasValue("30"),
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
			Config: r.setupUnmanagedDisks(data, SupportedImageDataSourceStorageTier),
			Check: acceptance.ComposeTestCheckFunc(
				data.CheckWithClientForResource(r.virtualMachineExists, "azurestack_virtual_machine.testsource"),
				data.CheckWithClientForResource(r.generalizeVirtualMachine(data), "azurestack_virtual_machine.testsource"),
			),
		},
		{
			// We have to create the images first explicitly, then retrieve the data source, because in this case we do not have explicit dependency on the image resources
			Config: r.localFilter_setup(data, SupportedImageDataSourceStorageTier),
		},
		{
			Config: r.localFilter(data, SupportedImageDataSourceStorageTier),
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

func (r ImageDataSource) setupUnmanagedDisks(data acceptance.TestData, storageTier string) string {
	template := r.template(data)
	return fmt.Sprintf(`
provider "azurestack" {
  features {}
}

%s

resource "azurestack_network_interface" "testsource" {
  name                = "acctnicsource-${local.number}"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  ip_configuration {
    name                          = "testconfigurationsource"
    subnet_id                     = azurestack_subnet.test.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurestack_public_ip.test.id
  }
}

resource "azurestack_storage_account" "test" {
  name                     = "accsa${local.random_string}"
  resource_group_name      = azurestack_resource_group.test.name
  location                 = azurestack_resource_group.test.location
  account_tier             = "%s"
  account_replication_type = "LRS"
}

resource "azurestack_storage_container" "test" {
  name                  = "vhds"
  storage_account_name  = azurestack_storage_account.test.name
  container_access_type = "blob"
}

# NOTE: using the legacy vm resource since this test requires an unmanaged disk
resource "azurestack_virtual_machine" "testsource" {
  name                  = "testsource"
  location              = azurestack_resource_group.test.location
  resource_group_name   = azurestack_resource_group.test.name
  network_interface_ids = [azurestack_network_interface.testsource.id]
  vm_size               = "Standard_D1_v2"

  storage_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "16.04-LTS"
    version   = "latest"
  }

  storage_os_disk {
    name          = "myosdisk1"
    vhd_uri       = "${azurestack_storage_account.test.primary_blob_endpoint}${azurestack_storage_container.test.name}/myosdisk1.vhd"
    caching       = "ReadWrite"
    create_option = "FromImage"
    disk_size_gb  = "30"
  }

  os_profile {
    computer_name  = "mdimagetestsource"
    admin_username = local.admin_username
    admin_password = local.admin_password
  }

  os_profile_linux_config {
    disable_password_authentication = false
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}
`, template, storageTier)
}

func (r ImageDataSource) basicImage(data acceptance.TestData, storageTier string) string {
	template := r.setupUnmanagedDisks(data, storageTier)
	return fmt.Sprintf(`
%s

resource "azurestack_image" "test" {
  name                = "accteste"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  os_disk {
    os_type  = "Linux"
    os_state = "Generalized"
    blob_uri = "${azurestack_storage_account.test.primary_blob_endpoint}${azurestack_storage_container.test.name}/myosdisk1.vhd"
    size_gb  = 30
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
`, template)
}

func (r ImageDataSource) localFilter_setup(data acceptance.TestData, storageTier string) string {
	template := r.setupUnmanagedDisks(data, storageTier)
	return fmt.Sprintf(`
%s

resource "azurestack_image" "abc" {
  name                = "abc-acctest-%d"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name

  os_disk {
    os_type  = "Linux"
    os_state = "Generalized"
    blob_uri = "${azurestack_storage_account.test.primary_blob_endpoint}${azurestack_storage_container.test.name}/myosdisk1.vhd"
    size_gb  = 30
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
    blob_uri = "${azurestack_storage_account.test.primary_blob_endpoint}${azurestack_storage_container.test.name}/myosdisk1.vhd"
    size_gb  = 30
    caching  = "None"
  }

  tags = {
    environment = "Dev"
    cost-center = "Ops"
  }
}
`, template, data.RandomInteger, data.RandomInteger)
}

func (r ImageDataSource) localFilter(data acceptance.TestData, storageTier string) string {
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
`, r.localFilter_setup(data, storageTier))
}

func (ImageDataSource) template(data acceptance.TestData) string {
	return fmt.Sprintf(`
locals {
  number            = "%d"
  location          = %q
  domain_name_label = "acctestvm-%s"
  random_string     = %q
  admin_username    = "testadmin%d"
  admin_password    = "Password1234!%d"
}

resource "azurestack_resource_group" "test" {
  name     = "acctestRG-${local.number}"
  location = local.location
}

resource "azurestack_virtual_network" "test" {
  name                = "acctvn-${local.number}"
  resource_group_name = azurestack_resource_group.test.name
  location            = azurestack_resource_group.test.location
  address_space       = ["10.0.0.0/16"]
}

resource "azurestack_subnet" "test" {
  name                 = "internal"
  resource_group_name  = azurestack_resource_group.test.name
  virtual_network_name = azurestack_virtual_network.test.name
  address_prefix       = "10.0.2.0/24"
}

resource "azurestack_public_ip" "test" {
  name                = "acctpip-${local.number}"
  location            = azurestack_resource_group.test.location
  resource_group_name = azurestack_resource_group.test.name
  allocation_method   = "Dynamic"
  domain_name_label   = local.domain_name_label
}
`, data.RandomInteger, data.Locations.Primary, data.RandomString, data.RandomString, data.RandomInteger, data.RandomInteger)
}

func (ImageDataSource) virtualMachineExists(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
	id, err := parse.VirtualMachineID(state.ID)
	if err != nil {
		return err
	}

	resp, err := client.Compute.VMClient.Get(ctx, id.ResourceGroup, id.Name, "")
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			return fmt.Errorf("%s does not exist", *id)
		}

		return fmt.Errorf("Bad: Get on client: %+v", err)
	}

	return nil
}

func (ImageDataSource) generalizeVirtualMachine(data acceptance.TestData) func(context.Context, *clients.Client, *pluginsdk.InstanceState) error {
	return func(ctx context.Context, client *clients.Client, state *pluginsdk.InstanceState) error {
		id, err := parse.VirtualMachineID(state.ID)
		if err != nil {
			return err
		}

		// these are nested in a Set in the Legacy VM resource, simpler to compute them
		userName := fmt.Sprintf("testadmin%d", data.RandomInteger)
		password := fmt.Sprintf("Password1234!%d", data.RandomInteger)

		// first retrieve the Virtual Machine, since we need to find
		nicIdRaw := state.Attributes["network_interface_ids.0"]
		nicId, err := networkParse.NetworkInterfaceID(nicIdRaw)
		if err != nil {
			return err
		}

		log.Printf("[DEBUG] Retrieving Network Interface..")
		nic, err := client.Network.InterfacesClient.Get(ctx, nicId.ResourceGroup, nicId.Name, "")
		if err != nil {
			return fmt.Errorf("retrieving %s: %+v", *nicId, err)
		}

		publicIpRaw := ""
		if props := nic.InterfacePropertiesFormat; props != nil {
			if configs := props.IPConfigurations; configs != nil {
				for _, config := range *props.IPConfigurations {
					if config.InterfaceIPConfigurationPropertiesFormat == nil {
						continue
					}

					if config.InterfaceIPConfigurationPropertiesFormat.PublicIPAddress == nil {
						continue
					}

					if config.InterfaceIPConfigurationPropertiesFormat.PublicIPAddress.ID == nil {
						continue
					}

					publicIpRaw = *config.InterfaceIPConfigurationPropertiesFormat.PublicIPAddress.ID
					break
				}
			}
		}
		if publicIpRaw == "" {
			return fmt.Errorf("retrieving %s: could not determine Public IP Address ID", *nicId)
		}

		log.Printf("[DEBUG] Retrieving Public IP Address %q..", publicIpRaw)
		publicIpId, err := networkParse.PublicIpAddressID(publicIpRaw)
		if err != nil {
			return err
		}

		publicIpAddress, err := client.Network.PublicIPsClient.Get(ctx, publicIpId.ResourceGroup, publicIpId.Name, "")
		if err != nil {
			return fmt.Errorf("retrieving %s: %+v", *publicIpId, err)
		}
		fqdn := ""
		if props := publicIpAddress.PublicIPAddressPropertiesFormat; props != nil {
			if dns := props.DNSSettings; dns != nil {
				if dns.Fqdn != nil {
					fqdn = *dns.Fqdn
				}
			}
		}
		if fqdn == "" {
			return fmt.Errorf("unable to determine FQDN for %q", *publicIpId)
		}

		log.Printf("[DEBUG] Running Generalization Command..")
		sshGeneralizationCommand := ssh.Runner{
			Hostname: fqdn,
			Port:     22,
			Username: userName,
			Password: password,
			CommandsToRun: []string{
				ssh.LinuxAgentDeprovisionCommand,
			},
		}
		if err := sshGeneralizationCommand.Run(ctx); err != nil {
			return fmt.Errorf("Bad: running generalization command: %+v", err)
		}

		log.Printf("[DEBUG] Deallocating VM..")
		// Upgrading to the 2021-07-01 exposed a new hibernate parameter in the GET method
		future, err := client.Compute.VMClient.Deallocate(ctx, id.ResourceGroup, id.Name)
		if err != nil {
			return fmt.Errorf("Bad: deallocating vm: %+v", err)
		}
		log.Printf("[DEBUG] Waiting for Deallocation..")
		if err = future.WaitForCompletionRef(ctx, client.Compute.VMClient.Client); err != nil {
			return fmt.Errorf("Bad: waiting for deallocation: %+v", err)
		}

		log.Printf("[DEBUG] Generalizing VM..")
		if _, err = client.Compute.VMClient.Generalize(ctx, id.ResourceGroup, id.Name); err != nil {
			return fmt.Errorf("Bad: Generalizing error %+v", err)
		}

		return nil
	}
}
