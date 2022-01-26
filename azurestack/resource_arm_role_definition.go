package azurestack

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-provider-azurerm/azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/azurerm/utils"
	"github.com/hashicorp/terraform-provider-azurestack/azurestack/helpers/response"
	azSchema "github.com/hashicorp/terraform-provider-azurestack/azurestack/internal/tf/schema"
)

func resourceArmRoleDefinition() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmRoleDefinitionCreateUpdate,
		Read:   resourceArmRoleDefinitionRead,
		Update: resourceArmRoleDefinitionCreateUpdate,
		Delete: resourceArmRoleDefinitionDelete,

		Importer: azSchema.ValidateResourceIDPriorToImport(func(id string) error {
			_, err := parseRoleDefinitionId(id)
			return err
		}),

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Read:   schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		SchemaVersion: 1,

		StateUpgraders: []schema.StateUpgrader{
			{
				Type:    resourceArmRoleDefinitionV0().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceArmRoleDefinitionStateUpgradeV0,
				Version: 0,
			},
		},

		Schema: map[string]*schema.Schema{
			"role_definition_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
			},

			"scope": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"permissions": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"actions": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"not_actions": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						// NOTE: not supported with api-version=2015-07-01
						// "data_actions": {
						// 	Type:     schema.TypeSet,
						// 	Optional: true,
						// 	Elem: &schema.Schema{
						// 		Type: schema.TypeString,
						// 	},
						// 	Set: schema.HashString,
						// },
						// "not_data_actions": {
						// 	Type:     schema.TypeSet,
						// 	Optional: true,
						// 	Elem: &schema.Schema{
						// 		Type: schema.TypeString,
						// 	},
						// 	Set: schema.HashString,
						// },
					},
				},
			},

			"assignable_scopes": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"role_definition_resource_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceArmRoleDefinitionCreateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).authorizationClient.RoleDefinitionsClient
	ctx, cancel := ForCreateUpdate(meta.(*ArmClient).StopContext, d)
	defer cancel()

	roleDefinitionId := d.Get("role_definition_id").(string)
	if roleDefinitionId == "" {
		uuid, err := uuid.GenerateUUID()
		if err != nil {
			return fmt.Errorf("generating UUID for Role Assignment: %+v", err)
		}

		roleDefinitionId = uuid
	}

	name := d.Get("name").(string)
	scope := d.Get("scope").(string)
	description := d.Get("description").(string)
	roleType := "CustomRole"
	permissions := expandRoleDefinitionPermissions(d)
	assignableScopes := expandRoleDefinitionAssignableScopes(d)

	if d.IsNewResource() {
		existing, err := client.Get(ctx, scope, roleDefinitionId)
		if err != nil {
			if !response.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("checking for presence of existing Role Definition ID for %q (Scope %q)", name, scope)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			importID := fmt.Sprintf("%s|%s", *existing.ID, scope)
			return tf.ImportAsExistsError("azurerm_role_definition", importID)
		}
	}

	properties := authorization.RoleDefinition{
		RoleDefinitionProperties: &authorization.RoleDefinitionProperties{
			RoleName:         utils.String(name),
			Description:      utils.String(description),
			RoleType:         utils.String(roleType),
			Permissions:      &permissions,
			AssignableScopes: &assignableScopes,
		},
	}

	if _, err := client.CreateOrUpdate(ctx, scope, roleDefinitionId, properties); err != nil {
		return err
	}

	// (@jackofallops) - Updates are subject to eventual consistency, and could be read as stale data
	if !d.IsNewResource() {
		id, err := parseRoleDefinitionId(d.Id())
		if err != nil {
			return err
		}
		stateConf := &resource.StateChangeConf{
			Pending: []string{
				"Pending",
			},
			Target: []string{
				"OK",
			},
			Refresh:                   roleDefinitionUpdateStateRefreshFunc(ctx, client, id.ResourceID),
			MinTimeout:                10 * time.Second,
			ContinuousTargetOccurence: 6,
			Timeout:                   d.Timeout(schema.TimeoutUpdate),
		}

		if _, err := stateConf.WaitForState(); err != nil {
			return fmt.Errorf("waiting for update to Role Definition %q to finish replicating", name)
		}
	}

	read, err := client.Get(ctx, scope, roleDefinitionId)
	if err != nil {
		return err
	}
	if read.ID == nil || *read.ID == "" {
		return fmt.Errorf("Cannot read Role Definition ID for %q (Scope %q)", name, scope)
	}

	d.SetId(fmt.Sprintf("%s|%s", *read.ID, scope))
	return resourceArmRoleDefinitionRead(d, meta)
}

func resourceArmRoleDefinitionRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).authorizationClient.RoleDefinitionsClient
	ctx, cancel := ForRead(meta.(*ArmClient).StopContext, d)
	defer cancel()

	roleDefinitionId, err := parseRoleDefinitionId(d.Id())
	if err != nil {
		return err
	}

	d.Set("scope", roleDefinitionId.Scope)
	d.Set("role_definition_id", roleDefinitionId.RoleID)
	d.Set("role_definition_resource_id", roleDefinitionId.ResourceID)

	resp, err := client.Get(ctx, roleDefinitionId.Scope, roleDefinitionId.RoleID)
	if err != nil {
		if response.ResponseWasNotFound(resp.Response) {
			log.Printf("[DEBUG] Role Definition %q was not found - removing from state", d.Id())
			d.SetId("")
			return nil
		}

		return fmt.Errorf("loading Role Definition %q: %+v", d.Id(), err)
	}

	if props := resp.RoleDefinitionProperties; props != nil {
		d.Set("name", props.RoleName)
		d.Set("description", props.Description)

		permissions := flattenRoleDefinitionPermissions(props.Permissions)
		if err := d.Set("permissions", permissions); err != nil {
			return err
		}

		assignableScopes := flattenRoleDefinitionAssignableScopes(props.AssignableScopes)
		if err := d.Set("assignable_scopes", assignableScopes); err != nil {
			return err
		}
	}

	return nil
}

func resourceArmRoleDefinitionDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).authorizationClient.RoleDefinitionsClient
	ctx, cancel := ForDelete(meta.(*ArmClient).StopContext, d)
	defer cancel()

	id, _ := parseRoleDefinitionId(d.Id())

	resp, err := client.Delete(ctx, id.Scope, id.RoleID)
	if err != nil {
		if !response.ResponseWasNotFound(resp.Response) {
			return fmt.Errorf("deleting Role Definition %q at Scope %q: %+v", id.RoleID, id.Scope, err)
		}
	}
	// Deletes are not instant and can take time to propagate
	stateConf := &resource.StateChangeConf{
		Pending: []string{
			"Pending",
		},
		Target: []string{
			"Deleted",
			"NotFound",
		},
		Refresh:                   roleDefinitionDeleteStateRefreshFunc(ctx, client, id.ResourceID),
		MinTimeout:                10 * time.Second,
		ContinuousTargetOccurence: 6,
		Timeout:                   d.Timeout(schema.TimeoutDelete),
	}

	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("waiting for delete on Role Definition %q to complete", id.RoleID)
	}

	return nil
}

func expandRoleDefinitionPermissions(d *schema.ResourceData) []authorization.Permission {
	output := make([]authorization.Permission, 0)

	permissions := d.Get("permissions").([]interface{})
	for _, v := range permissions {
		input := v.(map[string]interface{})
		permission := authorization.Permission{}

		actionsOutput := make([]string, 0)
		actions := input["actions"].([]interface{})
		for _, a := range actions {
			actionsOutput = append(actionsOutput, a.(string))
		}
		permission.Actions = &actionsOutput

		// NOTE: not supported with api-version=2015-07-01
		// dataActionsOutput := make([]string, 0)
		// dataActions := input["data_actions"].(*schema.Set)
		// for _, a := range dataActions.List() {
		// 	dataActionsOutput = append(dataActionsOutput, a.(string))
		// }
		// permission.DataActions = &dataActionsOutput

		notActionsOutput := make([]string, 0)
		notActions := input["not_actions"].([]interface{})
		for _, a := range notActions {
			notActionsOutput = append(notActionsOutput, a.(string))
		}
		permission.NotActions = &notActionsOutput

		// NOTE: not supported with api-version=2015-07-01
		// notDataActionsOutput := make([]string, 0)
		// notDataActions := input["not_data_actions"].(*schema.Set)
		// for _, a := range notDataActions.List() {
		// 	notDataActionsOutput = append(notDataActionsOutput, a.(string))
		// }
		// permission.NotDataActions = &notDataActionsOutput

		output = append(output, permission)
	}

	return output
}

func expandRoleDefinitionAssignableScopes(d *schema.ResourceData) []string {
	scopes := make([]string, 0)

	assignableScopes := d.Get("assignable_scopes").([]interface{})
	if len(assignableScopes) == 0 {
		assignedScope := d.Get("scope").(string)
		scopes = append(scopes, assignedScope)
	} else {
		for _, scope := range assignableScopes {
			scopes = append(scopes, scope.(string))
		}
	}

	return scopes
}

func flattenRoleDefinitionPermissions(input *[]authorization.Permission) []interface{} {
	permissions := make([]interface{}, 0)
	if input == nil {
		return permissions
	}

	for _, permission := range *input {
		output := make(map[string]interface{})

		actions := make([]string, 0)
		if s := permission.Actions; s != nil {
			actions = *s
		}
		output["actions"] = actions

		// NOTE: not supported with api-version=2015-07-01
		// dataActions := make([]interface{}, 0)
		// if permission.DataActions != nil {
		// 	for _, dataAction := range *permission.DataActions {
		// 		dataActions = append(dataActions, dataAction)
		// 	}
		// }
		// output["data_actions"] = schema.NewSet(schema.HashString, dataActions)

		notActions := make([]string, 0)
		if s := permission.NotActions; s != nil {
			notActions = *s
		}
		output["not_actions"] = notActions

		// NOTE: not supported with api-version=2015-07-01
		// notDataActions := make([]interface{}, 0)
		// if permission.NotDataActions != nil {
		// 	for _, dataAction := range *permission.NotDataActions {
		// 		notDataActions = append(notDataActions, dataAction)
		// 	}
		// }
		// output["not_data_actions"] = schema.NewSet(schema.HashString, notDataActions)

		permissions = append(permissions, output)
	}

	return permissions
}

func flattenRoleDefinitionAssignableScopes(input *[]string) []interface{} {
	scopes := make([]interface{}, 0)
	if input == nil {
		return scopes
	}

	for _, scope := range *input {
		scopes = append(scopes, scope)
	}

	return scopes
}

func roleDefinitionUpdateStateRefreshFunc(ctx context.Context, client *authorization.RoleDefinitionsClient, roleDefinitionId string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := client.GetByID(ctx, roleDefinitionId)
		if err != nil {
			if response.ResponseWasNotFound(resp.Response) {
				return resp, "NotFound", err
			}
			return resp, "Error", err
		}
		return "OK", "OK", nil
	}
}

func roleDefinitionDeleteStateRefreshFunc(ctx context.Context, client *authorization.RoleDefinitionsClient, roleDefinitionId string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := client.GetByID(ctx, roleDefinitionId)
		if err != nil {
			if response.ResponseWasNotFound(resp.Response) {
				return resp, "NotFound", nil
			}
			return nil, "Error", err
		}
		return "Pending", "Pending", nil
	}
}

// copied from 	"github.com/hashicorp/terraform-provider-azurerm/azurerm/internal/services/authorization/parse" v2.48.0-openshift

type RoleDefinitionID struct {
	ResourceID string
	Scope      string
	RoleID     string
}

// RoleDefinitionId is a pseudo ID for storing Scope parameter as this it not retrievable from API
// It is formed of the Azure Resource ID for the Role and the Scope it is created against
func parseRoleDefinitionId(input string) (*RoleDefinitionID, error) {
	parts := strings.Split(input, "|")
	if len(parts) != 2 {
		return nil, fmt.Errorf("could not parse Role Definition ID, invalid format %q", input)
	}

	idParts := strings.Split(parts[0], "roleDefinitions/")

	if !strings.HasPrefix(parts[1], "/subscriptions/") && !strings.HasPrefix(parts[1], "/providers/Microsoft.Management/managementGroups/") {
		return nil, fmt.Errorf("failed to parse scope from Role Definition ID %q", input)
	}

	roleDefinitionID := RoleDefinitionID{
		ResourceID: parts[0],
		Scope:      parts[1],
	}

	if len(idParts) < 1 {
		return nil, fmt.Errorf("failed to parse Role Definition ID from resource ID %q", input)
	} else {
		roleDefinitionID.RoleID = idParts[1]
	}

	return &roleDefinitionID, nil
}
