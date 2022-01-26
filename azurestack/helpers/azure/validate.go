package azure

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-provider-azurerm/azurerm/helpers/azure"
)

func ValidateResourceID(i interface{}, k string) (_ []string, errors []error) {
	v, ok := i.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
		return
	}

	if _, err := ParseAzureResourceID(v); err != nil {
		errors = append(errors, fmt.Errorf("Can not parse %q as a resource id: %v", k, err))
	}

	return
}

// true for a resource ID or an empty string
func ValidateResourceIDOrEmpty(i interface{}, k string) (_ []string, errors []error) {
	v, ok := i.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
		return
	}

	if v == "" {
		return
	}

	return ValidateResourceID(i, k)
}

func VaultName(v interface{}, k string) (warnings []string, errors []error) {
	value := v.(string)
	if matched := regexp.MustCompile(`^[a-zA-Z0-9-]{3,24}$`).Match([]byte(value)); !matched {
		errors = append(errors, fmt.Errorf("%q may only contain alphanumeric characters and dashes and must be between 3-24 chars", k))
	}

	return warnings, errors
}

func SubscriptionID(i interface{}, k string) (warnings []string, errors []error) {
	v, ok := i.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
		return
	}
	id, err := azure.ParseAzureResourceID(v)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q expected to be valid subscription ID, got %q", k, v))
		return
	}

	if _, err := uuid.ParseUUID(id.SubscriptionID); err != nil {
		errors = append(errors, fmt.Errorf("expected subscription id value in %q to be a valid UUID, got %v", v, id.SubscriptionID))
	}

	if id.ResourceGroup != "" || len(id.Path) > 0 {
		errors = append(errors, fmt.Errorf("%q expected to be valid subscription ID, got other ID type %q", k, v))
		return
	}

	return
}

func ManagementGroupID(i interface{}, k string) (warnings []string, errors []error) {
	v, ok := i.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
		return
	}

	if _, err := ParseManagementGroupID(v); err != nil {
		// if _, err := parse.ManagementGroupID(v); err != nil {
		errors = append(errors, fmt.Errorf("Can not parse %q as a management group id: %v", k, err))
		return
	}

	return
}
