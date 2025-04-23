package models

import "github.com/praetorian-inc/AzureHound/models/azure"

type RedHatOpenShiftCluster struct {
	azure.RedHatOpenShiftCluster
	SubscriptionId  string `json:"subscriptionId"`
	ResourceGroupId string `json:"resourceGroupId"`
	TenantId        string `json:"tenantId"`
}
