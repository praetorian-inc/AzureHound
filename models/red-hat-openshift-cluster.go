package models

import "github.com/praetorian-inc/AzureHound/v2/models/azure"

type RedHatOpenShiftCluster struct {
	azure.RedHatOpenShiftCluster
	SubscriptionId  string `json:"subscriptionId"`
	ResourceGroupId string `json:"resourceGroupId"`
	TenantId        string `json:"tenantId"`
}
