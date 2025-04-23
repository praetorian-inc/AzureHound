package models

import "github.com/praetorian-inc/AzureHound/v2/models/azure"

type RBACRoleDefinition struct {
	azure.RBACRoleDefinition
	SubscriptionId       string `json:"subscriptionId"`
	RBACRoleDefinitionId string `json:"RBACRoleDefinitionId"`
	TenantId             string `json:"tenantId"`
}
