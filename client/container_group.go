package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/client/query"
	"github.com/praetorian-inc/AzureHound/models/azure"
)

// ListAzureContainerGroups https://learn.microsoft.com/en-us/rest/api/container-instances/container-groups/list?view=rest-container-instances-2023-05-01
func (s *azureClient) ListAzureContainerGroups(ctx context.Context, subscriptionId string) <-chan AzureResult[azure.ContainerGroup] {
	var (
		out    = make(chan AzureResult[azure.ContainerGroup])
		path   = fmt.Sprintf("/subscriptions/%s/providers/Microsoft.ContainerInstance/containerGroups", subscriptionId)
		params = query.RMParams{ApiVersion: "2023-05-01"}
	)

	go getAzureObjectList[azure.ContainerGroup](s.resourceManager, ctx, path, params, out)

	return out
}
