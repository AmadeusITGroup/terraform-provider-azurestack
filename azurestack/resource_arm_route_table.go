package azurestack

import (
	"fmt"
	"log"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-10-01/network"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/hashicorp/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/azurerm/utils"
	"github.com/hashicorp/terraform-provider-azurestack/azurestack/helpers/response"
)

var routeTableResourceName = "azurestack_route_table"

func resourceArmRouteTable() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmRouteTableCreateUpdate,
		Read:   resourceArmRouteTableRead,
		Update: resourceArmRouteTableCreateUpdate,
		Delete: resourceArmRouteTableDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},

			"location": locationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"route": {
				Type:       schema.TypeList,
				Optional:   true,
				Computed:   true,
				ConfigMode: schema.SchemaConfigModeAttr,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.NoZeroValues,
						},

						"address_prefix": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.NoZeroValues,
						},

						"next_hop_type": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								string(network.RouteNextHopTypeVirtualNetworkGateway),
								string(network.RouteNextHopTypeVnetLocal),
								string(network.RouteNextHopTypeInternet),
								string(network.RouteNextHopTypeVirtualAppliance),
								string(network.RouteNextHopTypeNone),
							}, true),
							DiffSuppressFunc: ignoreCaseDiffSuppressFunc,
						},

						"next_hop_in_ip_address": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.NoZeroValues,
						},
					},
				},
			},

			"subnets": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"tags": tagsSchema(),
		},
	}
}

func resourceArmRouteTableCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).routeTablesClient
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[INFO] preparing arguments for AzureStack Route Table creation.")

	name := d.Get("name").(string)
	location := azureStackNormalizeLocation(d.Get("location").(string))
	resGroup := d.Get("resource_group_name").(string)
	tags := d.Get("tags").(map[string]interface{})

	routes := expandRouteTableRoutes(d)

	routeSet := network.RouteTable{
		Name:     &name,
		Location: &location,
		RouteTablePropertiesFormat: &network.RouteTablePropertiesFormat{
			Routes: &routes,
		},
		Tags: *expandTags(tags),
	}

	future, err := client.CreateOrUpdate(ctx, resGroup, name, routeSet)
	if err != nil {
		return fmt.Errorf("Creating/Updating Route Table %q (Resource Group %q): %+v", name, resGroup, err)
	}

	if err := future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for completion of Route Table %q (Resource Group %q): %+v", name, resGroup, err)
	}

	read, err := client.Get(ctx, resGroup, name, "")
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read Route Table %q (resource group %q) ID", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmRouteTableRead(d, meta)
}

func resourceArmRouteTableRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).routeTablesClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	name := id.Path["routeTables"]

	resp, err := client.Get(ctx, resGroup, name, "")
	if err != nil {
		if response.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("making Read request on Azure Route Table %q: %+v", name, err)
	}

	d.Set("name", name)
	d.Set("resource_group_name", resGroup)
	if location := resp.Location; location != nil {
		d.Set("location", azureStackNormalizeLocation(*location))
	}

	if props := resp.RouteTablePropertiesFormat; props != nil {
		if err := d.Set("route", flattenRouteTableRoutes(props.Routes)); err != nil {
			return err
		}

		if err := d.Set("subnets", flattenRouteTableSubnets(props.Subnets)); err != nil {
			return err
		}
	}

	flattenAndSetTags(d, &resp.Tags)

	return nil
}

func resourceArmRouteTableDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).routeTablesClient
	ctx := meta.(*ArmClient).StopContext

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	name := id.Path["routeTables"]

	future, err := client.Delete(ctx, resGroup, name)
	if err != nil {
		if !response.WasNotFound(future.Response()) {
			return fmt.Errorf("deleting Route Table %q (Resource Group %q): %+v", name, resGroup, err)
		}
	}

	if err := future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for deletion of Route Table %q (Resource Group %q): %+v", name, resGroup, err)
	}

	return nil
}

func expandRouteTableRoutes(d *schema.ResourceData) []network.Route {
	configs := d.Get("route").([]interface{})
	routes := make([]network.Route, 0, len(configs))

	for _, configRaw := range configs {
		data := configRaw.(map[string]interface{})

		name := data["name"].(string)
		route := network.Route{
			Name: &name,
			RoutePropertiesFormat: &network.RoutePropertiesFormat{
				AddressPrefix: utils.String(data["address_prefix"].(string)),
				NextHopType:   network.RouteNextHopType(data["next_hop_type"].(string)),
			},
		}

		if v := data["next_hop_in_ip_address"].(string); v != "" {
			route.RoutePropertiesFormat.NextHopIPAddress = &v
		}

		routes = append(routes, route)
	}

	return routes
}

func flattenRouteTableRoutes(input *[]network.Route) []interface{} {
	results := make([]interface{}, 0)

	if routes := input; routes != nil {
		for _, route := range *routes {
			r := make(map[string]interface{})

			r["name"] = *route.Name

			if props := route.RoutePropertiesFormat; props != nil {
				r["address_prefix"] = *props.AddressPrefix
				r["next_hop_type"] = string(props.NextHopType)
				if ip := props.NextHopIPAddress; ip != nil {
					r["next_hop_in_ip_address"] = *ip
				}
			}

			results = append(results, r)
		}
	}

	return results
}

func flattenRouteTableSubnets(subnets *[]network.Subnet) []string {
	output := []string{}

	if subnets != nil {
		for _, subnet := range *subnets {
			output = append(output, *subnet.ID)
		}
	}

	return output
}

type RouteTableId struct {
	SubscriptionId string
	ResourceGroup  string
	Name           string
}

func NewRouteTableID(subscriptionId, resourceGroup, name string) RouteTableId {
	return RouteTableId{
		SubscriptionId: subscriptionId,
		ResourceGroup:  resourceGroup,
		Name:           name,
	}
}

func (id RouteTableId) String() string {
	segments := []string{
		fmt.Sprintf("Name %q", id.Name),
		fmt.Sprintf("Resource Group %q", id.ResourceGroup),
	}
	segmentsStr := strings.Join(segments, " / ")
	return fmt.Sprintf("%s: (%s)", "Route Table", segmentsStr)
}

func (id RouteTableId) ID() string {
	fmtString := "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s"
	return fmt.Sprintf(fmtString, id.SubscriptionId, id.ResourceGroup, id.Name)
}

// ParseRouteTableID parses a RouteTable ID into an RouteTableId struct
func ParseRouteTableID(input string) (*RouteTableId, error) {
	id, err := azure.ParseAzureResourceID(input)
	if err != nil {
		return nil, err
	}

	resourceId := RouteTableId{
		SubscriptionId: id.SubscriptionID,
		ResourceGroup:  id.ResourceGroup,
	}

	if resourceId.SubscriptionId == "" {
		return nil, fmt.Errorf("ID was missing the 'subscriptions' element")
	}

	if resourceId.ResourceGroup == "" {
		return nil, fmt.Errorf("ID was missing the 'resourceGroups' element")
	}

	if resourceId.Name, err = id.PopSegment("routeTables"); err != nil {
		return nil, err
	}

	if err := id.ValidateNoEmptySegments(input); err != nil {
		return nil, err
	}

	return &resourceId, nil
}
