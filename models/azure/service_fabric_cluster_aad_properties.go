package azure

type ServiceFabricClusterAADProperties struct {
	ClientApplication  string `json:"clientApplication,omitempty"`
	ClusterApplication string `json:"clusterApplication,omitempty"`
	TenantId           string `json:"tenantId,omitempty"`
}
