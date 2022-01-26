package azurestack

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/storage/mgmt/storage"
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	vmScaleSet "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2016-04-01/dns"
	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	keyvaultmgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2019-09-01/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-11-01/subscriptions"
	mainStorage "github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/hashicorp/go-azure-helpers/authentication"
	"github.com/hashicorp/go-azure-helpers/sender"
	"github.com/hashicorp/terraform-plugin-sdk/httpclient"
	"github.com/hashicorp/terraform-provider-azurerm/azurerm/utils"
	"github.com/hashicorp/terraform-provider-azurestack/azurestack/helpers/response"
)

type AuthorizationClient struct {
	GroupsClient            *graphrbac.GroupsClient
	RoleAssignmentsClient   *authorization.RoleAssignmentsClient
	RoleDefinitionsClient   *authorization.RoleDefinitionsClient
	ServicePrincipalsClient *graphrbac.ServicePrincipalsClient
}

// ArmClient contains the handles to all the specific Azure Resource Manager
// resource classes' respective clients.
type ArmClient struct {
	clientId                 string
	tenantId                 string
	subscriptionId           string
	terraformVersion         string
	usingServicePrincipal    bool
	environment              azure.Environment
	skipProviderRegistration bool

	StopContext context.Context

	// DNS
	dnsClient   dns.RecordSetsClient
	zonesClient dns.ZonesClient

	// Networking
	ifaceClient                  network.InterfacesClient
	localNetConnClient           network.LocalNetworkGatewaysClient
	secRuleClient                network.SecurityRulesClient
	vnetGatewayClient            network.VirtualNetworkGatewaysClient
	vnetGatewayConnectionsClient network.VirtualNetworkGatewayConnectionsClient

	// Resources
	providersClient resources.ProvidersClient
	resourcesClient resources.Client

	resourceGroupsClient resources.GroupsClient
	deploymentsClient    resources.DeploymentsClient

	// Compute
	availSetClient       compute.AvailabilitySetsClient
	diskClient           compute.DisksClient
	imagesClient         compute.ImagesClient
	vmExtensionClient    compute.VirtualMachineExtensionsClient
	vmClient             compute.VirtualMachinesClient
	vmImageClient        compute.VirtualMachineImagesClient
	vmScaleSetClient     vmScaleSet.VirtualMachineScaleSetsClient
	storageServiceClient storage.AccountsClient

	// Network
	vnetClient         network.VirtualNetworksClient
	secGroupClient     network.SecurityGroupsClient
	publicIPClient     network.PublicIPAddressesClient
	subnetClient       network.SubnetsClient
	loadBalancerClient network.LoadBalancersClient
	routesClient       network.RoutesClient
	routeTablesClient  network.RouteTablesClient
	vnetPeeringClient  network.VirtualNetworkPeeringsClient

	// KeyVault
	keyVaultClient     keyvault.VaultsClient
	keyVaultMgmtClient keyvaultmgmt.BaseClient

	// Subscription
	subscriptionsClient subscriptions.Client

	//Authorization
	authorizationClient AuthorizationClient
}

func (c *ArmClient) configureClient(client *autorest.Client, auth autorest.Authorizer) {
	setUserAgent(client, c.terraformVersion)
	client.Authorizer = auth
	client.Sender = sender.BuildSender("AzureStack")
	client.SkipResourceProviderRegistration = c.skipProviderRegistration
	client.PollingDuration = 60 * time.Minute
}

func setUserAgent(client *autorest.Client, tfVersion string) {
	tfUserAgent := httpclient.TerraformUserAgent(tfVersion)

	// if the user agent already has a value append the Terraform user agent string
	if curUserAgent := client.UserAgent; curUserAgent != "" {
		client.UserAgent = fmt.Sprintf("%s %s", curUserAgent, tfUserAgent)
	} else {
		client.UserAgent = tfUserAgent
	}

	// append the CloudShell version to the user agent if it exists
	if azureAgent := os.Getenv("AZURE_HTTP_USER_AGENT"); azureAgent != "" {
		client.UserAgent = fmt.Sprintf("%s %s", client.UserAgent, azureAgent)
	}
}

// getArmClient is a helper method which returns a fully instantiated
// *ArmClient based on the Config's current settings.
func getArmClient(authCfg *authentication.Config, tfVersion string, skipProviderRegistration bool) (*ArmClient, error) {
	env, err := authentication.LoadEnvironmentFromUrl(authCfg.CustomResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	// client declarations:
	client := ArmClient{
		clientId:                 authCfg.ClientID,
		tenantId:                 authCfg.TenantID,
		subscriptionId:           authCfg.SubscriptionID,
		terraformVersion:         tfVersion,
		environment:              *env,
		usingServicePrincipal:    authCfg.AuthenticatedAsAServicePrincipal,
		skipProviderRegistration: skipProviderRegistration,
	}

	oauth, err := authCfg.BuildOAuthConfig(env.ActiveDirectoryEndpoint)
	if err != nil {
		return nil, err
	}

	sender := sender.BuildSender("AzureStack")

	// Resource Manager endpoints
	endpoint := env.ResourceManagerEndpoint

	// Instead of the same endpoint use token audience to get the correct token.
	auth, err := authCfg.GetAuthorizationToken(sender, oauth, env.TokenAudience)
	if err != nil {
		return nil, err
	}

	// Graph Endpoints
	graphEndpoint := env.GraphEndpoint
	graphAuth, err := authCfg.GetAuthorizationToken(sender, oauth, graphEndpoint)
	if err != nil {
		return nil, err
	}

	// KeyVault Management Endpoint
	keyVaultAuth := authCfg.BearerAuthorizerCallback(sender, oauth)

	client.registerAuthorizationClients(graphEndpoint, endpoint, client.tenantId, client.subscriptionId, graphAuth, auth)

	client.registerComputeClients(endpoint, client.subscriptionId, auth)
	client.registerDNSClients(endpoint, client.subscriptionId, auth)
	client.registerNetworkingClients(endpoint, client.subscriptionId, auth)
	client.registerResourcesClients(endpoint, client.subscriptionId, auth)
	client.registerStorageClients(endpoint, client.subscriptionId, auth)
	client.registerSubscriptionsClient(endpoint, auth)
	client.registerKeyVaultClients(endpoint, client.subscriptionId, auth)
	client.registerKeyVaultMgmtClients(endpoint, client.subscriptionId, keyVaultAuth)

	return &client, nil
}

func (c *ArmClient) registerAuthorizationClients(graphEndpoint, endpoint, tenantId, subscriptionId string, graphAuth, auth autorest.Authorizer) {

	groupsClient := graphrbac.NewGroupsClientWithBaseURI(graphEndpoint, tenantId)
	c.configureClient(&groupsClient.Client, graphAuth)

	roleAssignmentsClient := authorization.NewRoleAssignmentsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&roleAssignmentsClient.Client, auth)

	roleDefinitionsClient := authorization.NewRoleDefinitionsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&roleDefinitionsClient.Client, auth)

	servicePrincipalsClient := graphrbac.NewServicePrincipalsClientWithBaseURI(graphEndpoint, tenantId)
	c.configureClient(&servicePrincipalsClient.Client, graphAuth)

	c.authorizationClient = AuthorizationClient{
		GroupsClient:            &groupsClient,
		RoleAssignmentsClient:   &roleAssignmentsClient,
		RoleDefinitionsClient:   &roleDefinitionsClient,
		ServicePrincipalsClient: &servicePrincipalsClient,
	}

}

func (c *ArmClient) registerComputeClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	availabilitySetsClient := compute.NewAvailabilitySetsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&availabilitySetsClient.Client, auth)
	c.availSetClient = availabilitySetsClient

	diskClient := compute.NewDisksClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&diskClient.Client, auth)
	c.diskClient = diskClient

	imagesClient := compute.NewImagesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&imagesClient.Client, auth)
	c.imagesClient = imagesClient

	extensionsClient := compute.NewVirtualMachineExtensionsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&extensionsClient.Client, auth)
	c.vmExtensionClient = extensionsClient

	scaleSetsClient := vmScaleSet.NewVirtualMachineScaleSetsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&scaleSetsClient.Client, auth)
	c.vmScaleSetClient = scaleSetsClient

	virtualMachinesClient := compute.NewVirtualMachinesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&virtualMachinesClient.Client, auth)
	c.vmClient = virtualMachinesClient

	virtualMachineImagesClient := compute.NewVirtualMachineImagesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&virtualMachineImagesClient.Client, auth)
	c.vmImageClient = virtualMachineImagesClient
}

func (c *ArmClient) registerDNSClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	dn := dns.NewRecordSetsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&dn.Client, auth)
	c.dnsClient = dn

	zo := dns.NewZonesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&zo.Client, auth)
	c.zonesClient = zo
}

func (c *ArmClient) registerNetworkingClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	interfacesClient := network.NewInterfacesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&interfacesClient.Client, auth)
	c.ifaceClient = interfacesClient

	gatewaysClient := network.NewVirtualNetworkGatewaysClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&gatewaysClient.Client, auth)
	c.vnetGatewayClient = gatewaysClient

	gatewayConnectionsClient := network.NewVirtualNetworkGatewayConnectionsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&gatewayConnectionsClient.Client, auth)
	c.vnetGatewayConnectionsClient = gatewayConnectionsClient

	localNetworkGatewaysClient := network.NewLocalNetworkGatewaysClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&localNetworkGatewaysClient.Client, auth)
	c.localNetConnClient = localNetworkGatewaysClient

	loadBalancersClient := network.NewLoadBalancersClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&loadBalancersClient.Client, auth)
	c.loadBalancerClient = loadBalancersClient

	networksClient := network.NewVirtualNetworksClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&networksClient.Client, auth)
	c.vnetClient = networksClient

	publicIPAddressesClient := network.NewPublicIPAddressesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&publicIPAddressesClient.Client, auth)
	c.publicIPClient = publicIPAddressesClient

	securityGroupsClient := network.NewSecurityGroupsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&securityGroupsClient.Client, auth)
	c.secGroupClient = securityGroupsClient

	securityRulesClient := network.NewSecurityRulesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&securityRulesClient.Client, auth)
	c.secRuleClient = securityRulesClient

	subnetsClient := network.NewSubnetsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&subnetsClient.Client, auth)
	c.subnetClient = subnetsClient

	routeTablesClient := network.NewRouteTablesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&routeTablesClient.Client, auth)
	c.routeTablesClient = routeTablesClient

	routesClient := network.NewRoutesClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&routesClient.Client, auth)
	c.routesClient = routesClient

	peeringCLient := network.NewVirtualNetworkPeeringsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&peeringCLient.Client, auth)
	c.vnetPeeringClient = peeringCLient
}

func (c *ArmClient) registerResourcesClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	resourcesClient := resources.NewClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&resourcesClient.Client, auth)
	c.resourcesClient = resourcesClient

	deploymentsClient := resources.NewDeploymentsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&deploymentsClient.Client, auth)
	c.deploymentsClient = deploymentsClient

	resourceGroupsClient := resources.NewGroupsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&resourceGroupsClient.Client, auth)
	c.resourceGroupsClient = resourceGroupsClient

	providersClient := resources.NewProvidersClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&providersClient.Client, auth)
	c.providersClient = providersClient
}

func (c *ArmClient) registerStorageClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	accountsClient := storage.NewAccountsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&accountsClient.Client, auth)
	c.storageServiceClient = accountsClient
}

func (c *ArmClient) registerSubscriptionsClient(endpoint string, auth autorest.Authorizer) {
	subcriptionsClient := subscriptions.NewClientWithBaseURI(endpoint)
	c.configureClient(&subcriptionsClient.Client, auth)
	c.subscriptionsClient = subcriptionsClient
}

func (c *ArmClient) registerKeyVaultClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	keyVaultClient := keyvault.NewVaultsClientWithBaseURI(endpoint, subscriptionId)
	c.configureClient(&keyVaultClient.Client, auth)
	c.keyVaultClient = keyVaultClient
}

func (c *ArmClient) registerKeyVaultMgmtClients(endpoint, subscriptionId string, auth autorest.Authorizer) {
	keyVaultMgmtClient := keyvaultmgmt.New()
	c.configureClient(&keyVaultMgmtClient.Client, auth)
	c.keyVaultMgmtClient = keyVaultMgmtClient
}

var (
	storageKeyCacheMu sync.RWMutex
	storageKeyCache   = make(map[string]string)
)

func (armClient *ArmClient) getKeyForStorageAccount(ctx context.Context, resourceGroupName, storageAccountName string) (string, bool, error) {
	cacheIndex := resourceGroupName + "/" + storageAccountName
	storageKeyCacheMu.RLock()
	key, ok := storageKeyCache[cacheIndex]
	storageKeyCacheMu.RUnlock()

	if ok {
		return key, true, nil
	}

	storageKeyCacheMu.Lock()
	defer storageKeyCacheMu.Unlock()
	key, ok = storageKeyCache[cacheIndex]
	if !ok {
		accountKeys, err := armClient.storageServiceClient.ListKeys(ctx, resourceGroupName, storageAccountName)
		if response.ResponseWasNotFound(accountKeys.Response) {
			return "", false, nil
		}
		if err != nil {
			// We assume this is a transient error rather than a 404 (which is caught above),  so assume the
			// account still exists.
			return "", true, fmt.Errorf("retrieving keys for storage account %q: %s", storageAccountName, err)
		}

		if accountKeys.Keys == nil {
			return "", false, fmt.Errorf("Nil key returned for storage account %q", storageAccountName)
		}

		keys := *accountKeys.Keys
		if len(keys) <= 0 {
			return "", false, fmt.Errorf("No keys returned for storage account %q", storageAccountName)
		}

		keyPtr := keys[0].Value
		if keyPtr == nil {
			return "", false, fmt.Errorf("The first key returned is nil for storage account %q", storageAccountName)
		}

		key = *keyPtr
		storageKeyCache[cacheIndex] = key
	}

	return key, true, nil
}

func (armClient *ArmClient) getBlobStorageClientForStorageAccount(ctx context.Context, resourceGroupName, storageAccountName string) (*mainStorage.BlobStorageClient, bool, error) {
	key, accountExists, err := armClient.getKeyForStorageAccount(ctx, resourceGroupName, storageAccountName)
	if err != nil {
		return nil, accountExists, err
	}
	if !accountExists {
		return nil, false, nil
	}

	storageClient, err := mainStorage.NewClient(storageAccountName, key, armClient.environment.StorageEndpointSuffix,
		"2016-05-31", true)
	if err != nil {
		return nil, true, fmt.Errorf("creating storage client for storage account %q: %s", storageAccountName, err)
	}

	blobClient := storageClient.GetBlobService()
	return &blobClient, true, nil
}

var (
	keyVaultsCache = map[string]keyVaultDetails{}
	keysmith       = &sync.RWMutex{}
	lock           = map[string]*sync.RWMutex{}
)

type keyVaultDetails struct {
	keyVaultId       string
	dataPlaneBaseUri string
	resourceGroup    string
}

func (c *ArmClient) KeyVaultIDFromBaseUrl(ctx context.Context, keyVaultBaseUrl string) (*string, error) {
	keyVaultName, err := c.parseKeyVaultNameFromBaseUrl(keyVaultBaseUrl)
	if err != nil {
		return nil, err
	}

	cacheKey := c.cacheKeyForKeyVault(*keyVaultName)
	keysmith.Lock()
	if lock[cacheKey] == nil {
		lock[cacheKey] = &sync.RWMutex{}
	}
	keysmith.Unlock()
	lock[cacheKey].Lock()
	defer lock[cacheKey].Unlock()

	if v, ok := keyVaultsCache[cacheKey]; ok {
		return &v.keyVaultId, nil
	}

	filter := fmt.Sprintf("resourceType eq 'Microsoft.KeyVault/vaults' and name eq '%s'", *keyVaultName)
	result, err := c.resourcesClient.List(ctx, filter, "", utils.Int32(5))
	if err != nil {
		return nil, fmt.Errorf("listing resources matching %q: %+v", filter, err)
	}

	for result.NotDone() {
		for _, v := range result.Values() {
			if v.ID == nil {
				continue
			}

			id, err := VaultID(*v.ID)
			if err != nil {
				return nil, fmt.Errorf("parsing %q: %+v", *v.ID, err)
			}
			if !strings.EqualFold(id.Name, *keyVaultName) {
				continue
			}

			props, err := c.keyVaultClient.Get(ctx, id.ResourceGroup, id.Name)
			if err != nil {
				return nil, fmt.Errorf("retrieving %s: %+v", *id, err)
			}
			if props.Properties == nil || props.Properties.VaultURI == nil {
				return nil, fmt.Errorf("retrieving %s: `properties.VaultUri` was nil", *id)
			}

			c.AddKeyVaultToCache(*id, *props.Properties.VaultURI)
			return utils.String(id.ID()), nil
		}

		if err := result.NextWithContext(ctx); err != nil {
			return nil, fmt.Errorf("iterating over results: %+v", err)
		}
	}

	// we haven't found it, but Data Sources and Resources need to handle this error separately
	return nil, nil
}

func (c *ArmClient) parseKeyVaultNameFromBaseUrl(input string) (*string, error) {
	uri, err := url.Parse(input)
	if err != nil {
		return nil, err
	}
	// https://tharvey-keyvault.vault.azure.net/
	segments := strings.Split(uri.Host, ".")
	// if len(segments) != 4 { TODO: use keyvault suffix ?
	// 	return nil, fmt.Errorf("expected a URI in the format `vaultname.vault.azure.net` but got %q", uri.Host)
	// }
	return &segments[0], nil
}

func (c *ArmClient) cacheKeyForKeyVault(name string) string {
	return strings.ToLower(name)
}

func (c *ArmClient) AddKeyVaultToCache(keyVaultId VaultId, dataPlaneUri string) {
	cacheKey := c.cacheKeyForKeyVault(keyVaultId.Name)
	keysmith.Lock()
	keyVaultsCache[cacheKey] = keyVaultDetails{
		keyVaultId:       keyVaultId.ID(),
		dataPlaneBaseUri: dataPlaneUri,
		resourceGroup:    keyVaultId.ResourceGroup,
	}
	keysmith.Unlock()
}

func (c *ArmClient) PurgeKeyVaultCache(keyVaultId VaultId) {
	cacheKey := c.cacheKeyForKeyVault(keyVaultId.Name)
	keysmith.Lock()
	if lock[cacheKey] == nil {
		lock[cacheKey] = &sync.RWMutex{}
	}
	keysmith.Unlock()
	lock[cacheKey].Lock()
	delete(keyVaultsCache, cacheKey)
	lock[cacheKey].Unlock()
}

func (c *ArmClient) BaseUriForKeyVault(ctx context.Context, keyVaultId VaultId) (*string, error) {
	cacheKey := c.cacheKeyForKeyVault(keyVaultId.Name)
	keysmith.Lock()
	if lock[cacheKey] == nil {
		lock[cacheKey] = &sync.RWMutex{}
	}
	keysmith.Unlock()
	lock[cacheKey].Lock()
	defer lock[cacheKey].Unlock()

	resp, err := c.keyVaultClient.Get(ctx, keyVaultId.ResourceGroup, keyVaultId.Name)
	if err != nil {
		if response.ResponseWasNotFound(resp.Response) {
			return nil, fmt.Errorf("%s was not found", keyVaultId)
		}
		return nil, fmt.Errorf("retrieving %s: %+v", keyVaultId, err)
	}

	if resp.Properties == nil || resp.Properties.VaultURI == nil {
		return nil, fmt.Errorf("`properties` was nil for %s", keyVaultId)
	}

	return resp.Properties.VaultURI, nil
}

func (c *ArmClient) KeyVaultExists(ctx context.Context, keyVaultId VaultId) (bool, error) {
	cacheKey := c.cacheKeyForKeyVault(keyVaultId.Name)
	keysmith.Lock()
	if lock[cacheKey] == nil {
		lock[cacheKey] = &sync.RWMutex{}
	}
	keysmith.Unlock()
	lock[cacheKey].Lock()
	defer lock[cacheKey].Unlock()

	if _, ok := keyVaultsCache[cacheKey]; ok {
		return true, nil
	}

	resp, err := c.keyVaultClient.Get(ctx, keyVaultId.ResourceGroup, keyVaultId.Name)
	if err != nil {
		if response.ResponseWasNotFound(resp.Response) {
			return false, nil
		}
		return false, fmt.Errorf("retrieving %s: %+v", keyVaultId, err)
	}

	if resp.Properties == nil || resp.Properties.VaultURI == nil {
		return false, fmt.Errorf("`properties` was nil for %s", keyVaultId)
	}

	c.AddKeyVaultToCache(keyVaultId, *resp.Properties.VaultURI)

	return true, nil
}
