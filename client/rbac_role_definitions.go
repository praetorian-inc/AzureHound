package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/v2/client/query"
	"github.com/praetorian-inc/AzureHound/v2/models/azure"
)

// ListAzureRBACRoleDefinitions https://learn.microsoft.com/en-us/rest/api/authorization/role-definitions/list?view=rest-authorization-2022-04-01
func (s *azureClient) ListAzureRBACRoleDefinitions(ctx context.Context, subscriptionId string) <-chan AzureResult[azure.RBACRoleDefinition] {
	var (
		out    = make(chan AzureResult[azure.RBACRoleDefinition])
		path   = fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions", subscriptionId)
		params = query.RMParams{ApiVersion: "2018-07-01"}
	)

	go getAzureObjectList[azure.RBACRoleDefinition](s.resourceManager, ctx, path, params, out)

	return out
}
