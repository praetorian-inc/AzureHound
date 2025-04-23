package models

import "github.com/praetorian-inc/AzureHound/models/azure"

type ContainerApp struct {
	azure.ContainerApp
	SubscriptionId  string `json:"subscriptionId"`
	ResourceGroupId string `json:"resourceGroupId"`
	TenantId        string `json:"tenantId"`
}
