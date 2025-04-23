package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/v2/client/query"
	"github.com/praetorian-inc/AzureHound/v2/models/azure"
)

// ListAzureContainerApps https://learn.microsoft.com/en-us/rest/api/resource-manager/containerapps/container-apps/list-by-subscription
func (s *azureClient) ListAzureContainerApps(ctx context.Context, subscriptionId string) <-chan AzureResult[azure.ContainerApp] {
	var (
		out    = make(chan AzureResult[azure.ContainerApp])
		path   = fmt.Sprintf("/subscriptions/%s/providers/Microsoft.App/containerApps", subscriptionId)
		params = query.RMParams{ApiVersion: "2024-03-01"}
	)

	go getAzureObjectList[azure.ContainerApp](s.resourceManager, ctx, path, params, out)

	return out
}
