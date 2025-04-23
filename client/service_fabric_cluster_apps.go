package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/client/query"
	"github.com/praetorian-inc/AzureHound/models/azure"
)

// ListAzureServiceFabricClusterApps https://learn.microsoft.com/en-us/rest/api/servicefabric/application/applications/list?view=rest-servicefabric-application-2023-11-01-preview
func (s *azureClient) ListAzureServiceFabricClusterApps(ctx context.Context, subscriptionId string, resourceGroupName string, clusterName string) <-chan AzureResult[azure.ServiceFabricClusterApp] {
	var (
		out    = make(chan AzureResult[azure.ServiceFabricClusterApp])
		path   = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ServiceFabric/clusters/%s/applications", subscriptionId, resourceGroupName, clusterName)
		params = query.RMParams{ApiVersion: "2023-11-01-preview"}
	)

	go getAzureObjectList[azure.ServiceFabricClusterApp](s.resourceManager, ctx, path, params, out)

	return out
}
