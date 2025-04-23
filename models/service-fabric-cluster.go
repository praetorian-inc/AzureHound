package models

import "github.com/praetorian-inc/AzureHound/models/azure"

type ServiceFabricCluster struct {
	azure.ServiceFabricCluster
	SubscriptionId    string `json:"subscriptionId"`
	ResourceGroupId   string `json:"resourceGroupId"`
	ResourceGroupName string `json:"resourceGroupName"`
	TenantId          string `json:"tenantId"`
}
