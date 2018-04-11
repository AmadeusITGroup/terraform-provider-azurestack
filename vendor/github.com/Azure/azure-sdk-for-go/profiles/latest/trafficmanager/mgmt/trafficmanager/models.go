// +build go1.9

// Copyright 2018 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This code was auto-generated by:
// github.com/Azure/azure-sdk-for-go/tools/profileBuilder

package trafficmanager

import original "github.com/Azure/azure-sdk-for-go/services/trafficmanager/mgmt/2017-05-01/trafficmanager"

const (
	DefaultBaseURI = original.DefaultBaseURI
)

type BaseClient = original.BaseClient

func New(subscriptionID string) BaseClient {
	return original.New(subscriptionID)
}
func NewWithBaseURI(baseURI string, subscriptionID string) BaseClient {
	return original.NewWithBaseURI(baseURI, subscriptionID)
}

type EndpointsClient = original.EndpointsClient

func NewEndpointsClient(subscriptionID string) EndpointsClient {
	return original.NewEndpointsClient(subscriptionID)
}
func NewEndpointsClientWithBaseURI(baseURI string, subscriptionID string) EndpointsClient {
	return original.NewEndpointsClientWithBaseURI(baseURI, subscriptionID)
}

type GeographicHierarchiesClient = original.GeographicHierarchiesClient

func NewGeographicHierarchiesClient(subscriptionID string) GeographicHierarchiesClient {
	return original.NewGeographicHierarchiesClient(subscriptionID)
}
func NewGeographicHierarchiesClientWithBaseURI(baseURI string, subscriptionID string) GeographicHierarchiesClient {
	return original.NewGeographicHierarchiesClientWithBaseURI(baseURI, subscriptionID)
}

type EndpointMonitorStatus = original.EndpointMonitorStatus

const (
	CheckingEndpoint EndpointMonitorStatus = original.CheckingEndpoint
	Degraded         EndpointMonitorStatus = original.Degraded
	Disabled         EndpointMonitorStatus = original.Disabled
	Inactive         EndpointMonitorStatus = original.Inactive
	Online           EndpointMonitorStatus = original.Online
	Stopped          EndpointMonitorStatus = original.Stopped
)

func PossibleEndpointMonitorStatusValues() []EndpointMonitorStatus {
	return original.PossibleEndpointMonitorStatusValues()
}

type EndpointStatus = original.EndpointStatus

const (
	EndpointStatusDisabled EndpointStatus = original.EndpointStatusDisabled
	EndpointStatusEnabled  EndpointStatus = original.EndpointStatusEnabled
)

func PossibleEndpointStatusValues() []EndpointStatus {
	return original.PossibleEndpointStatusValues()
}

type MonitorProtocol = original.MonitorProtocol

const (
	HTTP  MonitorProtocol = original.HTTP
	HTTPS MonitorProtocol = original.HTTPS
	TCP   MonitorProtocol = original.TCP
)

func PossibleMonitorProtocolValues() []MonitorProtocol {
	return original.PossibleMonitorProtocolValues()
}

type ProfileMonitorStatus = original.ProfileMonitorStatus

const (
	ProfileMonitorStatusCheckingEndpoints ProfileMonitorStatus = original.ProfileMonitorStatusCheckingEndpoints
	ProfileMonitorStatusDegraded          ProfileMonitorStatus = original.ProfileMonitorStatusDegraded
	ProfileMonitorStatusDisabled          ProfileMonitorStatus = original.ProfileMonitorStatusDisabled
	ProfileMonitorStatusInactive          ProfileMonitorStatus = original.ProfileMonitorStatusInactive
	ProfileMonitorStatusOnline            ProfileMonitorStatus = original.ProfileMonitorStatusOnline
)

func PossibleProfileMonitorStatusValues() []ProfileMonitorStatus {
	return original.PossibleProfileMonitorStatusValues()
}

type ProfileStatus = original.ProfileStatus

const (
	ProfileStatusDisabled ProfileStatus = original.ProfileStatusDisabled
	ProfileStatusEnabled  ProfileStatus = original.ProfileStatusEnabled
)

func PossibleProfileStatusValues() []ProfileStatus {
	return original.PossibleProfileStatusValues()
}

type TrafficRoutingMethod = original.TrafficRoutingMethod

const (
	Geographic  TrafficRoutingMethod = original.Geographic
	Performance TrafficRoutingMethod = original.Performance
	Priority    TrafficRoutingMethod = original.Priority
	Weighted    TrafficRoutingMethod = original.Weighted
)

func PossibleTrafficRoutingMethodValues() []TrafficRoutingMethod {
	return original.PossibleTrafficRoutingMethodValues()
}

type CheckTrafficManagerRelativeDNSNameAvailabilityParameters = original.CheckTrafficManagerRelativeDNSNameAvailabilityParameters
type CloudError = original.CloudError
type CloudErrorBody = original.CloudErrorBody
type DeleteOperationResult = original.DeleteOperationResult
type DNSConfig = original.DNSConfig
type Endpoint = original.Endpoint
type EndpointProperties = original.EndpointProperties
type GeographicHierarchy = original.GeographicHierarchy
type GeographicHierarchyProperties = original.GeographicHierarchyProperties
type MonitorConfig = original.MonitorConfig
type NameAvailability = original.NameAvailability
type Profile = original.Profile
type ProfileListResult = original.ProfileListResult
type ProfileProperties = original.ProfileProperties
type ProxyResource = original.ProxyResource
type Region = original.Region
type Resource = original.Resource
type TrackedResource = original.TrackedResource
type ProfilesClient = original.ProfilesClient

func NewProfilesClient(subscriptionID string) ProfilesClient {
	return original.NewProfilesClient(subscriptionID)
}
func NewProfilesClientWithBaseURI(baseURI string, subscriptionID string) ProfilesClient {
	return original.NewProfilesClientWithBaseURI(baseURI, subscriptionID)
}
func UserAgent() string {
	return original.UserAgent() + " profiles/latest"
}
func Version() string {
	return original.Version()
}
