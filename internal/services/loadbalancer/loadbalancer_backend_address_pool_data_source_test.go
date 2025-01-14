package loadbalancer_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/acceptance/check"
)

func TestAccBackendAddressPoolDataSource_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurestack_lb_backend_address_pool", "test")
	r := LoadBalancerBackendAddressPool{}

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.dataSourceBasic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("id").Exists(),
			),
		},
	})
}

func (r LoadBalancerBackendAddressPool) dataSourceBasic(data acceptance.TestData) string {
	resource := r.basicSkuBasic(data)
	return fmt.Sprintf(`
%s

data "azurestack_lb_backend_address_pool" "test" {
  name            = azurestack_lb_backend_address_pool.test.name
  loadbalancer_id = azurestack_lb_backend_address_pool.test.loadbalancer_id
}
`, resource)
}
