package azurestack

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmNetworkInterfaceBackendAddressPoolAssociation() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetworkInterfaceBackendAddressPoolAssociationCreate,
		Read:   resourceNetworkInterfaceBackendAddressPoolAssociationRead,
		Delete: resourceNetworkInterfaceBackendAddressPoolAssociationDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"network_interface_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: azure.ValidateResourceID,
			},

			"ip_configuration_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"backend_address_pool_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: azure.ValidateResourceID,
			},
		},
	}
}

func resourceNetworkInterfaceBackendAddressPoolAssociationCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).ifaceClient
	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	log.Printf("[INFO] preparing arguments for Network Interface <-> Load Balancer Backend Address Pool Association creation.")

	networkInterfaceId := d.Get("network_interface_id").(string)
	ipConfigurationName := d.Get("ip_configuration_name").(string)
	backendAddressPoolId := d.Get("backend_address_pool_id").(string)

	// Locking this resource so we don't make modifications to it at the same time if there is a
	// During destroy nic, updates to nic are not allowed. Hence, nics are not detached
	azureStackLockByName(ipConfigurationName, networkInterfaceResourceName)
	defer azureStackUnlockByName(ipConfigurationName, networkInterfaceResourceName)

	nicId, err := azure.ParseAzureResourceID(networkInterfaceId)
	if err != nil {
		return err
	}
	resourceGroup := nicId.ResourceGroup
	networkInterfaceName := nicId.Path["networkInterfaces"]

	read, err := client.Get(ctx, resourceGroup, networkInterfaceName, "")
	if err != nil {
		if utils.ResponseWasNotFound(read.Response) {
			return fmt.Errorf("%s was not found!", *nicId)
		}

		return fmt.Errorf("retrieving %s: %+v", *nicId, err)
	}

	props := read.InterfacePropertiesFormat
	if props == nil {
		return fmt.Errorf("Error: `properties` was nil for %s", *nicId)
	}

	ipConfigs := props.IPConfigurations
	if ipConfigs == nil {
		return fmt.Errorf("Error: `properties.IPConfigurations` was nil for %s", *nicId)
	}

	c := FindNetworkInterfaceIPConfiguration(props.IPConfigurations, ipConfigurationName)
	if c == nil {
		return fmt.Errorf("Error: IP Configuration %q was not found on %s", ipConfigurationName, *nicId)
	}

	config := *c
	p := config.InterfaceIPConfigurationPropertiesFormat
	if p == nil {
		return fmt.Errorf("Error: `IPConfiguration.properties` was nil for %s", *nicId)
	}

	pools := make([]network.BackendAddressPool, 0)

	// first double-check it doesn't exist
	resourceId := fmt.Sprintf("%s/ipConfigurations/%s|%s", networkInterfaceId, ipConfigurationName, backendAddressPoolId)
	if p.LoadBalancerBackendAddressPools != nil {
		for _, existingPool := range *p.LoadBalancerBackendAddressPools {
			if id := existingPool.ID; id != nil {
				if *id == backendAddressPoolId {
					return tf.ImportAsExistsError("azurestack_network_interface_backend_address_pool_association", resourceId)
				}

				pools = append(pools, existingPool)
			}
		}
	}

	pool := network.BackendAddressPool{
		ID: utils.String(backendAddressPoolId),
	}
	pools = append(pools, pool)
	p.LoadBalancerBackendAddressPools = &pools

	props.IPConfigurations = updateNetworkInterfaceIPConfiguration(config, props.IPConfigurations)

	future, err := client.CreateOrUpdate(ctx, resourceGroup, networkInterfaceName, read)
	if err != nil {
		return fmt.Errorf("updating Backend Address Pool Association for %s: %+v", *nicId, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for completion of Backend Address Pool Association for %s: %+v", *nicId, err)
	}

	d.SetId(resourceId)

	return resourceNetworkInterfaceBackendAddressPoolAssociationRead(d, meta)
}

func resourceNetworkInterfaceBackendAddressPoolAssociationRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).ifaceClient
	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	splitId := strings.Split(d.Id(), "|")
	if len(splitId) != 2 {
		return fmt.Errorf("Expected ID to be in the format {networkInterfaceId}/ipConfigurations/{ipConfigurationName}|{backendAddressPoolId} but got %q", d.Id())
	}

	nicId, err := azure.ParseAzureResourceID(splitId[0])
	if err != nil {
		return err
	}

	ipConfigurationName := nicId.Path["ipConfigurations"]
	networkInterfaceName := nicId.Path["networkInterfaces"]
	resourceGroup := nicId.ResourceGroup
	backendAddressPoolId := splitId[1]

	read, err := client.Get(ctx, resourceGroup, networkInterfaceName, "")
	if err != nil {
		if utils.ResponseWasNotFound(read.Response) {
			log.Printf("Network Interface %q (Resource Group %q) was not found - removing from state!", networkInterfaceName, resourceGroup)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("retrieving Network Interface %q (Resource Group %q): %+v", networkInterfaceName, resourceGroup, err)
	}

	nicProps := read.InterfacePropertiesFormat
	if nicProps == nil {
		return fmt.Errorf("Error: `properties` was nil for Network Interface %q (Resource Group %q)", networkInterfaceName, resourceGroup)
	}

	ipConfigs := nicProps.IPConfigurations
	if ipConfigs == nil {
		return fmt.Errorf("Error: `properties.IPConfigurations` was nil for Network Interface %q (Resource Group %q)", networkInterfaceName, resourceGroup)
	}

	c := FindNetworkInterfaceIPConfiguration(nicProps.IPConfigurations, ipConfigurationName)
	if c == nil {
		log.Printf("IP Configuration %q was not found in Network Interface %q (Resource Group %q) - removing from state!", ipConfigurationName, networkInterfaceName, resourceGroup)
		d.SetId("")
		return nil
	}
	config := *c

	found := false
	if props := config.InterfaceIPConfigurationPropertiesFormat; props != nil {
		if backendPools := props.LoadBalancerBackendAddressPools; backendPools != nil {
			for _, pool := range *backendPools {
				if pool.ID == nil {
					continue
				}

				if *pool.ID == backendAddressPoolId {
					found = true
					break
				}
			}
		}
	}

	if !found {
		log.Printf("[DEBUG] Association between Network Interface %q (Resource Group %q) and Load Balancer Backend Pool %q was not found - removing from state!", networkInterfaceName, resourceGroup, backendAddressPoolId)
		d.SetId("")
		return nil
	}

	d.Set("backend_address_pool_id", backendAddressPoolId)
	d.Set("ip_configuration_name", ipConfigurationName)
	d.Set("network_interface_id", read.ID)

	return nil
}

func resourceNetworkInterfaceBackendAddressPoolAssociationDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).ifaceClient
	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	splitId := strings.Split(d.Id(), "|")
	if len(splitId) != 2 {
		return fmt.Errorf("Expected ID to be in the format {networkInterfaceId}/ipConfigurations/{ipConfigurationName}|{backendAddressPoolId} but got %q", d.Id())
	}

	nicId, err := azure.ParseAzureResourceID(splitId[0])
	if err != nil {
		return err
	}

	ipConfigurationName := nicId.Path["ipConfigurations"]
	networkInterfaceName := nicId.Path["networkInterfaces"]
	resourceGroup := nicId.ResourceGroup
	backendAddressPoolId := splitId[1]

	// Locking this resource so we don't make modifications to it at the same time if there is a
	// During destroy nic, updates to nic are not allowed. Hence, nics are not detached
	azureStackLockByName(ipConfigurationName, networkInterfaceResourceName)
	defer azureStackUnlockByName(ipConfigurationName, networkInterfaceResourceName)

	read, err := client.Get(ctx, resourceGroup, networkInterfaceName, "")
	if err != nil {
		if utils.ResponseWasNotFound(read.Response) {
			return fmt.Errorf("Network Interface %q (Resource Group %q) was not found!", networkInterfaceName, resourceGroup)
		}

		return fmt.Errorf("retrieving Network Interface %q (Resource Group %q): %+v", networkInterfaceName, resourceGroup, err)
	}

	nicProps := read.InterfacePropertiesFormat
	if nicProps == nil {
		return fmt.Errorf("Error: `properties` was nil for Network Interface %q (Resource Group %q)", networkInterfaceName, resourceGroup)
	}

	ipConfigs := nicProps.IPConfigurations
	if ipConfigs == nil {
		return fmt.Errorf("Error: `properties.IPConfigurations` was nil for Network Interface %q (Resource Group %q)", networkInterfaceName, resourceGroup)
	}

	c := FindNetworkInterfaceIPConfiguration(nicProps.IPConfigurations, ipConfigurationName)
	if c == nil {
		return fmt.Errorf("Error: IP Configuration %q was not found on Network Interface %q (Resource Group %q)", ipConfigurationName, networkInterfaceName, resourceGroup)
	}
	config := *c

	props := config.InterfaceIPConfigurationPropertiesFormat
	if props == nil {
		return fmt.Errorf("Error: Properties for IPConfiguration %q was nil for Network Interface %q (Resource Group %q)", ipConfigurationName, networkInterfaceName, resourceGroup)
	}

	backendAddressPools := make([]network.BackendAddressPool, 0)
	if backendPools := props.LoadBalancerBackendAddressPools; backendPools != nil {
		for _, pool := range *backendPools {
			if pool.ID == nil {
				continue
			}

			if *pool.ID != backendAddressPoolId {
				backendAddressPools = append(backendAddressPools, pool)
			}
		}
	}
	props.LoadBalancerBackendAddressPools = &backendAddressPools
	nicProps.IPConfigurations = updateNetworkInterfaceIPConfiguration(config, nicProps.IPConfigurations)

	future, err := client.CreateOrUpdate(ctx, resourceGroup, ipConfigurationName, read)
	if err != nil {
		return fmt.Errorf("removing Backend Address Pool Association for Network Interface %q (Resource Group %q): %+v", networkInterfaceName, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for removal of Backend Address Pool Association for NIC %q (Resource Group %q): %+v", networkInterfaceName, resourceGroup, err)
	}

	return nil
}