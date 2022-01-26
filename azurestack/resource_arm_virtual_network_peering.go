package azurestack

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-10-01/network"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-provider-azurestack/azurestack/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurestack/azurestack/helpers/response"
)

// peerMutex is used to prevent multiple Peering resources being created, updated
// or deleted at the same time
var vnetPeeringResourceName = "azurestack_virtual_network_peering"

func resourceArmVirtualNetworkPeering() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmVirtualNetworkPeeringCreateUpdate,
		Read:   resourceArmVirtualNetworkPeeringRead,
		Update: resourceArmVirtualNetworkPeeringCreateUpdate,
		Delete: resourceArmVirtualNetworkPeeringDelete,
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
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"resource_group_name": resourceGroupNameSchema(),

			"virtual_network_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"remote_virtual_network_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"allow_virtual_network_access": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},

			"allow_forwarded_traffic": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},

			"allow_gateway_transit": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},

			"use_remote_gateways": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourceArmVirtualNetworkPeeringCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).vnetPeeringClient
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[INFO] preparing arguments for Azure ARM virtual network peering creation.")

	name := d.Get("name").(string)
	vnetName := d.Get("virtual_network_name").(string)
	resGroup := d.Get("resource_group_name").(string)

	peer := network.VirtualNetworkPeering{
		Name:                                  &name,
		VirtualNetworkPeeringPropertiesFormat: getVirtualNetworkPeeringProperties(d),
	}

	azureStackLockByName(name, vnetPeeringResourceName)
	defer azureStackUnlockByName(name, vnetPeeringResourceName)

	if err := resource.Retry(300*time.Second, retryVnetPeeringsClientCreateUpdate(d, resGroup, vnetName, name, peer, meta)); err != nil {
		return err
	}

	read, err := client.Get(ctx, resGroup, vnetName, name)
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read ID of Virtual Network Peering %q (resource group %q)", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmVirtualNetworkPeeringRead(d, meta)
}

func resourceArmVirtualNetworkPeeringRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).vnetPeeringClient
	ctx := meta.(*ArmClient).StopContext

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	vnetName := id.Path["virtualNetworks"]
	name := id.Path["virtualNetworkPeerings"]

	resp, err := client.Get(ctx, resGroup, vnetName, name)
	if err != nil {
		if response.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("making Read request on Azure virtual network peering %q: %+v", name, err)
	}

	// update appropriate values
	d.Set("resource_group_name", resGroup)
	d.Set("name", resp.Name)
	d.Set("virtual_network_name", vnetName)

	if peer := resp.VirtualNetworkPeeringPropertiesFormat; peer != nil {
		d.Set("allow_virtual_network_access", peer.AllowVirtualNetworkAccess)
		d.Set("allow_forwarded_traffic", peer.AllowForwardedTraffic)
		d.Set("allow_gateway_transit", peer.AllowGatewayTransit)
		d.Set("use_remote_gateways", peer.UseRemoteGateways)
		if network := peer.RemoteVirtualNetwork; network != nil {
			d.Set("remote_virtual_network_id", network.ID)
		}
	}

	return nil
}

func resourceArmVirtualNetworkPeeringDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).vnetPeeringClient
	ctx := meta.(*ArmClient).StopContext

	id, err := azure.ParseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	vnetName := id.Path["virtualNetworks"]
	name := id.Path["virtualNetworkPeerings"]

	azureStackLockByName(name, vnetPeeringResourceName)
	defer azureStackUnlockByName(name, vnetPeeringResourceName)

	future, err := client.Delete(ctx, resGroup, vnetName, name)
	if err != nil {
		return fmt.Errorf("deleting Virtual Network Peering %q (Network %q / RG %q): %+v", name, vnetName, resGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for deletion of Virtual Network Peering %q (Network %q / RG %q): %+v", name, vnetName, resGroup, err)
	}

	return err
}

func getVirtualNetworkPeeringProperties(d *schema.ResourceData) *network.VirtualNetworkPeeringPropertiesFormat {
	allowVirtualNetworkAccess := d.Get("allow_virtual_network_access").(bool)
	allowForwardedTraffic := d.Get("allow_forwarded_traffic").(bool)
	allowGatewayTransit := d.Get("allow_gateway_transit").(bool)
	useRemoteGateways := d.Get("use_remote_gateways").(bool)
	remoteVirtualNetworkID := d.Get("remote_virtual_network_id").(string)

	return &network.VirtualNetworkPeeringPropertiesFormat{
		AllowVirtualNetworkAccess: &allowVirtualNetworkAccess,
		AllowForwardedTraffic:     &allowForwardedTraffic,
		AllowGatewayTransit:       &allowGatewayTransit,
		UseRemoteGateways:         &useRemoteGateways,
		RemoteVirtualNetwork: &network.SubResource{
			ID: &remoteVirtualNetworkID,
		},
	}
}

func retryVnetPeeringsClientCreateUpdate(d *schema.ResourceData, resGroup string, vnetName string, name string, peer network.VirtualNetworkPeering, meta interface{}) func() *resource.RetryError {
	return func() *resource.RetryError {
		vnetPeeringsClient := meta.(*ArmClient).vnetPeeringClient
		ctx := meta.(*ArmClient).StopContext

		future, err := vnetPeeringsClient.CreateOrUpdate(ctx, resGroup, vnetName, name, peer)
		if err != nil {
			if response.ResponseErrorIsRetryable(err) {
				return resource.RetryableError(err)
			} else if future.Response().StatusCode == 400 && strings.Contains(err.Error(), "ReferencedResourceNotProvisioned") {
				// Resource is not yet ready, this may be the case if the Vnet was just created or another peering was just initiated.
				return resource.RetryableError(err)
			}

			return resource.NonRetryableError(err)
		}

		if err = future.WaitForCompletionRef(ctx, vnetPeeringsClient.Client); err != nil {
			return resource.NonRetryableError(err)
		}

		return nil
	}
}
