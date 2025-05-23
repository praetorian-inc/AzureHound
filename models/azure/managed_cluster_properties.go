// Copyright (C) 2022 Specter Ops, Inc.
//
// This file is part of AzureHound.
//
// AzureHound is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// AzureHound is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package azure

// Properties of the vault
type ManagedClusterProperties struct {
	// The name of the AzureRM Resource Group the Managed Cluster's Virtual Machine Scale Set resides
	AzurePortalFQDN         string                                        `json:"azurePortalFQDN,omitempty"`
	EnableRBAC              bool                                          `json:"enableRBAC,omitempty"`
	FQDN                    string                                        `json:"fqdn,omitempty"`
	IdentityProfile         map[string]ManagedClusterUserAssignedIdentity `json:"identityProfile,omitempty"`
	NodeResourceGroup       string                                        `json:"nodeResourceGroup,omitempty"`
	PodIdentityProfile      ManagedClusterPodIdentityProfile              `json:"podIdentityProfile,omitempty"`
	PublicNetworkAccess     string                                        `json:"publicNetworkAccess,omitempty"`
	ServicePrincipalProfile ManagedClusterServicePrincipalProfile         `json:"servicePrincipalProfile,omitempty"`
	WindowsProfile          ManagedClusterWindowsProfile                  `json:"windowsProfile,omitempty"`
}
