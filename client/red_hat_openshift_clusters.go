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

package client

import (
	"context"
	"fmt"

	"github.com/praetorian-inc/AzureHound/client/query"
	"github.com/praetorian-inc/AzureHound/models/azure"
)

// ListAzureRedHatOpenShiftCluster https://learn.microsoft.com/en-us/rest/api/openshift/open-shift-clusters/list?view=rest-openshift-2023-11-22
func (s *azureClient) ListAzureRedHatOpenShiftClusters(ctx context.Context, subscriptionId string) <-chan AzureResult[azure.RedHatOpenShiftCluster] {
	var (
		out    = make(chan AzureResult[azure.RedHatOpenShiftCluster])
		path   = fmt.Sprintf("/subscriptions/%s/providers/Microsoft.RedHatOpenShift/openShiftClusters", subscriptionId)
		params = query.RMParams{ApiVersion: "2023-11-22"}
	)

	go getAzureObjectList[azure.RedHatOpenShiftCluster](s.resourceManager, ctx, path, params, out)

	return out
}
