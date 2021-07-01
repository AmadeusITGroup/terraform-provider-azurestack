package securityinsight

// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Code generated by Microsoft (R) AutoRest Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"context"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/validation"
	"github.com/Azure/go-autorest/tracing"
	"net/http"
)

// IncidentCommentsClient is the API spec for Microsoft.SecurityInsights (Azure Security Insights) resource provider
type IncidentCommentsClient struct {
	BaseClient
}

// NewIncidentCommentsClient creates an instance of the IncidentCommentsClient client.
func NewIncidentCommentsClient(subscriptionID string) IncidentCommentsClient {
	return NewIncidentCommentsClientWithBaseURI(DefaultBaseURI, subscriptionID)
}

// NewIncidentCommentsClientWithBaseURI creates an instance of the IncidentCommentsClient client using a custom
// endpoint.  Use this when interacting with an Azure cloud that uses a non-standard base URI (sovereign clouds, Azure
// stack).
func NewIncidentCommentsClientWithBaseURI(baseURI string, subscriptionID string) IncidentCommentsClient {
	return IncidentCommentsClient{NewWithBaseURI(baseURI, subscriptionID)}
}

// CreateComment creates or updates the incident comment.
// Parameters:
// resourceGroupName - the name of the resource group within the user's subscription. The name is case
// insensitive.
// operationalInsightsResourceProvider - the namespace of workspaces resource provider-
// Microsoft.OperationalInsights.
// workspaceName - the name of the workspace.
// incidentID - incident ID
// incidentCommentID - incident comment ID
// incidentComment - the incident comment
func (client IncidentCommentsClient) CreateComment(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, incidentCommentID string, incidentComment IncidentComment) (result IncidentComment, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/IncidentCommentsClient.CreateComment")
		defer func() {
			sc := -1
			if result.Response.Response != nil {
				sc = result.Response.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	if err := validation.Validate([]validation.Validation{
		{TargetValue: client.SubscriptionID,
			Constraints: []validation.Constraint{{Target: "client.SubscriptionID", Name: validation.Pattern, Rule: `^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$`, Chain: nil}}},
		{TargetValue: resourceGroupName,
			Constraints: []validation.Constraint{{Target: "resourceGroupName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "resourceGroupName", Name: validation.MinLength, Rule: 1, Chain: nil},
				{Target: "resourceGroupName", Name: validation.Pattern, Rule: `^[-\w\._\(\)]+$`, Chain: nil}}},
		{TargetValue: workspaceName,
			Constraints: []validation.Constraint{{Target: "workspaceName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "workspaceName", Name: validation.MinLength, Rule: 1, Chain: nil}}},
		{TargetValue: incidentComment,
			Constraints: []validation.Constraint{{Target: "incidentComment.IncidentCommentProperties", Name: validation.Null, Rule: false,
				Chain: []validation.Constraint{{Target: "incidentComment.IncidentCommentProperties.Message", Name: validation.Null, Rule: true, Chain: nil}}}}}}); err != nil {
		return result, validation.NewError("securityinsight.IncidentCommentsClient", "CreateComment", err.Error())
	}

	req, err := client.CreateCommentPreparer(ctx, resourceGroupName, operationalInsightsResourceProvider, workspaceName, incidentID, incidentCommentID, incidentComment)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "CreateComment", nil, "Failure preparing request")
		return
	}

	resp, err := client.CreateCommentSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "CreateComment", resp, "Failure sending request")
		return
	}

	result, err = client.CreateCommentResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "CreateComment", resp, "Failure responding to request")
		return
	}

	return
}

// CreateCommentPreparer prepares the CreateComment request.
func (client IncidentCommentsClient) CreateCommentPreparer(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, incidentCommentID string, incidentComment IncidentComment) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"incidentCommentId":                   autorest.Encode("path", incidentCommentID),
		"incidentId":                          autorest.Encode("path", incidentID),
		"operationalInsightsResourceProvider": autorest.Encode("path", operationalInsightsResourceProvider),
		"resourceGroupName":                   autorest.Encode("path", resourceGroupName),
		"subscriptionId":                      autorest.Encode("path", client.SubscriptionID),
		"workspaceName":                       autorest.Encode("path", workspaceName),
	}

	const APIVersion = "2019-01-01-preview"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/json; charset=utf-8"),
		autorest.AsPut(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/{operationalInsightsResourceProvider}/workspaces/{workspaceName}/providers/Microsoft.SecurityInsights/incidents/{incidentId}/comments/{incidentCommentId}", pathParameters),
		autorest.WithJSON(incidentComment),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// CreateCommentSender sends the CreateComment request. The method will close the
// http.Response Body if it receives an error.
func (client IncidentCommentsClient) CreateCommentSender(req *http.Request) (*http.Response, error) {
	return client.Send(req, azure.DoRetryWithRegistration(client.Client))
}

// CreateCommentResponder handles the response to the CreateComment request. The method always
// closes the http.Response Body.
func (client IncidentCommentsClient) CreateCommentResponder(resp *http.Response) (result IncidentComment, err error) {
	err = autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// DeleteComment delete the incident comment.
// Parameters:
// resourceGroupName - the name of the resource group within the user's subscription. The name is case
// insensitive.
// operationalInsightsResourceProvider - the namespace of workspaces resource provider-
// Microsoft.OperationalInsights.
// workspaceName - the name of the workspace.
// incidentID - incident ID
// incidentCommentID - incident comment ID
func (client IncidentCommentsClient) DeleteComment(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, incidentCommentID string) (result autorest.Response, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/IncidentCommentsClient.DeleteComment")
		defer func() {
			sc := -1
			if result.Response != nil {
				sc = result.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	if err := validation.Validate([]validation.Validation{
		{TargetValue: client.SubscriptionID,
			Constraints: []validation.Constraint{{Target: "client.SubscriptionID", Name: validation.Pattern, Rule: `^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$`, Chain: nil}}},
		{TargetValue: resourceGroupName,
			Constraints: []validation.Constraint{{Target: "resourceGroupName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "resourceGroupName", Name: validation.MinLength, Rule: 1, Chain: nil},
				{Target: "resourceGroupName", Name: validation.Pattern, Rule: `^[-\w\._\(\)]+$`, Chain: nil}}},
		{TargetValue: workspaceName,
			Constraints: []validation.Constraint{{Target: "workspaceName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "workspaceName", Name: validation.MinLength, Rule: 1, Chain: nil}}}}); err != nil {
		return result, validation.NewError("securityinsight.IncidentCommentsClient", "DeleteComment", err.Error())
	}

	req, err := client.DeleteCommentPreparer(ctx, resourceGroupName, operationalInsightsResourceProvider, workspaceName, incidentID, incidentCommentID)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "DeleteComment", nil, "Failure preparing request")
		return
	}

	resp, err := client.DeleteCommentSender(req)
	if err != nil {
		result.Response = resp
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "DeleteComment", resp, "Failure sending request")
		return
	}

	result, err = client.DeleteCommentResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "DeleteComment", resp, "Failure responding to request")
		return
	}

	return
}

// DeleteCommentPreparer prepares the DeleteComment request.
func (client IncidentCommentsClient) DeleteCommentPreparer(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, incidentCommentID string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"incidentCommentId":                   autorest.Encode("path", incidentCommentID),
		"incidentId":                          autorest.Encode("path", incidentID),
		"operationalInsightsResourceProvider": autorest.Encode("path", operationalInsightsResourceProvider),
		"resourceGroupName":                   autorest.Encode("path", resourceGroupName),
		"subscriptionId":                      autorest.Encode("path", client.SubscriptionID),
		"workspaceName":                       autorest.Encode("path", workspaceName),
	}

	const APIVersion = "2019-01-01-preview"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsDelete(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/{operationalInsightsResourceProvider}/workspaces/{workspaceName}/providers/Microsoft.SecurityInsights/incidents/{incidentId}/comments/{incidentCommentId}", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// DeleteCommentSender sends the DeleteComment request. The method will close the
// http.Response Body if it receives an error.
func (client IncidentCommentsClient) DeleteCommentSender(req *http.Request) (*http.Response, error) {
	return client.Send(req, azure.DoRetryWithRegistration(client.Client))
}

// DeleteCommentResponder handles the response to the DeleteComment request. The method always
// closes the http.Response Body.
func (client IncidentCommentsClient) DeleteCommentResponder(resp *http.Response) (result autorest.Response, err error) {
	err = autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusNoContent),
		autorest.ByClosing())
	result.Response = resp
	return
}

// GetComment gets an incident comment.
// Parameters:
// resourceGroupName - the name of the resource group within the user's subscription. The name is case
// insensitive.
// operationalInsightsResourceProvider - the namespace of workspaces resource provider-
// Microsoft.OperationalInsights.
// workspaceName - the name of the workspace.
// incidentID - incident ID
// incidentCommentID - incident comment ID
func (client IncidentCommentsClient) GetComment(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, incidentCommentID string) (result IncidentComment, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/IncidentCommentsClient.GetComment")
		defer func() {
			sc := -1
			if result.Response.Response != nil {
				sc = result.Response.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	if err := validation.Validate([]validation.Validation{
		{TargetValue: client.SubscriptionID,
			Constraints: []validation.Constraint{{Target: "client.SubscriptionID", Name: validation.Pattern, Rule: `^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$`, Chain: nil}}},
		{TargetValue: resourceGroupName,
			Constraints: []validation.Constraint{{Target: "resourceGroupName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "resourceGroupName", Name: validation.MinLength, Rule: 1, Chain: nil},
				{Target: "resourceGroupName", Name: validation.Pattern, Rule: `^[-\w\._\(\)]+$`, Chain: nil}}},
		{TargetValue: workspaceName,
			Constraints: []validation.Constraint{{Target: "workspaceName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "workspaceName", Name: validation.MinLength, Rule: 1, Chain: nil}}}}); err != nil {
		return result, validation.NewError("securityinsight.IncidentCommentsClient", "GetComment", err.Error())
	}

	req, err := client.GetCommentPreparer(ctx, resourceGroupName, operationalInsightsResourceProvider, workspaceName, incidentID, incidentCommentID)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "GetComment", nil, "Failure preparing request")
		return
	}

	resp, err := client.GetCommentSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "GetComment", resp, "Failure sending request")
		return
	}

	result, err = client.GetCommentResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "GetComment", resp, "Failure responding to request")
		return
	}

	return
}

// GetCommentPreparer prepares the GetComment request.
func (client IncidentCommentsClient) GetCommentPreparer(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, incidentCommentID string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"incidentCommentId":                   autorest.Encode("path", incidentCommentID),
		"incidentId":                          autorest.Encode("path", incidentID),
		"operationalInsightsResourceProvider": autorest.Encode("path", operationalInsightsResourceProvider),
		"resourceGroupName":                   autorest.Encode("path", resourceGroupName),
		"subscriptionId":                      autorest.Encode("path", client.SubscriptionID),
		"workspaceName":                       autorest.Encode("path", workspaceName),
	}

	const APIVersion = "2019-01-01-preview"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/{operationalInsightsResourceProvider}/workspaces/{workspaceName}/providers/Microsoft.SecurityInsights/incidents/{incidentId}/comments/{incidentCommentId}", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// GetCommentSender sends the GetComment request. The method will close the
// http.Response Body if it receives an error.
func (client IncidentCommentsClient) GetCommentSender(req *http.Request) (*http.Response, error) {
	return client.Send(req, azure.DoRetryWithRegistration(client.Client))
}

// GetCommentResponder handles the response to the GetComment request. The method always
// closes the http.Response Body.
func (client IncidentCommentsClient) GetCommentResponder(resp *http.Response) (result IncidentComment, err error) {
	err = autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// ListByIncident gets all incident comments.
// Parameters:
// resourceGroupName - the name of the resource group within the user's subscription. The name is case
// insensitive.
// operationalInsightsResourceProvider - the namespace of workspaces resource provider-
// Microsoft.OperationalInsights.
// workspaceName - the name of the workspace.
// incidentID - incident ID
// filter - filters the results, based on a Boolean condition. Optional.
// orderby - sorts the results. Optional.
// top - returns only the first n results. Optional.
// skipToken - skiptoken is only used if a previous operation returned a partial result. If a previous response
// contains a nextLink element, the value of the nextLink element will include a skiptoken parameter that
// specifies a starting point to use for subsequent calls. Optional.
func (client IncidentCommentsClient) ListByIncident(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, filter string, orderby string, top *int32, skipToken string) (result IncidentCommentListPage, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/IncidentCommentsClient.ListByIncident")
		defer func() {
			sc := -1
			if result.icl.Response.Response != nil {
				sc = result.icl.Response.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	if err := validation.Validate([]validation.Validation{
		{TargetValue: client.SubscriptionID,
			Constraints: []validation.Constraint{{Target: "client.SubscriptionID", Name: validation.Pattern, Rule: `^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$`, Chain: nil}}},
		{TargetValue: resourceGroupName,
			Constraints: []validation.Constraint{{Target: "resourceGroupName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "resourceGroupName", Name: validation.MinLength, Rule: 1, Chain: nil},
				{Target: "resourceGroupName", Name: validation.Pattern, Rule: `^[-\w\._\(\)]+$`, Chain: nil}}},
		{TargetValue: workspaceName,
			Constraints: []validation.Constraint{{Target: "workspaceName", Name: validation.MaxLength, Rule: 90, Chain: nil},
				{Target: "workspaceName", Name: validation.MinLength, Rule: 1, Chain: nil}}}}); err != nil {
		return result, validation.NewError("securityinsight.IncidentCommentsClient", "ListByIncident", err.Error())
	}

	result.fn = client.listByIncidentNextResults
	req, err := client.ListByIncidentPreparer(ctx, resourceGroupName, operationalInsightsResourceProvider, workspaceName, incidentID, filter, orderby, top, skipToken)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "ListByIncident", nil, "Failure preparing request")
		return
	}

	resp, err := client.ListByIncidentSender(req)
	if err != nil {
		result.icl.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "ListByIncident", resp, "Failure sending request")
		return
	}

	result.icl, err = client.ListByIncidentResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "ListByIncident", resp, "Failure responding to request")
		return
	}
	if result.icl.hasNextLink() && result.icl.IsEmpty() {
		err = result.NextWithContext(ctx)
		return
	}

	return
}

// ListByIncidentPreparer prepares the ListByIncident request.
func (client IncidentCommentsClient) ListByIncidentPreparer(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, filter string, orderby string, top *int32, skipToken string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"incidentId":                          autorest.Encode("path", incidentID),
		"operationalInsightsResourceProvider": autorest.Encode("path", operationalInsightsResourceProvider),
		"resourceGroupName":                   autorest.Encode("path", resourceGroupName),
		"subscriptionId":                      autorest.Encode("path", client.SubscriptionID),
		"workspaceName":                       autorest.Encode("path", workspaceName),
	}

	const APIVersion = "2019-01-01-preview"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}
	if len(filter) > 0 {
		queryParameters["$filter"] = autorest.Encode("query", filter)
	}
	if len(orderby) > 0 {
		queryParameters["$orderby"] = autorest.Encode("query", orderby)
	}
	if top != nil {
		queryParameters["$top"] = autorest.Encode("query", *top)
	}
	if len(skipToken) > 0 {
		queryParameters["$skipToken"] = autorest.Encode("query", skipToken)
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/{operationalInsightsResourceProvider}/workspaces/{workspaceName}/providers/Microsoft.SecurityInsights/incidents/{incidentId}/comments", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// ListByIncidentSender sends the ListByIncident request. The method will close the
// http.Response Body if it receives an error.
func (client IncidentCommentsClient) ListByIncidentSender(req *http.Request) (*http.Response, error) {
	return client.Send(req, azure.DoRetryWithRegistration(client.Client))
}

// ListByIncidentResponder handles the response to the ListByIncident request. The method always
// closes the http.Response Body.
func (client IncidentCommentsClient) ListByIncidentResponder(resp *http.Response) (result IncidentCommentList, err error) {
	err = autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// listByIncidentNextResults retrieves the next set of results, if any.
func (client IncidentCommentsClient) listByIncidentNextResults(ctx context.Context, lastResults IncidentCommentList) (result IncidentCommentList, err error) {
	req, err := lastResults.incidentCommentListPreparer(ctx)
	if err != nil {
		return result, autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "listByIncidentNextResults", nil, "Failure preparing next results request")
	}
	if req == nil {
		return
	}
	resp, err := client.ListByIncidentSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		return result, autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "listByIncidentNextResults", resp, "Failure sending next results request")
	}
	result, err = client.ListByIncidentResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "securityinsight.IncidentCommentsClient", "listByIncidentNextResults", resp, "Failure responding to next results request")
	}
	return
}

// ListByIncidentComplete enumerates all values, automatically crossing page boundaries as required.
func (client IncidentCommentsClient) ListByIncidentComplete(ctx context.Context, resourceGroupName string, operationalInsightsResourceProvider string, workspaceName string, incidentID string, filter string, orderby string, top *int32, skipToken string) (result IncidentCommentListIterator, err error) {
	if tracing.IsEnabled() {
		ctx = tracing.StartSpan(ctx, fqdn+"/IncidentCommentsClient.ListByIncident")
		defer func() {
			sc := -1
			if result.Response().Response.Response != nil {
				sc = result.page.Response().Response.Response.StatusCode
			}
			tracing.EndSpan(ctx, sc, err)
		}()
	}
	result.page, err = client.ListByIncident(ctx, resourceGroupName, operationalInsightsResourceProvider, workspaceName, incidentID, filter, orderby, top, skipToken)
	return
}
