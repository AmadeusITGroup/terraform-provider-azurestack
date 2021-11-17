package azure

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/azure"
)

type ManagementGroupId struct {
	Name string
}

type ResourceGroupId struct {
	SubscriptionId string
	ResourceGroup  string
}

func ParseManagementGroupID(input string) (*ManagementGroupId, error) {
	// func ManagementGroupID(input string) (*ManagementGroupId, error) {
	regex := regexp.MustCompile(`^/providers/[Mm]icrosoft\.[Mm]anagement/[Mm]anagement[Gg]roups/`)
	if !regex.MatchString(input) {
		return nil, fmt.Errorf("Unable to parse Management Group ID %q", input)
	}

	// Split the input ID by the regex
	segments := regex.Split(input, -1)
	if len(segments) != 2 {
		return nil, fmt.Errorf("Unable to parse Management Group ID %q: expected id to have two segments after splitting", input)
	}

	groupID := segments[1]
	if groupID == "" {
		return nil, fmt.Errorf("unable to parse Management Group ID %q: management group name is empty", input)
	}
	if segments := strings.Split(groupID, "/"); len(segments) != 1 {
		return nil, fmt.Errorf("unable to parse Management Group ID %q: ID has extra segments", input)
	}

	id := ManagementGroupId{
		Name: groupID,
	}

	return &id, nil
}

func NewResourceGroupID(subscriptionId, resourceGroup string) ResourceGroupId {
	return ResourceGroupId{
		SubscriptionId: subscriptionId,
		ResourceGroup:  resourceGroup,
	}
}

func (id ResourceGroupId) String() string {
	segments := []string{
		fmt.Sprintf("Resource Group %q", id.ResourceGroup),
	}
	segmentsStr := strings.Join(segments, " / ")
	return fmt.Sprintf("%s: (%s)", "Resource Group", segmentsStr)
}

func (id ResourceGroupId) ID() string {
	fmtString := "/subscriptions/%s/resourceGroups/%s"
	return fmt.Sprintf(fmtString, id.SubscriptionId, id.ResourceGroup)
}

// ResourceGroupID parses a ResourceGroup ID into an ResourceGroupId struct
func ParseResourceGroupID(input string) (*ResourceGroupId, error) {
	// func ResourceGroupID(input string) (*ResourceGroupId, error) {
	id, err := azure.ParseAzureResourceID(input)
	if err != nil {
		return nil, err
	}

	resourceId := ResourceGroupId{
		SubscriptionId: id.SubscriptionID,
		ResourceGroup:  id.ResourceGroup,
	}

	if resourceId.SubscriptionId == "" {
		return nil, fmt.Errorf("ID was missing the 'subscriptions' element")
	}

	if resourceId.ResourceGroup == "" {
		return nil, fmt.Errorf("ID was missing the 'resourceGroups' element")
	}

	if err := id.ValidateNoEmptySegments(input); err != nil {
		return nil, err
	}

	return &resourceId, nil
}
