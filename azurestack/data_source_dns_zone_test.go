package azurestack

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccDataAzureStackDnsZone_basic(t *testing.T) {
	dataSourceName := "data.azurestack_dns_zone.test"
	ri := acctest.RandInt()

	name := fmt.Sprintf("acctestdnszone-%d", ri)
	resourceGroupName := fmt.Sprintf("acctestRG-%d", ri)

	config := testAccDataAzureStackDnsZoneBasic(name, resourceGroupName, testLocation())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testCheckAzureStackDnsZoneDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "name", name),
					resource.TestCheckResourceAttr(dataSourceName, "resource_group_name", resourceGroupName),
					resource.TestCheckResourceAttr(dataSourceName, "number_of_record_sets", fmt.Sprintf("acctest-%d", ri)),
					resource.TestCheckResourceAttr(dataSourceName, "idle_timeout_in_minutes", "30"),
					resource.TestCheckResourceAttr(dataSourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "tags.environment", "test"),
				),
			},
		},
	})
}

func testAccDataAzureStackDnsZoneBasic(name string, resourceGroupName string, location string) string {
	return fmt.Sprintf(`
resource "azurestack_resource_group" "test" {
  name     = "%s"
  location = "%s"
}

resource "azurestack_dns_zone" "test" {
  name                         = "acctestzone%s.com"
  resource_group_name          = "${azurestack_resource_group.test.name}"
  tags = {
    environment = "test"
  }
}

data "azurestack_dns_zone" "test" {
  name                = "${azurestack_dns_zone.test.name}"
  resource_group_name = "${azurestack_resource_group.test.name}"
}
`, resourceGroupName, location, name)
}
