package azurestack

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	KeyVaultMgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2019-09-01/keyvault"
	"github.com/hashicorp/go-azure-helpers/response"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	uuid "github.com/satori/go.uuid"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	commonValidate "github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/validate"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
	localAzure "github.com/terraform-providers/terraform-provider-azurestack/azurestack/helpers/azure"
)

// As can be seen in the API definition, the Sku Family only supports the value
// `A` and is a required field
// https://github.com/Azure/azure-rest-api-specs/blob/master/arm-keyvault/2015-06-01/swagger/keyvault.json#L239
var armKeyVaultSkuFamily = "A"

var keyVaultResourceName = "azurestack_key_vault"

func resourceArmKeyVault() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmKeyVaultCreate,
		Read:   resourceArmKeyVaultRead,
		Update: resourceArmKeyVaultUpdate,
		Delete: resourceArmKeyVaultDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		// NOTE: origin
		// TODO: uncomment
		// MigrateState: resourceArmKeyVaultMigrateState,
		// StateUpgraders: []schema.StateUpgrader{
		// 	migration.KeyVaultV1ToV2Upgrader(),
		// },
		SchemaVersion: 2,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: func() map[string]*schema.Schema {
			rSchema := map[string]*schema.Schema{
				"name": {
					Type:         schema.TypeString,
					Required:     true,
					ForceNew:     true,
					ValidateFunc: localAzure.VaultName,
				},

				// TODO: instead use azure.SchemaLocation()
				"location": locationSchema(),

				// TODO: instead use azure.SchemaResourceGroupName()
				"resource_group_name": resourceGroupNameSchema(),

				"sku_name": {
					Type:     schema.TypeString,
					Required: true,
					ValidateFunc: validation.StringInSlice([]string{
						string(keyvault.Standard),
						string(keyvault.Premium),
					}, false),
				},

				"tenant_id": {
					Type:         schema.TypeString,
					Required:     true,
					ValidateFunc: validation.IsUUID,
				},

				"access_policy": {
					Type:       schema.TypeList,
					ConfigMode: schema.SchemaConfigModeAttr,
					Optional:   true,
					Computed:   true,
					MaxItems:   1024,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"tenant_id": {
								Type:         schema.TypeString,
								Required:     true,
								ValidateFunc: validation.IsUUID,
							},
							"object_id": {
								Type:         schema.TypeString,
								Required:     true,
								ValidateFunc: validation.IsUUID,
							},
							"application_id": {
								Type:         schema.TypeString,
								Optional:     true,
								ValidateFunc: validation.Any(validation.IsUUID, validation.StringIsEmpty),
							},
							"certificate_permissions": schemaCertificatePermissions(),
							"key_permissions":         schemaKeyPermissions(),
							"secret_permissions":      schemaSecretPermissions(),
							"storage_permissions":     schemaStoragePermissions(),
						},
					},
				},

				"enabled_for_deployment": {
					Type:     schema.TypeBool,
					Optional: true,
				},

				"enabled_for_disk_encryption": {
					Type:     schema.TypeBool,
					Optional: true,
				},

				"enabled_for_template_deployment": {
					Type:     schema.TypeBool,
					Optional: true,
				},

				"enable_rbac_authorization": {
					Type:     schema.TypeBool,
					Optional: true,
				},

				"network_acls": {
					Type:     schema.TypeList,
					Optional: true,
					Computed: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"default_action": {
								Type:     schema.TypeString,
								Required: true,
								ValidateFunc: validation.StringInSlice([]string{
									string(keyvault.Allow),
									string(keyvault.Deny),
								}, false),
							},
							"bypass": {
								Type:     schema.TypeString,
								Required: true,
								ValidateFunc: validation.StringInSlice([]string{
									string(keyvault.None),
									string(keyvault.AzureServices),
								}, false),
							},
							"ip_rules": {
								Type:     schema.TypeSet,
								Optional: true,
								Elem: &schema.Schema{
									Type: schema.TypeString,
									ValidateFunc: validation.Any(
										commonValidate.IPv4Address,
										commonValidate.CIDR,
									),
								},
								Set: HashIPv4AddressOrCIDR,
							},
							"virtual_network_subnet_ids": {
								Type:     schema.TypeSet,
								Optional: true,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Set:      HashStringIgnoreCase,
							},
						},
					},
				},

				"purge_protection_enabled": {
					Type:     schema.TypeBool,
					Optional: true,
				},

				"soft_delete_retention_days": {
					Type:         schema.TypeInt,
					Optional:     true,
					Default:      90,
					ValidateFunc: validation.IntBetween(7, 90),
				},

				"contact": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"email": {
								Type:     schema.TypeString,
								Required: true,
							},
							"name": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"phone": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},

				"tags": tagsSchema(),

				// Computed
				"vault_uri": {
					Type:     schema.TypeString,
					Computed: true,
				},
			}

			// NOTE: we disable these following lines due to depreciation
			// if !features.ThreePointOh() {
			// 	rSchema["soft_delete_enabled"] = &schema.Schema{
			// 		Type:       schema.TypeBool,
			// 		Optional:   true,
			// 		Computed:   true,
			// 		Deprecated: `Azure has removed support for disabling Soft Delete as of 2020-12-15, as such this field is no longer configurable and can be safely removed. This field will be removed in version 3.0 of the Azure Provider.`,
			// 	}
			// }

			return rSchema
		}(),
	}
}

func resourceArmKeyVaultCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultClient
	subscriptionId := meta.(*ArmClient).subscriptionId
	dataPlaneClient := meta.(*ArmClient).keyVaultMgmtClient

	ctx, cancel := ForCreate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id := newVaultID(subscriptionId, d.Get("resource_group_name").(string), d.Get("name").(string))
	location := azure.NormalizeLocation(d.Get("location").(string))

	// Locking this resource so we don't make modifications to it at the same time if there is a
	// key vault access policy trying to update it as well
	azureStackLockByName(id.Name, keyVaultResourceName)
	defer azureStackUnlockByName(id.Name, keyVaultResourceName)

	// check for the presence of an existing, live one which should be imported into the state
	existing, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if !utils.ResponseWasNotFound(existing.Response) {
			return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
		}
	}

	if !utils.ResponseWasNotFound(existing.Response) {
		return tf.ImportAsExistsError("azurerm_key_vault", id.ID())
	}

	// before creating check to see if the key vault exists in the soft delete state
	softDeletedKeyVault, err := client.GetDeleted(ctx, id.Name, location)
	if err != nil {
		// If Terraform lacks permission to read at the Subscription we'll get 409, not 404
		if !utils.ResponseWasNotFound(softDeletedKeyVault.Response) && !utils.ResponseWasForbidden(softDeletedKeyVault.Response) {
			return fmt.Errorf("checking for the presence of an existing Soft-Deleted Key Vault %q (Location %q): %+v", id.Name, location, err)
		}
	}

	// NOTE: we assume that the provider do not support features
	// if so, does the user want us to recover it?

	recoverSoftDeletedKeyVault := false
	// if !utils.ResponseWasNotFound(softDeletedKeyVault.Response) && !utils.ResponseWasForbidden(softDeletedKeyVault.Response) {
	// 	if !meta.(*ArmClient).Features.KeyVault.RecoverSoftDeletedKeyVaults {
	// 		// this exists but the users opted out so they must import this it out-of-band
	// 		return fmt.Errorf(optedOutOfRecoveringSoftDeletedKeyVaultErrorFmt(id.Name, location))
	// 	}

	// 	recoverSoftDeletedKeyVault = true
	// }

	tenantUUID := uuid.FromStringOrNil(d.Get("tenant_id").(string))
	enabledForDeployment := d.Get("enabled_for_deployment").(bool)
	enabledForDiskEncryption := d.Get("enabled_for_disk_encryption").(bool)
	enabledForTemplateDeployment := d.Get("enabled_for_template_deployment").(bool)
	enableRbacAuthorization := d.Get("enable_rbac_authorization").(bool)
	t := d.Get("tags").(map[string]interface{})

	policies := d.Get("access_policy").([]interface{})
	accessPolicies := expandAccessPolicies(policies)

	networkAclsRaw := d.Get("network_acls").([]interface{})
	networkAcls, subnetIds := expandKeyVaultNetworkAcls(networkAclsRaw)

	sku := keyvault.Sku{
		Family: &armKeyVaultSkuFamily,
		Name:   keyvault.SkuName(d.Get("sku_name").(string)),
	}

	parameters := keyvault.VaultCreateOrUpdateParameters{
		Location: &location,
		Properties: &keyvault.VaultProperties{
			TenantID:                     &tenantUUID,
			Sku:                          &sku,
			AccessPolicies:               accessPolicies,
			EnabledForDeployment:         &enabledForDeployment,
			EnabledForDiskEncryption:     &enabledForDiskEncryption,
			EnabledForTemplateDeployment: &enabledForTemplateDeployment,
			EnableRbacAuthorization:      &enableRbacAuthorization,
			NetworkAcls:                  networkAcls,

			// @tombuildsstuff: as of 2020-12-15 this is now defaulted on, and appears to be so in all regions
			// This has been confirmed in Azure Public and Azure China - but I couldn't find any more
			// documentation with further details
			// NOTE: In AzureStackHub, Azure keyvault will not work properly if this feature is enabled. That's why, we decided to disable it.
			EnableSoftDelete: utils.Bool(false),
		},
		// NOTE: origin
		// Tags: tags.Expand(t),
		Tags: *expandTags(t),
	}

	if purgeProtectionEnabled := d.Get("purge_protection_enabled").(bool); purgeProtectionEnabled {
		parameters.Properties.EnablePurgeProtection = utils.Bool(purgeProtectionEnabled)
	}

	if v := d.Get("soft_delete_retention_days"); v != 90 {
		parameters.Properties.SoftDeleteRetentionInDays = utils.Int32(int32(v.(int)))
	}

	if recoverSoftDeletedKeyVault {
		parameters.Properties.CreateMode = keyvault.CreateModeRecover
	}

	// also lock on the Virtual Network ID's since modifications in the networking stack are exclusive
	virtualNetworkNames := make([]string, 0)
	for _, v := range subnetIds {
		id, err := subnetIDInsensitively(v)
		if err != nil {
			return err
		}
		if !utils.SliceContainsValue(virtualNetworkNames, id.VirtualNetworkName) {
			virtualNetworkNames = append(virtualNetworkNames, id.VirtualNetworkName)
		}
	}

	azureStackLockMultipleByName(&virtualNetworkNames, virtualNetworkResourceName)
	defer azureStackUnlockMultipleByName(&virtualNetworkNames, virtualNetworkResourceName)

	if _, err := client.CreateOrUpdate(ctx, id.ResourceGroup, id.Name, parameters); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	read, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		return fmt.Errorf("retrieving %s: %+v", id, err)
	}
	if read.Properties == nil || read.Properties.VaultURI == nil {
		return fmt.Errorf("retrieving %s: `properties.VaultUri` was nil", id)
	}
	d.SetId(id.ID())
	meta.(*ArmClient).AddKeyVaultToCache(id, *read.Properties.VaultURI)

	if props := read.Properties; props != nil {
		if vault := props.VaultURI; vault != nil {
			log.Printf("[DEBUG] Waiting for %s to become available", id)
			stateConf := &resource.StateChangeConf{
				Pending:                   []string{"pending"},
				Target:                    []string{"available"},
				Refresh:                   keyVaultRefreshFunc(*vault),
				Delay:                     30 * time.Second,
				PollInterval:              10 * time.Second,
				ContinuousTargetOccurence: 10,
				Timeout:                   d.Timeout(schema.TimeoutCreate),
			}

			if _, err := stateConf.WaitForState(); err != nil {
				return fmt.Errorf("Error waiting for %s to become available: %s", id, err)
			}
		}
	}

	if v, ok := d.GetOk("contact"); ok {
		contacts := KeyVaultMgmt.Contacts{
			ContactList: expandKeyVaultCertificateContactList(v.(*schema.Set).List()),
		}
		if read.Properties == nil || read.Properties.VaultURI == nil {
			return fmt.Errorf("failed to get vault base url for %s: %s", id, err)
		}
		if _, err := dataPlaneClient.SetCertificateContacts(ctx, *read.Properties.VaultURI, contacts); err != nil {
			return fmt.Errorf("failed to set Contacts for %s: %+v", id, err)
		}
	}

	return resourceArmKeyVaultRead(d, meta)
}

func resourceArmKeyVaultUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultClient
	managementClient := meta.(*ArmClient).keyVaultMgmtClient

	ctx, cancel := ForUpdate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := VaultID(d.Id())
	if err != nil {
		return err
	}

	// Locking this resource so we don't make modifications to it at the same time if there is a
	// key vault access policy trying to update it as well
	azureStackLockByName(id.Name, keyVaultResourceName)
	defer azureStackUnlockByName(id.Name, keyVaultResourceName)

	d.Partial(true)

	// first pull the existing key vault since we need to lock on several bits of its information
	existing, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}
	if existing.Properties == nil {
		return fmt.Errorf("retrieving %s: `properties` was nil", *id)
	}

	update := keyvault.VaultPatchParameters{}

	if d.HasChange("access_policy") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		policiesRaw := d.Get("access_policy").([]interface{})
		accessPolicies := expandAccessPolicies(policiesRaw)
		update.Properties.AccessPolicies = accessPolicies
	}

	if d.HasChange("enabled_for_deployment") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		update.Properties.EnabledForDeployment = utils.Bool(d.Get("enabled_for_deployment").(bool))
	}

	if d.HasChange("enabled_for_disk_encryption") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		update.Properties.EnabledForDiskEncryption = utils.Bool(d.Get("enabled_for_disk_encryption").(bool))
	}

	if d.HasChange("enabled_for_template_deployment") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		update.Properties.EnabledForTemplateDeployment = utils.Bool(d.Get("enabled_for_template_deployment").(bool))
	}

	if d.HasChange("enable_rbac_authorization") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		update.Properties.EnableRbacAuthorization = utils.Bool(d.Get("enable_rbac_authorization").(bool))
	}

	if d.HasChange("network_acls") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		networkAclsRaw := d.Get("network_acls").([]interface{})
		networkAcls, subnetIds := expandKeyVaultNetworkAcls(networkAclsRaw)

		// also lock on the Virtual Network ID's since modifications in the networking stack are exclusive
		virtualNetworkNames := make([]string, 0)
		for _, v := range subnetIds {
			id, err := subnetIDInsensitively(v)
			if err != nil {
				return err
			}

			if !utils.SliceContainsValue(virtualNetworkNames, id.VirtualNetworkName) {
				virtualNetworkNames = append(virtualNetworkNames, id.VirtualNetworkName)
			}
		}

		azureStackLockMultipleByName(&virtualNetworkNames, virtualNetworkResourceName)
		defer azureStackUnlockMultipleByName(&virtualNetworkNames, virtualNetworkResourceName)

		update.Properties.NetworkAcls = networkAcls
	}

	if d.HasChange("purge_protection_enabled") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		newValue := d.Get("purge_protection_enabled").(bool)

		// existing.Properties guaranteed non-nil above
		oldValue := false
		if existing.Properties.EnablePurgeProtection != nil {
			oldValue = *existing.Properties.EnablePurgeProtection
		}

		// whilst this should have got caught in the customizeDiff this won't work if that fields interpolated
		// hence the double-checking here
		if oldValue && !newValue {
			return fmt.Errorf("updating %s: once Purge Protection has been Enabled it's not possible to disable it", *id)
		}

		update.Properties.EnablePurgeProtection = utils.Bool(newValue)
	}

	if d.HasChange("sku_name") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		update.Properties.Sku = &keyvault.Sku{
			Family: &armKeyVaultSkuFamily,
			Name:   keyvault.SkuName(d.Get("sku_name").(string)),
		}
	}

	if d.HasChange("soft_delete_retention_days") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		// existing.Properties guaranteed non-nil above
		var oldValue int32 = 0
		if existing.Properties.SoftDeleteRetentionInDays != nil {
			oldValue = *existing.Properties.SoftDeleteRetentionInDays
		}

		// whilst this should have got caught in the customizeDiff this won't work if that fields interpolated
		// hence the double-checking here
		if oldValue != 0 {
			// Code="BadRequest" Message="The property \"softDeleteRetentionInDays\" has been set already and it can't be modified."
			return fmt.Errorf("updating %s: once `soft_delete_retention_days` has been configured it cannot be modified", *id)
		}

		update.Properties.SoftDeleteRetentionInDays = utils.Int32(int32(d.Get("soft_delete_retention_days").(int)))
	}

	if d.HasChange("tenant_id") {
		if update.Properties == nil {
			update.Properties = &keyvault.VaultPatchProperties{}
		}

		tenantUUID := uuid.FromStringOrNil(d.Get("tenant_id").(string))
		update.Properties.TenantID = &tenantUUID
	}

	if d.HasChange("tags") {
		t := d.Get("tags").(map[string]interface{})
		// NOTE: origin
		// update.Tags = tags.Expand(t)
		update.Tags = *expandTags(t)
	}

	if _, err := client.Update(ctx, id.ResourceGroup, id.Name, update); err != nil {
		return fmt.Errorf("updating %s: %+v", *id, err)
	}

	if d.HasChange("contact") {
		contacts := KeyVaultMgmt.Contacts{
			ContactList: expandKeyVaultCertificateContactList(d.Get("contact").(*schema.Set).List()),
		}
		if existing.Properties == nil || existing.Properties.VaultURI == nil {
			return fmt.Errorf("failed to get vault base url for %s: %s", *id, err)
		}

		var err error
		if len(*contacts.ContactList) == 0 {
			_, err = managementClient.DeleteCertificateContacts(ctx, *existing.Properties.VaultURI)
		} else {
			_, err = managementClient.SetCertificateContacts(ctx, *existing.Properties.VaultURI, contacts)
		}

		if err != nil {
			return fmt.Errorf("setting Contacts for %s: %+v", *id, err)
		}
	}

	d.Partial(false)

	return resourceArmKeyVaultRead(d, meta)
}

func resourceArmKeyVaultRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultClient
	managementClient := meta.(*ArmClient).keyVaultMgmtClient

	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := VaultID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[DEBUG] %s was not found - removing from state!", *id)
			d.SetId("")
			return nil
		}
		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}
	if resp.Properties == nil {
		return fmt.Errorf("retrieving %s: `properties` was nil", *id)
	}
	if resp.Properties.VaultURI == nil {
		return fmt.Errorf("retrieving %s: `properties.VaultUri` was nil", *id)
	}

	props := *resp.Properties
	meta.(*ArmClient).AddKeyVaultToCache(*id, *resp.Properties.VaultURI)

	d.Set("name", id.Name)
	d.Set("resource_group_name", id.ResourceGroup)
	d.Set("location", azureStackNormalizeLocation(*resp.Location))

	d.Set("tenant_id", props.TenantID.String())
	d.Set("enabled_for_deployment", props.EnabledForDeployment)
	d.Set("enabled_for_disk_encryption", props.EnabledForDiskEncryption)
	d.Set("enabled_for_template_deployment", props.EnabledForTemplateDeployment)
	d.Set("enable_rbac_authorization", props.EnableRbacAuthorization)
	d.Set("purge_protection_enabled", props.EnablePurgeProtection)
	d.Set("vault_uri", props.VaultURI)

	// @tombuildsstuff: the API doesn't return this field if it's not configured
	// however https://docs.microsoft.com/en-us/azure/key-vault/general/soft-delete-overview
	// defaults this to 90 days, as such we're going to have to assume that for the moment
	// in lieu of anything being returned
	softDeleteRetentionDays := 90
	if props.SoftDeleteRetentionInDays != nil && *props.SoftDeleteRetentionInDays != 0 {
		softDeleteRetentionDays = int(*props.SoftDeleteRetentionInDays)
	}
	d.Set("soft_delete_retention_days", softDeleteRetentionDays)

	// TODO: remove in 3.0
	// if !features.ThreePointOh() {
	// 	d.Set("soft_delete_enabled", true)
	// }

	skuName := ""
	if sku := props.Sku; sku != nil {
		// the Azure API is inconsistent here, so rewrite this into the casing we expect
		for _, v := range keyvault.PossibleSkuNameValues() {
			if strings.EqualFold(string(v), string(sku.Name)) {
				skuName = string(v)
			}
		}
	}
	d.Set("sku_name", skuName)

	if err := d.Set("network_acls", flattenKeyVaultNetworkAcls(props.NetworkAcls)); err != nil {
		return fmt.Errorf("setting `network_acls` for KeyVault %q: %+v", *resp.Name, err)
	}

	flattenedPolicies := flattenAccessPolicies(props.AccessPolicies)
	if err := d.Set("access_policy", flattenedPolicies); err != nil {
		return fmt.Errorf("setting `access_policy` for KeyVault %q: %+v", *resp.Name, err)
	}

	contactsResp, err := managementClient.GetCertificateContacts(ctx, *props.VaultURI)
	if err != nil {
		if !utils.ResponseWasForbidden(contactsResp.Response) && !utils.ResponseWasNotFound(contactsResp.Response) {
			return fmt.Errorf("retrieving `contact` for KeyVault: %+v", err)
		}
	}
	if err := d.Set("contact", flattenKeyVaultCertificateContactList(contactsResp)); err != nil {
		return fmt.Errorf("setting `contact` for KeyVault: %+v", err)
	}

	flattenAndSetTags(d, &resp.Tags)

	return nil
}

func resourceArmKeyVaultDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultClient

	ctx, cancel := ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := VaultID(d.Id())
	if err != nil {
		return err
	}

	azureStackLockByName(id.Name, keyVaultResourceName)
	defer azureStackUnlockByName(id.Name, keyVaultResourceName)

	read, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(read.Response) {
			return nil
		}

		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	if read.Properties == nil {
		return fmt.Errorf("retrieving %q: `properties` was nil", *id)
	}
	if read.Location == nil {
		return fmt.Errorf("retrieving %q: `location` was nil", *id)
	}

	// Check to see if purge protection is enabled or not...
	// purgeProtectionEnabled := false
	// if ppe := read.Properties.EnablePurgeProtection; ppe != nil {
	// 	purgeProtectionEnabled = *ppe
	// }
	// softDeleteEnabled := false
	// if sde := read.Properties.EnableSoftDelete; sde != nil {
	// 	softDeleteEnabled = *sde
	// }

	// ensure we lock on the latest network names, to ensure we handle Azure's networking layer being limited to one change at a time
	virtualNetworkNames := make([]string, 0)
	if props := read.Properties; props != nil {
		if acls := props.NetworkAcls; acls != nil {
			if rules := acls.VirtualNetworkRules; rules != nil {
				for _, v := range *rules {
					if v.ID == nil {
						continue
					}

					subnetId, err := subnetIDInsensitively(*v.ID)
					if err != nil {
						return err
					}

					if !utils.SliceContainsValue(virtualNetworkNames, subnetId.VirtualNetworkName) {
						virtualNetworkNames = append(virtualNetworkNames, subnetId.VirtualNetworkName)
					}
				}
			}
		}
	}

	azureStackLockMultipleByName(&virtualNetworkNames, virtualNetworkResourceName)
	defer azureStackUnlockMultipleByName(&virtualNetworkNames, virtualNetworkResourceName)

	resp, err := client.Delete(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if !response.WasNotFound(resp.Response) {
			return fmt.Errorf("retrieving %s: %+v", *id, err)
		}
	}

	// NOTE: we assume that the provider do not support features
	// Purge the soft deleted key vault permanently if the feature flag is enabled
	// if meta.(*ArmClient).Features.KeyVault.PurgeSoftDeleteOnDestroy && softDeleteEnabled {
	// 	// KeyVaults with Purge Protection Enabled cannot be deleted unless done by Azure
	// 	if purgeProtectionEnabled {
	// 		deletedInfo, err := getSoftDeletedStateForKeyVault(ctx, client, id.Name, *read.Location)
	// 		if err != nil {
	// 			return fmt.Errorf("retrieving the Deletion Details for %s: %+v", *id, err)
	// 		}

	// 		// in the future it'd be nice to raise a warning, but this is the best we can do for now
	// 		if deletedInfo != nil {
	// 			log.Printf("[DEBUG] The Key Vault %q has Purge Protection Enabled and was deleted on %q. Azure will purge this on %q", id.Name, deletedInfo.deleteDate, deletedInfo.purgeDate)
	// 		} else {
	// 			log.Printf("[DEBUG] The Key Vault %q has Purge Protection Enabled and will be purged automatically by Azure", id.Name)
	// 		}
	// 		return nil
	// 	}

	// 	log.Printf("[DEBUG] KeyVault %q marked for purge - executing purge", id.Name)
	// 	future, err := client.PurgeDeleted(ctx, id.Name, *read.Location)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	log.Printf("[DEBUG] Waiting for purge of KeyVault %q..", id.Name)
	// 	err = future.WaitForCompletionRef(ctx, client.Client)
	// 	if err != nil {
	// 		return fmt.Errorf("purging %s: %+v", *id, err)
	// 	}
	// 	log.Printf("[DEBUG] Purged KeyVault %q.", id.Name)
	// }

	meta.(*ArmClient).PurgeKeyVaultCache(*id)

	return nil
}

func keyVaultRefreshFunc(vaultUri string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		log.Printf("[DEBUG] Checking to see if KeyVault %q is available..", vaultUri)

		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}

		conn, err := client.Get(vaultUri)
		if err != nil {
			log.Printf("[DEBUG] Didn't find KeyVault at %q", vaultUri)
			return nil, "pending", fmt.Errorf("Error connecting to %q: %s", vaultUri, err)
		}

		defer conn.Body.Close()

		log.Printf("[DEBUG] Found KeyVault at %q", vaultUri)
		return "available", "available", nil
	}
}

func expandKeyVaultNetworkAcls(input []interface{}) (*keyvault.NetworkRuleSet, []string) {
	subnetIds := make([]string, 0)
	if len(input) == 0 {
		return nil, subnetIds
	}

	v := input[0].(map[string]interface{})

	bypass := v["bypass"].(string)
	defaultAction := v["default_action"].(string)

	ipRulesRaw := v["ip_rules"].(*schema.Set)
	ipRules := make([]keyvault.IPRule, 0)

	for _, v := range ipRulesRaw.List() {
		rule := keyvault.IPRule{
			Value: utils.String(v.(string)),
		}
		ipRules = append(ipRules, rule)
	}

	networkRulesRaw := v["virtual_network_subnet_ids"].(*schema.Set)
	networkRules := make([]keyvault.VirtualNetworkRule, 0)
	for _, v := range networkRulesRaw.List() {
		rawId := v.(string)
		subnetIds = append(subnetIds, rawId)
		rule := keyvault.VirtualNetworkRule{
			ID: utils.String(rawId),
		}
		networkRules = append(networkRules, rule)
	}

	ruleSet := keyvault.NetworkRuleSet{
		Bypass:              keyvault.NetworkRuleBypassOptions(bypass),
		DefaultAction:       keyvault.NetworkRuleAction(defaultAction),
		IPRules:             &ipRules,
		VirtualNetworkRules: &networkRules,
	}
	return &ruleSet, subnetIds
}

func expandKeyVaultCertificateContactList(input []interface{}) *[]KeyVaultMgmt.Contact {
	results := make([]KeyVaultMgmt.Contact, 0)
	if len(input) == 0 || input[0] == nil {
		return &results
	}

	for _, item := range input {
		v := item.(map[string]interface{})
		results = append(results, KeyVaultMgmt.Contact{
			Name:         utils.String(v["name"].(string)),
			EmailAddress: utils.String(v["email"].(string)),
			Phone:        utils.String(v["phone"].(string)),
		})
	}

	return &results
}

func flattenKeyVaultNetworkAcls(input *keyvault.NetworkRuleSet) []interface{} {
	if input == nil {
		return []interface{}{
			map[string]interface{}{
				"bypass":                     string(keyvault.AzureServices),
				"default_action":             string(keyvault.Allow),
				"ip_rules":                   schema.NewSet(schema.HashString, []interface{}{}),
				"virtual_network_subnet_ids": schema.NewSet(schema.HashString, []interface{}{}),
			},
		}
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

			id := *v.ID
			subnetId, err := subnetIDInsensitively(*v.ID)
			if err == nil {
				id = subnetId.ID()
			}

			virtualNetworkRules = append(virtualNetworkRules, id)
		}
	}
	output["virtual_network_subnet_ids"] = schema.NewSet(schema.HashString, virtualNetworkRules)

	return []interface{}{output}
}

func flattenKeyVaultCertificateContactList(input KeyVaultMgmt.Contacts) []interface{} {
	results := make([]interface{}, 0)
	if input.ContactList == nil {
		return results
	}

	for _, contact := range *input.ContactList {
		emailAddress := ""
		if contact.EmailAddress != nil {
			emailAddress = *contact.EmailAddress
		}

		name := ""
		if contact.Name != nil {
			name = *contact.Name
		}

		phone := ""
		if contact.Phone != nil {
			phone = *contact.Phone
		}

		results = append(results, map[string]interface{}{
			"email": emailAddress,
			"name":  name,
			"phone": phone,
		})
	}

	return results
}

// func optedOutOfRecoveringSoftDeletedKeyVaultErrorFmt(name, location string) string {
// 	return fmt.Sprintf(`
// An existing soft-deleted Key Vault exists with the Name %q in the location %q, however
// automatically recovering this KeyVault has been disabled via the "features" block.

// Terraform can automatically recover the soft-deleted Key Vault when this behaviour is
// enabled within the "features" block (located within the "provider" block) - more
// information can be found here:

// https://www.terraform.io/docs/providers/azurerm/index.html#features

// Alternatively you can manually recover this (e.g. using the Azure CLI) and then import
// this into Terraform via "terraform import", or pick a different name/location.
// `, name, location)
// }

// type keyVaultDeletionStatus struct {
// 	deleteDate string
// 	purgeDate  string
// }

// func getSoftDeletedStateForKeyVault(ctx context.Context, client *keyvault.VaultsClient, name string, location string) (*keyVaultDeletionStatus, error) {
// 	softDel, err := client.GetDeleted(ctx, name, location)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// we found an existing key vault that is not soft deleted
// 	if softDel.Properties == nil {
// 		return nil, nil
// 	}

// 	// the logic is this way because the GetDeleted call will return an existing key vault
// 	// that is not soft deleted, but the Deleted Vault properties will be nil
// 	props := *softDel.Properties

// 	result := keyVaultDeletionStatus{}
// 	if props.DeletionDate != nil {
// 		result.deleteDate = props.DeletionDate.Format(time.RFC3339)
// 	}
// 	if props.ScheduledPurgeDate != nil {
// 		result.purgeDate = props.ScheduledPurgeDate.Format(time.RFC3339)
// 	}

// 	if result.deleteDate == "" && result.purgeDate == "" {
// 		return nil, nil
// 	}

// 	return &result, nil
// }

// NOTE: retrieved from azurerm/internal/services/keyvault/parse/vault.go
type VaultId struct {
	SubscriptionId string
	ResourceGroup  string
	Name           string
}

func newVaultID(subscriptionId, resourceGroup, name string) VaultId {
	return VaultId{
		SubscriptionId: subscriptionId,
		ResourceGroup:  resourceGroup,
		Name:           name,
	}
}

func (id VaultId) String() string {
	segments := []string{
		fmt.Sprintf("Name %q", id.Name),
		fmt.Sprintf("Resource Group %q", id.ResourceGroup),
	}
	segmentsStr := strings.Join(segments, " / ")
	return fmt.Sprintf("%s: (%s)", "Vault", segmentsStr)
}

func (id VaultId) ID() string {
	fmtString := "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.KeyVault/vaults/%s"
	return fmt.Sprintf(fmtString, id.SubscriptionId, id.ResourceGroup, id.Name)
}

// VaultID parses a Vault ID into an VaultId struct
func VaultID(input string) (*VaultId, error) {
	id, err := azure.ParseAzureResourceID(input)
	if err != nil {
		return nil, err
	}

	resourceId := VaultId{
		SubscriptionId: id.SubscriptionID,
		ResourceGroup:  id.ResourceGroup,
	}

	if resourceId.SubscriptionId == "" {
		return nil, fmt.Errorf("ID was missing the 'subscriptions' element")
	}

	if resourceId.ResourceGroup == "" {
		return nil, fmt.Errorf("ID was missing the 'resourceGroups' element")
	}

	if resourceId.Name, err = id.PopSegment("vaults"); err != nil {
		return nil, err
	}

	if err := id.ValidateNoEmptySegments(input); err != nil {
		return nil, err
	}

	return &resourceId, nil
}

// helpers
// retrieved from azurerm/internal/tf/set/set.go

func HashStringIgnoreCase(v interface{}) int {
	return schema.HashString(strings.ToLower(v.(string)))
}

func HashIPv4AddressOrCIDR(ipv4 interface{}) int {
	warnings, errors := commonValidate.IPv4Address(ipv4, "")

	// maybe cidr, just hash it
	if len(warnings) > 0 || len(errors) > 0 {
		return schema.HashString(ipv4)
	}

	// convert to cidr hash
	cidr := fmt.Sprintf("%s/32", ipv4.(string))
	return schema.HashString(cidr)
}

//internal

type NestedItemId struct {
	KeyVaultBaseUrl string
	NestedItemType  string
	Name            string
	Version         string
}

func (n NestedItemId) ID() string {
	// example: https://tharvey-keyvault.vault.azure.net/type/bird/fdf067c93bbb4b22bff4d8b7a9a56217
	segments := []string{
		strings.TrimSuffix(n.KeyVaultBaseUrl, "/"),
		n.NestedItemType,
		n.Name,
	}
	if n.Version != "" {
		segments = append(segments, n.Version)
	}
	return strings.TrimSuffix(strings.Join(segments, "/"), "/")
}

func nestedItemResourceImporter(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := ParseNestedItemID(d.Id())
	if err != nil {
		return []*schema.ResourceData{d}, fmt.Errorf("parsing ID %q for Key Vault Child import: %v", d.Id(), err)
	}

	keyVaultId, err := meta.(*ArmClient).KeyVaultIDFromBaseUrl(ctx, id.KeyVaultBaseUrl)
	if err != nil {
		return []*schema.ResourceData{d}, fmt.Errorf("retrieving the Resource ID the Key Vault at URL %q: %s", id.KeyVaultBaseUrl, err)
	}
	d.Set("key_vault_id", keyVaultId)

	return []*schema.ResourceData{d}, nil
}

// ParseNestedItemID parses a Key Vault Nested Item ID (such as a Certificate, Key or Secret)
// containing a version into a NestedItemId object
func ParseNestedItemID(input string) (*NestedItemId, error) {
	item, err := parseNestedItemId(input)
	if err != nil {
		return nil, err
	}

	if item.Version == "" {
		return nil, fmt.Errorf("expected a versioned ID but no version in %q", input)
	}

	return item, nil
}

func parseNestedItemId(id string) (*NestedItemId, error) {
	// versioned example: https://tharvey-keyvault.vault.azure.net/type/bird/fdf067c93bbb4b22bff4d8b7a9a56217
	// versionless example: https://tharvey-keyvault.vault.azure.net/type/bird/
	idURL, err := url.ParseRequestURI(id)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse Azure KeyVault Child Id: %s", err)
	}

	path := idURL.Path

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	components := strings.Split(path, "/")

	if len(components) != 2 && len(components) != 3 {
		return nil, fmt.Errorf("KeyVault Nested Item should contain 2 or 3 segments, got %d from %q", len(components), path)
	}

	version := ""
	if len(components) == 3 {
		version = components[2]
	}

	childId := NestedItemId{
		KeyVaultBaseUrl: fmt.Sprintf("%s://%s/", idURL.Scheme, idURL.Host),
		NestedItemType:  components[0],
		Name:            components[1],
		Version:         version,
	}

	return &childId, nil
}

func validateVaultID(input interface{}, key string) (warnings []string, errors []error) {
	v, ok := input.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected %q to be a string", key))
		return
	}

	if _, err := VaultID(v); err != nil {
		errors = append(errors, err)
	}

	return
}

func validateNestedItemName(v interface{}, k string) (warnings []string, errors []error) {
	value := v.(string)

	if matched := regexp.MustCompile(`^[0-9a-zA-Z-]+$`).Match([]byte(value)); !matched {
		errors = append(errors, fmt.Errorf("%q may only contain alphanumeric characters and dashes", k))
	}

	return warnings, errors
}