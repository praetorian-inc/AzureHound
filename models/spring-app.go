package models

import "github.com/praetorian-inc/AzureHound/v2/models/azure"

type SpringApp struct {
	azure.SpringApp
	SubscriptionId     string `json:"subscriptionId"`
	ResourceGroupId    string `json:"resourceGroupId"`
	ResourceGroupName  string `json:"resourceGroupName"`
	SpringAppServiceId string `json:"springAppServiceId"`
	TenantId           string `json:"tenantId"`
}
