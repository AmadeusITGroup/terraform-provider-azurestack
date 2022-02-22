package compute

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/compute/mgmt/compute"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-azurestack/internal/az/tags"
	"github.com/hashicorp/terraform-provider-azurestack/internal/clients"
	"github.com/hashicorp/terraform-provider-azurestack/internal/tf/timeouts"
	"github.com/hashicorp/terraform-provider-azurestack/internal/utils"
)

func imageDataSource() *schema.Resource {
	return &schema.Resource{
		Read: imageDataSourceRead,

		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{

			"name_regex": {
				Type:          schema.TypeString,
				Optional:      true,
				ValidateFunc:  validation.StringIsValidRegExp,
				ConflictsWith: []string{"name"},
			},
			"sort_descending": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"name": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"name_regex"},
			},

			"resource_group_name": commonschema.ResourceGroupNameForDataSource(),

			"location": commonschema.LocationOptional(),

			"zone_resilient": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"os_disk": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"blob_uri": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"caching": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"managed_disk_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"os_state": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"os_type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"size_gb": {
							Type:     schema.TypeInt,
							Computed: true,
						},
					},
				},
			},

			"data_disk": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"blob_uri": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"caching": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"lun": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"managed_disk_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"size_gb": {
							Type:     schema.TypeInt,
							Computed: true,
						},
					},
				},
			},

			"tags": tags.Schema(),
		},
	}
}

func imageDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Compute.ImageClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	resGroup := d.Get("resource_group_name").(string)

	name := d.Get("name").(string)
	nameRegex, nameRegexOk := d.GetOk("name_regex")

	if name == "" && !nameRegexOk {
		return fmt.Errorf("either name or name_regex is required")
	}

	var img compute.Image

	if !nameRegexOk {
		var err error
		if img, err = client.Get(ctx, resGroup, name, ""); err != nil {
			if utils.ResponseWasNotFound(img.Response) {
				return fmt.Errorf("image %q was not found in resource group %q", name, resGroup)
			}
			return fmt.Errorf("Error making Read request on Azure Image %q (resource group %q): %+v", name, resGroup, err)
		}
	} else {
		r := regexp.MustCompile(nameRegex.(string))

		list := make([]compute.Image, 0)
		resp, err := client.ListByResourceGroupComplete(ctx, resGroup)
		if err != nil {
			if utils.ResponseWasNotFound(resp.Response().Response) {
				return fmt.Errorf("No Images were found for Resource Group %q", resGroup)
			}
			return fmt.Errorf("Error getting list of images (resource group %q): %+v", resGroup, err)
		}

		for resp.NotDone() {
			img = resp.Value()
			if r.Match(([]byte)(*img.Name)) {
				list = append(list, img)
			}
			err = resp.NextWithContext(ctx)

			if err != nil {
				return err
			}
		}

		if 1 > len(list) {
			return fmt.Errorf("No Images were found for Resource Group %q", resGroup)
		}

		if len(list) > 1 {
			desc := d.Get("sort_descending").(bool)
			log.Printf("Image - multiple results found and `sort_descending` is set to: %t", desc)

			sort.Slice(list, func(i, j int) bool {
				return (!desc && *list[i].Name < *list[j].Name) ||
					(desc && *list[i].Name > *list[j].Name)
			})
		}
		img = list[0]
	}

	d.SetId(*img.ID)
	d.Set("name", img.Name)
	d.Set("resource_group_name", resGroup)
	d.Set("location", location.NormalizeNilable(img.Location))

	if profile := img.StorageProfile; profile != nil {
		if disk := profile.OsDisk; disk != nil {
			if err := d.Set("os_disk", flattenAzureRmImageOSDisk(disk)); err != nil {
				return fmt.Errorf("[DEBUG] Error setting AzureRM Image OS Disk error: %+v", err)
			}
		}

		if disks := profile.DataDisks; disks != nil {
			if err := d.Set("data_disk", flattenAzureRmImageDataDisks(disks)); err != nil {
				return fmt.Errorf("[DEBUG] Error setting AzureRM Image Data Disks error: %+v", err)
			}
		}

		d.Set("zone_resilient", profile.ZoneResilient)
	}

	tags.FlattenAndSet(d, img.Tags)
	return nil
}
