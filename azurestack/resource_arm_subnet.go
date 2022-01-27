package azurestack

import (
	"fmt"
	"log"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-provider-azurestack/azurestack/helpers/response"
)

var subnetResourceName = "azurestack_subnet"

func resourceArmSubnet() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmSubnetCreate,
		Read:   resourceArmSubnetRead,
		Update: resourceArmSubnetCreate,
		Delete: resourceArmSubnetDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
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

			"address_prefix": {
				Type:     schema.TypeString,
				Required: true,
			},

			"network_security_group_id": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"route_table_id": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"ip_configurations": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			// Not supported for 2017-03-09 profile
			// "service_endpoints": {
			// 	Type:     schema.TypeList,
			// 	Optional: true,
			// 	Elem:     &schema.Schema{Type: schema.TypeString},
			// },
		},
	}
}

func resourceArmSubnetCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).subnetClient
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[INFO] preparing arguments for Azure ARM Subnet creation.")

	name := d.Get("name").(string)
	vnetName := d.Get("virtual_network_name").(string)
	resGroup := d.Get("resource_group_name").(string)
	addressPrefix := d.Get("address_prefix").(string)

	azureStackLockByName(vnetName, virtualNetworkResourceName)
	defer azureStackUnlockByName(vnetName, virtualNetworkResourceName)

	properties := network.SubnetPropertiesFormat{
		AddressPrefix: &addressPrefix,
	}

	if v, ok := d.GetOk("network_security_group_id"); ok {
		nsgId := v.(string)
		properties.NetworkSecurityGroup = &network.SecurityGroup{
			ID: &nsgId,
		}

		networkSecurityGroupName, err := parseNetworkSecurityGroupName(nsgId)
		if err != nil {
			return err
		}

		azureStackLockByName(networkSecurityGroupName, networkSecurityGroupResourceName)
		defer azureStackUnlockByName(networkSecurityGroupName, networkSecurityGroupResourceName)
	}

	if v, ok := d.GetOk("route_table_id"); ok {
		rtId := v.(string)
		properties.RouteTable = &network.RouteTable{
			ID: &rtId,
		}

		routeTableName, err := parseRouteTableName(rtId)
		if err != nil {
			return err
		}

		azureStackLockByName(routeTableName, routeTableResourceName)
		defer azureStackUnlockByName(routeTableName, routeTableResourceName)
	}

	// Not supported for 2017-03-09 profile
	// serviceEndpoints, serviceEndpointsErr := expandAzureStackServiceEndpoints(d)
	// if serviceEndpointsErr != nil {
	// 	return fmt.Errorf("Building list of Service Endpoints: %+v", serviceEndpointsErr)
	// }

	// properties.ServiceEndpoints = &serviceEndpoints

	subnet := network.Subnet{
		Name:                   &name,
		SubnetPropertiesFormat: &properties,
	}

	future, err := client.CreateOrUpdate(ctx, resGroup, vnetName, name, subnet)
	if err != nil {
		return fmt.Errorf("Creating/Updating Subnet %q (VN %q / Resource Group %q): %+v", name, vnetName, resGroup, err)
	}

	err = future.WaitForCompletionRef(ctx, client.Client)
	if err != nil {
		return fmt.Errorf("waiting for completion of Subnet %q (VN %q / Resource Group %q): %+v", name, vnetName, resGroup, err)
	}

	read, err := client.Get(ctx, resGroup, vnetName, name, "")
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read ID of Subnet %q (VN %q / Resource Group %q)", vnetName, name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmSubnetRead(d, meta)
}

func resourceArmSubnetRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).subnetClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	vnetName := id.Path["virtualNetworks"]
	name := id.Path["subnets"]

	resp, err := client.Get(ctx, resGroup, vnetName, name, "")

	if err != nil {
		if response.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("making Read request on Azure Subnet %q: %+v", name, err)
	}

	d.Set("name", name)
	d.Set("resource_group_name", resGroup)
	d.Set("virtual_network_name", vnetName)

	if props := resp.SubnetPropertiesFormat; props != nil {
		d.Set("address_prefix", props.AddressPrefix)

		if props.NetworkSecurityGroup != nil {
			d.Set("network_security_group_id", props.NetworkSecurityGroup.ID)
		}

		if props.RouteTable != nil {
			d.Set("route_table_id", props.RouteTable.ID)
		}

		ips := flattenSubnetIPConfigurations(props.IPConfigurations)
		if err := d.Set("ip_configurations", ips); err != nil {
			return err
		}

		// Not supported for 2017-03-09 profile
		// serviceEndpoints := flattenSubnetServiceEndpoints(props.ServiceEndpoints)
		// if err := d.Set("service_endpoints", serviceEndpoints); err != nil {
		// 	return err
		// }
	}

	return nil
}

func resourceArmSubnetDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).subnetClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	name := id.Path["subnets"]
	vnetName := id.Path["virtualNetworks"]

	if v, ok := d.GetOk("network_security_group_id"); ok {
		networkSecurityGroupId := v.(string)
		networkSecurityGroupName, err := parseNetworkSecurityGroupName(networkSecurityGroupId)
		if err != nil {
			return err
		}

		azureStackLockByName(networkSecurityGroupName, networkSecurityGroupResourceName)
		defer azureStackUnlockByName(networkSecurityGroupName, networkSecurityGroupResourceName)
	}

	azureStackLockByName(vnetName, virtualNetworkResourceName)
	defer azureStackUnlockByName(vnetName, virtualNetworkResourceName)

	azureStackLockByName(name, subnetResourceName)
	defer azureStackUnlockByName(name, subnetResourceName)

	if v, ok := d.GetOk("route_table_id"); ok {
		rtId := v.(string)
		routeTableName, err := parseRouteTableName(rtId)
		if err != nil {
			return err
		}

		azureStackLockByName(routeTableName, routeTableResourceName)
		defer azureStackUnlockByName(routeTableName, routeTableResourceName)

		// This behaviour is only for AzureStack
		// If the route table is not dissasociated from the subnet prior to deletion
		// it will fail.

		// Get the subnet to dissasociate the route table, if we don't do this
		// the subnet cannot be deleted

		resp, err := client.Get(ctx, resGroup, vnetName, name, "")
		if err != nil {
			if response.ResponseWasNotFound(resp.Response) {
				d.SetId("")
				return nil
			}
			return fmt.Errorf("making Read request on Azure Subnet %q: %+v", name, err)
		}

		// Set the route table to nil
		resp.SubnetPropertiesFormat.RouteTable = nil

		log.Printf("[DEBUG] Dissasociating Subnet %q (VN %q / Resource Group %q)", name, vnetName, resGroup)

		// Dissasociate the subnet
		future, err := client.CreateOrUpdate(ctx, resGroup, vnetName, name, resp)
		if err != nil {
			return fmt.Errorf("Dissasociating Subnet %q (VN %q / Resource Group %q): %+v", name, vnetName, resGroup, err)
		}

		err = future.WaitForCompletionRef(ctx, client.Client)
		if err != nil {
			return fmt.Errorf("waiting for completion of Subnet %q (VN %q / Resource Group %q): %+v", name, vnetName, resGroup, err)
		}

	}

	future, err := client.Delete(ctx, resGroup, vnetName, name)
	if err != nil {
		return fmt.Errorf("deleting Subnet %q (VN %q / Resource Group %q): %+v", name, vnetName, resGroup, err)
	}

	err = future.WaitForCompletionRef(ctx, client.Client)
	if err != nil {
		return fmt.Errorf("waiting for completion for Subnet %q (VN %q / Resource Group %q): %+v", name, vnetName, resGroup, err)
	}

	return nil
}

// Since ServiceEndpointPropertiesFormat is not on the 2017-03-09 profile
// This will not compile

// func expandAzureStackServiceEndpoints(d *schema.ResourceData) ([]network.ServiceEndpointPropertiesFormat, error) {
// 	serviceEndpoints := d.Get("service_endpoints").([]interface{})
// 	enpoints := make([]network.ServiceEndpointPropertiesFormat, 0)
//
// 	for _, serviceEndpointsRaw := range serviceEndpoints {
// 		data := serviceEndpointsRaw.(string)
//
// 		endpoint := network.ServiceEndpointPropertiesFormat{
// 			Service: &data,
// 		}
//
// 		enpoints = append(enpoints, endpoint)
// 	}
//
// 	return enpoints, nil
// }

// func flattenSubnetServiceEndpoints(serviceEndpoints *[]network.ServiceEndpointPropertiesFormat) []string {
// 	endpoints := make([]string, 0)
//
// 	if serviceEndpoints != nil {
// 		for _, endpoint := range *serviceEndpoints {
// 			endpoints = append(endpoints, *endpoint.Service)
// 		}
// 	}
//
// 	return endpoints
// }

func flattenSubnetIPConfigurations(ipConfigurations *[]network.IPConfiguration) []string {
	ips := make([]string, 0)

	if ipConfigurations != nil {
		for _, ip := range *ipConfigurations {
			ips = append(ips, *ip.ID)
		}
	}

	return ips
}

type SubnetId struct {
	SubscriptionId     string
	ResourceGroup      string
	VirtualNetworkName string
	Name               string
}

func NewSubnetID(subscriptionId, resourceGroup, virtualNetworkName, name string) SubnetId {
	return SubnetId{
		SubscriptionId:     subscriptionId,
		ResourceGroup:      resourceGroup,
		VirtualNetworkName: virtualNetworkName,
		Name:               name,
	}
}

func (id SubnetId) String() string {
	segments := []string{
		fmt.Sprintf("Name %q", id.Name),
		fmt.Sprintf("Virtual Network Name %q", id.VirtualNetworkName),
		fmt.Sprintf("Resource Group %q", id.ResourceGroup),
	}
	segmentsStr := strings.Join(segments, " / ")
	return fmt.Sprintf("%s: (%s)", "Subnet", segmentsStr)
}

func (id SubnetId) ID() string {
	fmtString := "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s"
	return fmt.Sprintf(fmtString, id.SubscriptionId, id.ResourceGroup, id.VirtualNetworkName, id.Name)
}

// subnetIDInsensitively parses an Subnet ID into an SubnetId struct, insensitively
// This should only be used to parse an ID for rewriting, the SubnetID
// method should be used instead for validation etc.
//
// Whilst this may seem strange, this enables Terraform have consistent casing
// which works around issues in Core, whilst handling broken API responses.
func subnetIDInsensitively(input string) (*SubnetId, error) {
	id, err := azure.ParseAzureResourceID(input)
	if err != nil {
		return nil, err
	}

	resourceId := SubnetId{
		SubscriptionId: id.SubscriptionID,
		ResourceGroup:  id.ResourceGroup,
	}

	if resourceId.SubscriptionId == "" {
		return nil, fmt.Errorf("ID was missing the 'subscriptions' element")
	}

	if resourceId.ResourceGroup == "" {
		return nil, fmt.Errorf("ID was missing the 'resourceGroups' element")
	}

	// find the correct casing for the 'virtualNetworks' segment
	virtualNetworksKey := "virtualNetworks"
	for key := range id.Path {
		if strings.EqualFold(key, virtualNetworksKey) {
			virtualNetworksKey = key
			break
		}
	}
	if resourceId.VirtualNetworkName, err = id.PopSegment(virtualNetworksKey); err != nil {
		return nil, err
	}

	// find the correct casing for the 'subnets' segment
	subnetsKey := "subnets"
	for key := range id.Path {
		if strings.EqualFold(key, subnetsKey) {
			subnetsKey = key
			break
		}
	}
	if resourceId.Name, err = id.PopSegment(subnetsKey); err != nil {
		return nil, err
	}

	if err := id.ValidateNoEmptySegments(input); err != nil {
		return nil, err
	}

	return &resourceId, nil
}

// SubnetID parses a Subnet ID into an SubnetId struct
func ParseSubnetID(input string) (*SubnetId, error) {
	id, err := azure.ParseAzureResourceID(input)
	if err != nil {
		return nil, err
	}

	resourceId := SubnetId{
		SubscriptionId: id.SubscriptionID,
		ResourceGroup:  id.ResourceGroup,
	}

	if resourceId.SubscriptionId == "" {
		return nil, fmt.Errorf("ID was missing the 'subscriptions' element")
	}

	if resourceId.ResourceGroup == "" {
		return nil, fmt.Errorf("ID was missing the 'resourceGroups' element")
	}

	if resourceId.VirtualNetworkName, err = id.PopSegment("virtualNetworks"); err != nil {
		return nil, err
	}
	if resourceId.Name, err = id.PopSegment("subnets"); err != nil {
		return nil, err
	}

	if err := id.ValidateNoEmptySegments(input); err != nil {
		return nil, err
	}

	return &resourceId, nil
}
