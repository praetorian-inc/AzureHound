package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/v2/client/query"
	"github.com/praetorian-inc/AzureHound/v2/models/azure"
)

// ListAzureSpringAppServices https://learn.microsoft.com/en-us/rest/api/azurespringapps/services/list-by-subscription?view=rest-azurespringapps-2023-12-01
func (s *azureClient) ListAzureSpringAppServices(ctx context.Context, subscriptionId string) <-chan AzureResult[azure.SpringAppService] {
	var (
		out    = make(chan AzureResult[azure.SpringAppService])
		path   = fmt.Sprintf("/subscriptions/%s/providers/Microsoft.AppPlatform/Spring", subscriptionId)
		params = query.RMParams{ApiVersion: "2022-12-01"}
	)

	go getAzureObjectList[azure.SpringAppService](s.resourceManager, ctx, path, params, out)

	return out
}
