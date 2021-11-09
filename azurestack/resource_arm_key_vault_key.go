package azurestack

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmKeyVaultKey() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmKeyVaultKeyCreate,
		Read:   resourceArmKeyVaultKeyRead,
		Update: resourceArmKeyVaultKeyUpdate,
		Delete: resourceArmKeyVaultKeyDelete,
		Importer: &schema.ResourceImporter{
			State: nestedItemResourceImporter,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateNestedItemName,
			},

			"key_vault_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateVaultID,
			},

			"key_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				// turns out Azure's *really* sensitive about the casing of these
				// issue: https://github.com/Azure/azure-rest-api-specs/issues/1739
				ValidateFunc: validation.StringInSlice([]string{
					// TODO: add `oct` back in once this is fixed
					// https://github.com/Azure/azure-rest-api-specs/issues/1739#issuecomment-332236257
					string(keyvault.EC),
					string(keyvault.ECHSM),
					string(keyvault.RSA),
					string(keyvault.RSAHSM),
				}, false),
			},

			"key_size": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"curve"},
			},

			"key_opts": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					// turns out Azure's *really* sensitive about the casing of these
					// issue: https://github.com/Azure/azure-rest-api-specs/issues/1739
					ValidateFunc: validation.StringInSlice([]string{
						string(keyvault.Decrypt),
						string(keyvault.Encrypt),
						string(keyvault.Sign),
						string(keyvault.UnwrapKey),
						string(keyvault.Verify),
						string(keyvault.WrapKey),
					}, false),
				},
			},

			"curve": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(keyvault.P256),
					string(keyvault.P384),
					string(keyvault.P521),
					string(keyvault.SECP256K1),
				}, false),
				// TODO: the curve name should probably be mandatory for EC in the future,
				// but handle the diff so that we don't break existing configurations and
				// imported EC keys
				ConflictsWith: []string{"key_size"},
			},

			"not_before_date": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.IsRFC3339Time,
			},

			"expiration_date": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.IsRFC3339Time,
			},

			// Computed
			"version": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"versionless_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"n": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"e": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"x": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"y": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tags": tagsSchema(),
		},
	}
}

func resourceArmKeyVaultKeyCreate(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*ArmClient).keyVaultMgmtClient
	ctx, cancel := ForCreate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	log.Print("[INFO] preparing arguments for AzureRM KeyVault Key creation.")
	name := d.Get("name").(string)
	keyVaultId, err := VaultID(d.Get("key_vault_id").(string))
	if err != nil {
		return err
	}

	keyVaultBaseUri, err := meta.(*ArmClient).BaseUriForKeyVault(ctx, *keyVaultId)
	if err != nil {
		return fmt.Errorf("Error looking up Key %q vault url from id %q: %+v", name, *keyVaultId, err)
	}

	existing, err := client.GetKey(ctx, *keyVaultBaseUri, name, "")
	if err != nil {
		if !utils.ResponseWasNotFound(existing.Response) {
			return fmt.Errorf("Error checking for presence of existing Key %q (Key Vault %q): %s", name, *keyVaultBaseUri, err)
		}
	}

	if existing.Key != nil && existing.Key.Kid != nil && *existing.Key.Kid != "" {
		return tf.ImportAsExistsError("azurerm_key_vault_key", *existing.Key.Kid)
	}

	keyType := d.Get("key_type").(string)
	keyOptions := expandKeyVaultKeyOptions(d)
	t := d.Get("tags").(map[string]interface{})

	// TODO: support Importing Keys once this is fixed:
	// https://github.com/Azure/azure-rest-api-specs/issues/1747
	parameters := keyvault.KeyCreateParameters{
		Kty:    keyvault.JSONWebKeyType(keyType),
		KeyOps: keyOptions,
		KeyAttributes: &keyvault.KeyAttributes{
			Enabled: utils.Bool(true),
		},

		Tags: *expandTags(t),
	}

	if parameters.Kty == keyvault.EC || parameters.Kty == keyvault.ECHSM {
		curveName := d.Get("curve").(string)
		parameters.Curve = keyvault.JSONWebKeyCurveName(curveName)
	} else if parameters.Kty == keyvault.RSA || parameters.Kty == keyvault.RSAHSM {
		keySize, ok := d.GetOk("key_size")
		if !ok {
			return fmt.Errorf("Key size is required when creating an RSA key")
		}
		parameters.KeySize = utils.Int32(int32(keySize.(int)))
	}
	// TODO: support `oct` once this is fixed
	// https://github.com/Azure/azure-rest-api-specs/issues/1739#issuecomment-332236257

	if v, ok := d.GetOk("not_before_date"); ok {
		notBeforeDate, _ := time.Parse(time.RFC3339, v.(string)) // validated by schema
		notBeforeUnixTime := date.UnixTime(notBeforeDate)
		parameters.KeyAttributes.NotBefore = &notBeforeUnixTime
	}

	if v, ok := d.GetOk("expiration_date"); ok {
		expirationDate, _ := time.Parse(time.RFC3339, v.(string)) // validated by schema
		expirationUnixTime := date.UnixTime(expirationDate)
		parameters.KeyAttributes.Expires = &expirationUnixTime
	}

	if _, err := client.CreateKey(ctx, *keyVaultBaseUri, name, parameters); err != nil {
		// NOTE: features unsupported
		// if meta.(*ArmClient).Features.KeyVault.RecoverSoftDeletedKeyVaults && utils.ResponseWasConflict(resp.Response) {
		// 	recoveredKey, err := client.RecoverDeletedKey(ctx, *keyVaultBaseUri, name)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	log.Printf("[DEBUG] Recovering Key %q with ID: %q", name, *recoveredKey.Key.Kid)
		// 	if kid := recoveredKey.Key.Kid; kid != nil {
		// 		stateConf := &resource.StateChangeConf{
		// 			Pending:                   []string{"pending"},
		// 			Target:                    []string{"available"},
		// 			Refresh:                   keyVaultChildItemRefreshFunc(*kid),
		// 			Delay:                     30 * time.Second,
		// 			PollInterval:              10 * time.Second,
		// 			ContinuousTargetOccurence: 10,
		// 			Timeout:                   d.Timeout(schema.TimeoutCreate),
		// 		}

		// 		if _, err := stateConf.WaitForState(); err != nil {
		// 			return fmt.Errorf("Error waiting for Key Vault Secret %q to become available: %s", name, err)
		// 		}
		// 		log.Printf("[DEBUG] Key %q recovered with ID: %q", name, *kid)
		// 	}
		// } else {
		// 	return fmt.Errorf("Error Creating Key: %+v", err)
		// }
		return fmt.Errorf("Error Creating Key: %+v", err)
	}

	// "" indicates the latest version
	read, err := client.GetKey(ctx, *keyVaultBaseUri, name, "")
	if err != nil {
		return err
	}

	d.SetId(*read.Key.Kid)

	return resourceArmKeyVaultKeyRead(d, meta)
}

func resourceArmKeyVaultKeyUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultMgmtClient

	ctx, cancel := ForUpdate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := ParseNestedItemID(d.Id())
	if err != nil {
		return err
	}

	keyVaultIdRaw, err := meta.(*ArmClient).KeyVaultIDFromBaseUrl(ctx, id.KeyVaultBaseUrl)
	if err != nil {
		return fmt.Errorf("retrieving the Resource ID the Key Vault at URL %q: %s", id.KeyVaultBaseUrl, err)
	}
	if keyVaultIdRaw == nil {
		return fmt.Errorf("Unable to determine the Resource ID for the Key Vault at URL %q", id.KeyVaultBaseUrl)
	}

	keyVaultId, err := VaultID(*keyVaultIdRaw)
	if err != nil {
		return err
	}

	ok, err := meta.(*ArmClient).KeyVaultExists(ctx, *keyVaultId)
	if err != nil {
		return fmt.Errorf("Error checking if key vault %q for Key %q in Vault at url %q exists: %v", *keyVaultId, id.Name, id.KeyVaultBaseUrl, err)
	}
	if !ok {
		log.Printf("[DEBUG] Key %q Key Vault %q was not found in Key Vault at URI %q - removing from state", id.Name, *keyVaultId, id.KeyVaultBaseUrl)
		d.SetId("")
		return nil
	}

	keyOptions := expandKeyVaultKeyOptions(d)
	t := d.Get("tags").(map[string]interface{})

	parameters := keyvault.KeyUpdateParameters{
		KeyOps: keyOptions,
		KeyAttributes: &keyvault.KeyAttributes{
			Enabled: utils.Bool(true),
		},
		Tags: *expandTags(t),
	}

	if v, ok := d.GetOk("not_before_date"); ok {
		notBeforeDate, _ := time.Parse(time.RFC3339, v.(string)) // validated by schema
		notBeforeUnixTime := date.UnixTime(notBeforeDate)
		parameters.KeyAttributes.NotBefore = &notBeforeUnixTime
	}

	if v, ok := d.GetOk("expiration_date"); ok {
		expirationDate, _ := time.Parse(time.RFC3339, v.(string)) // validated by schema
		expirationUnixTime := date.UnixTime(expirationDate)
		parameters.KeyAttributes.Expires = &expirationUnixTime
	}

	if _, err = client.UpdateKey(ctx, id.KeyVaultBaseUrl, id.Name, "", parameters); err != nil {
		return err
	}

	return resourceArmKeyVaultKeyRead(d, meta)
}

func resourceArmKeyVaultKeyRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).keyVaultMgmtClient
	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := ParseNestedItemID(d.Id())
	if err != nil {
		return err
	}

	keyVaultIdRaw, err := meta.(*ArmClient).KeyVaultIDFromBaseUrl(ctx, id.KeyVaultBaseUrl)
	if err != nil {
		return fmt.Errorf("Error retrieving the Resource ID the Key Vault at URL %q: %s", id.KeyVaultBaseUrl, err)
	}
	if keyVaultIdRaw == nil {
		log.Printf("[DEBUG] Unable to determine the Resource ID for the Key Vault at URL %q - removing from state!", id.KeyVaultBaseUrl)
		d.SetId("")
		return nil
	}
	keyVaultId, err := VaultID(*keyVaultIdRaw)
	if err != nil {
		return err
	}

	ok, err := meta.(*ArmClient).KeyVaultExists(ctx, *keyVaultId)
	if err != nil {
		return fmt.Errorf("Error checking if key vault %q for Key %q in Vault at url %q exists: %v", *keyVaultId, id.Name, id.KeyVaultBaseUrl, err)
	}
	if !ok {
		log.Printf("[DEBUG] Key %q Key Vault %q was not found in Key Vault at URI %q - removing from state", id.Name, *keyVaultId, id.KeyVaultBaseUrl)
		d.SetId("")
		return nil
	}

	resp, err := client.GetKey(ctx, id.KeyVaultBaseUrl, id.Name, "")
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[DEBUG] Key %q was not found in Key Vault at URI %q - removing from state", id.Name, id.KeyVaultBaseUrl)
			d.SetId("")
			return nil
		}

		return err
	}

	d.Set("name", id.Name)

	if key := resp.Key; key != nil {
		d.Set("key_type", string(key.Kty))

		options := flattenKeyVaultKeyOptions(key.KeyOps)
		if err := d.Set("key_opts", options); err != nil {
			return err
		}

		d.Set("n", key.N)
		d.Set("e", key.E)
		d.Set("x", key.X)
		d.Set("y", key.Y)
		if key.N != nil {
			nBytes, err := base64.RawURLEncoding.DecodeString(*key.N)
			if err != nil {
				return fmt.Errorf("Could not decode N: %+v", err)
			}
			d.Set("key_size", len(nBytes)*8)
		}

		d.Set("curve", key.Crv)
	}

	if attributes := resp.Attributes; attributes != nil {
		if v := attributes.NotBefore; v != nil {
			d.Set("not_before_date", time.Time(*v).Format(time.RFC3339))
		}

		if v := attributes.Expires; v != nil {
			d.Set("expiration_date", time.Time(*v).Format(time.RFC3339))
		}
	}

	// Computed
	d.Set("version", id.Version)
	d.Set("versionless_id", fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(id.KeyVaultBaseUrl, "/"), id.NestedItemType, id.Name))

	flattenAndSetTags(d, &resp.Tags)

	return nil
}

func resourceArmKeyVaultKeyDelete(d *schema.ResourceData, meta interface{}) error {
	ctx, cancel := ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, err := ParseNestedItemID(d.Id())
	if err != nil {
		return err
	}

	keyVaultIdRaw, err := meta.(*ArmClient).KeyVaultIDFromBaseUrl(ctx, id.KeyVaultBaseUrl)
	if err != nil {
		return fmt.Errorf("Error retrieving the Resource ID the Key Vault at URL %q: %s", id.KeyVaultBaseUrl, err)
	}
	if keyVaultIdRaw == nil {
		return fmt.Errorf("Unable to determine the Resource ID for the Key Vault at URL %q", id.KeyVaultBaseUrl)
	}
	keyVaultId, err := VaultID(*keyVaultIdRaw)
	if err != nil {
		return err
	}

	ok, err := meta.(*ArmClient).KeyVaultExists(ctx, *keyVaultId)
	if err != nil {
		return fmt.Errorf("Error checking if key vault %q for Key %q in Vault at url %q exists: %v", *keyVaultId, id.Name, id.KeyVaultBaseUrl, err)
	}
	if !ok {
		log.Printf("[DEBUG] Key %q Key Vault %q was not found in Key Vault at URI %q - removing from state", id.Name, *keyVaultId, id.KeyVaultBaseUrl)
		d.SetId("")
		return nil
	}

	// NOTE: at this time, feature not yet implemented
	// shouldPurge := meta.(*ArmClient).Features.KeyVault.PurgeSoftDeleteOnDestroy
	shouldPurge := true
	description := fmt.Sprintf("Key %q (Key Vault %q)", id.Name, id.KeyVaultBaseUrl)
	deleter := deleteAndPurgeKey{
		client:      &meta.(*ArmClient).keyVaultMgmtClient,
		keyVaultUri: id.KeyVaultBaseUrl,
		name:        id.Name,
	}
	if err := deleteAndOptionallyPurge(ctx, description, shouldPurge, deleter); err != nil {
		return err
	}

	return nil
}

var _ deleteAndPurgeNestedItem = deleteAndPurgeKey{}

type deleteAndPurgeKey struct {
	client      *keyvault.BaseClient
	keyVaultUri string
	name        string
}

func (d deleteAndPurgeKey) DeleteNestedItem(ctx context.Context) (autorest.Response, error) {
	resp, err := d.client.DeleteKey(ctx, d.keyVaultUri, d.name)
	return resp.Response, err
}

func (d deleteAndPurgeKey) NestedItemHasBeenDeleted(ctx context.Context) (autorest.Response, error) {
	resp, err := d.client.GetKey(ctx, d.keyVaultUri, d.name, "")
	return resp.Response, err
}

func (d deleteAndPurgeKey) PurgeNestedItem(ctx context.Context) (autorest.Response, error) {
	return d.client.PurgeDeletedKey(ctx, d.keyVaultUri, d.name)
}

func (d deleteAndPurgeKey) NestedItemHasBeenPurged(ctx context.Context) (autorest.Response, error) {
	resp, err := d.client.GetDeletedKey(ctx, d.keyVaultUri, d.name)
	return resp.Response, err
}

func expandKeyVaultKeyOptions(d *schema.ResourceData) *[]keyvault.JSONWebKeyOperation {
	options := d.Get("key_opts").([]interface{})
	results := make([]keyvault.JSONWebKeyOperation, 0, len(options))

	for _, option := range options {
		results = append(results, keyvault.JSONWebKeyOperation(option.(string)))
	}

	return &results
}

func flattenKeyVaultKeyOptions(input *[]string) []interface{} {
	results := make([]interface{}, 0, len(*input))

	for _, option := range *input {
		results = append(results, option)
	}

	return results
}

type deleteAndPurgeNestedItem interface {
	DeleteNestedItem(ctx context.Context) (autorest.Response, error)
	NestedItemHasBeenDeleted(ctx context.Context) (autorest.Response, error)

	PurgeNestedItem(ctx context.Context) (autorest.Response, error)
	NestedItemHasBeenPurged(ctx context.Context) (autorest.Response, error)
}

func deleteAndOptionallyPurge(ctx context.Context, description string, shouldPurge bool, helper deleteAndPurgeNestedItem) error {
	timeout, ok := ctx.Deadline()
	if !ok {
		return fmt.Errorf("context is missing a timeout")
	}

	log.Printf("[DEBUG] Deleting %s..", description)
	if resp, err := helper.DeleteNestedItem(ctx); err != nil {
		if utils.ResponseWasNotFound(resp) {
			return nil
		}

		return fmt.Errorf("deleting %s: %+v", description, err)
	}
	log.Printf("[DEBUG] Waiting for %s to finish deleting..", description)
	stateConf := &resource.StateChangeConf{
		Pending: []string{"InProgress"},
		Target:  []string{"NotFound"},
		Refresh: func() (interface{}, string, error) {
			item, err := helper.NestedItemHasBeenDeleted(ctx)
			if err != nil {
				if utils.ResponseWasNotFound(item) {
					return item, "NotFound", nil
				}

				return nil, "Error", err
			}

			return item, "InProgress", nil
		},
		ContinuousTargetOccurence: 3,
		PollInterval:              5 * time.Second,
		Timeout:                   time.Until(timeout),
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("waiting for %s to be deleted: %+v", description, err)
	}
	log.Printf("[DEBUG] Deleted %s.", description)

	if !shouldPurge {
		log.Printf("[DEBUG] Skipping purging of %s as opted-out..", description)
		return nil
	}

	log.Printf("[DEBUG] Purging %s..", description)
	if _, err := helper.PurgeNestedItem(ctx); err != nil {
		return fmt.Errorf("purging %s: %+v", description, err)
	}

	log.Printf("[DEBUG] Waiting for %s to finish purging..", description)
	stateConf = &resource.StateChangeConf{
		Pending: []string{"InProgress"},
		Target:  []string{"NotFound"},
		Refresh: func() (interface{}, string, error) {
			item, err := helper.NestedItemHasBeenPurged(ctx)
			if err != nil {
				if utils.ResponseWasNotFound(item) {
					return item, "NotFound", nil
				}

				return nil, "Error", err
			}

			return item, "InProgress", nil
		},
		ContinuousTargetOccurence: 3,
		PollInterval:              5 * time.Second,
		Timeout:                   time.Until(timeout),
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("waiting for %s to finish purging: %+v", description, err)
	}
	log.Printf("[DEBUG] Purged %s.", description)

	return nil
}