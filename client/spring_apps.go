package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/client/query"
	"github.com/praetorian-inc/AzureHound/models/azure"
)

// ListAzureSpringApps https://learn.microsoft.com/en-us/rest/api/azurespringapps/apps/list?view=rest-azurespringapps-2023-12-01
func (s *azureClient) ListAzureSpringApps(ctx context.Context, subscriptionId string, resourceGroupName string, serviceName string) <-chan AzureResult[azure.SpringApp] {
	var (
		out    = make(chan AzureResult[azure.SpringApp])
		path   = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.AppPlatform/Spring/%s/apps", subscriptionId, resourceGroupName, serviceName)
		params = query.RMParams{ApiVersion: "2022-12-01"}
	)

	go getAzureObjectList[azure.SpringApp](s.resourceManager, ctx, path, params, out)

	return out
}
