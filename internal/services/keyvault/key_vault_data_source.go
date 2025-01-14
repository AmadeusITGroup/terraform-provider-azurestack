package keyvault

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/keyvault/mgmt/keyvault"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-provider-azurestack/internal/az/tags"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/services/keyvault/validate"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/set"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/timeouts"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

func keyVaultDataSource() *schema.Resource {
	return &schema.Resource{
		Read: keyVaultDataSourceRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: func() map[string]*schema.Schema {
			dsSchema := map[string]*schema.Schema{
				"name": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validate.VaultName,
				},

				"resource_group_name": commonschema.ResourceGroupNameForDataSource(),

				"location": commonschema.LocationComputed(),

				"sku_name": {
					Type:     schema.TypeString,
					Computed: true,
				},

				"vault_uri": {
					Type:     schema.TypeString,
					Computed: true,
				},

				"tenant_id": {
					Type:     schema.TypeString,
					Computed: true,
				},

				"access_policy": {
					Type:     schema.TypeList,
					Computed: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"tenant_id": {
								Type:     schema.TypeString,
								Computed: true,
							},
							"object_id": {
								Type:     schema.TypeString,
								Computed: true,
							},
							"application_id": {
								Type:     schema.TypeString,
								Computed: true,
							},
							"certificate_permissions": {
								Type:     schema.TypeList,
								Computed: true,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
							"key_permissions": {
								Type:     schema.TypeList,
								Computed: true,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
							"secret_permissions": {
								Type:     schema.TypeList,
								Computed: true,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
							"storage_permissions": {
								Type:     schema.TypeList,
								Computed: true,
								Elem: &schema.Schema{
									Type: schema.TypeString,
								},
							},
						},
					},
				},

				"enabled_for_deployment": {
					Type:     schema.TypeBool,
					Computed: true,
				},

				"enabled_for_disk_encryption": {
					Type:     schema.TypeBool,
					Computed: true,
				},

				"enabled_for_template_deployment": {
					Type:     schema.TypeBool,
					Computed: true,
				},

				"network_acls": {
					Type:     schema.TypeList,
					Computed: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"default_action": {
								Type:     schema.TypeString,
								Computed: true,
							},
							"bypass": {
								Type:     schema.TypeString,
								Computed: true,
							},
							"ip_rules": {
								Type:     schema.TypeSet,
								Computed: true,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Set:      schema.HashString,
							},
							"virtual_network_subnet_ids": {
								Type:     schema.TypeSet,
								Computed: true,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Set:      set.HashStringIgnoreCase,
							},
						},
					},
				},

				// "purge_protection_enabled": {
				// 	Type:     schema.TypeBool,
				// 	Computed: true,
				// },

				"tags": tags.SchemaDataSource(),
			}

			return dsSchema
		}(),
	}
}

func keyVaultDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).KeyVault.VaultsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	resp, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			return fmt.Errorf("KeyVault %q (Resource Group %q) does not exist", name, resourceGroup)
		}
		return fmt.Errorf("Error making Read request on KeyVault %q: %+v", name, err)
	}

	d.SetId(*resp.ID)

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resourceGroup)
	d.Set("location", location.NormalizeNilable(resp.Location))

	if props := resp.Properties; props != nil {
		d.Set("tenant_id", props.TenantID.String())
		d.Set("enabled_for_deployment", props.EnabledForDeployment)
		d.Set("enabled_for_disk_encryption", props.EnabledForDiskEncryption)
		d.Set("enabled_for_template_deployment", props.EnabledForTemplateDeployment)
		// d.Set("purge_protection_enabled", props.EnablePurgeProtection)
		d.Set("vault_uri", props.VaultURI)

		if sku := props.Sku; sku != nil {
			if err := d.Set("sku_name", string(sku.Name)); err != nil {
				return fmt.Errorf("Error setting `sku_name` for KeyVault %q: %+v", *resp.Name, err)
			}
		} else {
			return fmt.Errorf("Error making Read request on KeyVault %q: Unable to retrieve 'sku' value", *resp.Name)
		}

		flattenedPolicies := flattenAccessPolicies(props.AccessPolicies)
		if err := d.Set("access_policy", flattenedPolicies); err != nil {
			return fmt.Errorf("Error setting `access_policy` for KeyVault %q: %+v", *resp.Name, err)
		}

		if err := d.Set("network_acls", flattenKeyVaultDataSourceNetworkAcls(props.NetworkAcls)); err != nil {
			return fmt.Errorf("Error setting `network_acls` for KeyVault %q: %+v", *resp.Name, err)
		}
	}

	return tags.FlattenAndSet(d, resp.Tags)
}

func flattenKeyVaultDataSourceNetworkAcls(input *keyvault.NetworkRuleSet) []interface{} {
	if input == nil {
		return []interface{}{}
	}

	output := make(map[string]interface{})

	output["bypass"] = string(input.Bypass)
	output["default_action"] = string(input.DefaultAction)

	ipRules := make([]interface{}, 0)
	if input.IPRules != nil {
		for _, v := range *input.IPRules {
			if v.Value == nil {
				continue
			}

			ipRules = append(ipRules, *v.Value)
		}
	}
	output["ip_rules"] = schema.NewSet(schema.HashString, ipRules)

	virtualNetworkRules := make([]interface{}, 0)
	if input.VirtualNetworkRules != nil {
		for _, v := range *input.VirtualNetworkRules {
			if v.ID == nil {
				continue
			}

			virtualNetworkRules = append(virtualNetworkRules, *v.ID)
		}
	}
	output["virtual_network_subnet_ids"] = schema.NewSet(schema.HashString, virtualNetworkRules)

	return []interface{}{output}
}
