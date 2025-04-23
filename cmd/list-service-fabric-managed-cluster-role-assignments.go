package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"sync"
	"time"

	"github.com/praetorian-inc/AzureHound/v2/client"
	"github.com/praetorian-inc/AzureHound/v2/config"
	"github.com/praetorian-inc/AzureHound/v2/enums"
	"github.com/praetorian-inc/AzureHound/v2/models"
	"github.com/praetorian-inc/AzureHound/v2/panicrecovery"
	"github.com/praetorian-inc/AzureHound/v2/pipeline"
	"github.com/spf13/cobra"
)

func init() {
	listRootCmd.AddCommand(listServiceFabricManagedClusterRoleAssignment)
}

var listServiceFabricManagedClusterRoleAssignment = &cobra.Command{
	Use:          "service-fabric-managed-cluster-role-assignments",
	Long:         "Lists Azure Service Fabric Managed Cluster Role Assignments",
	Run:          listServiceFabricManagedClusterRoleAssignmentImpl,
	SilenceUsage: true,
}

func listServiceFabricManagedClusterRoleAssignmentImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure service fabric managed cluster role assignments...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		stream := listServiceFabricManagedClusterRoleAssignments(ctx, azClient, listServiceFabricManagedClusters(ctx, azClient, subscriptions))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listServiceFabricManagedClusterRoleAssignments(ctx context.Context, client client.AzureClient, ServiceFabricManagedClusters <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)

		for result := range pipeline.OrDone(ctx.Done(), ServiceFabricManagedClusters) {
			if ServiceFabricManagedCluster, ok := result.(AzureWrapper).Data.(models.ServiceFabricManagedCluster); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating service fabric managed cluster role assignments", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, ServiceFabricManagedCluster.Id); !ok {
					return
				}
			}
		}
	}()

	wg.Add(len(streams))
	for i := range streams {
		stream := streams[i]
		go func() {
			defer panicrecovery.PanicRecovery()
			defer wg.Done()
			for id := range stream {
				var (
					serviceFabricManagedClusterRoleAssignments = models.AzureRoleAssignments{
						ObjectId: id,
					}
					count = 0
				)
				for item := range client.ListRoleAssignmentsForResource(ctx, id, "", "") {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing role assignments for this service fabric managed cluster", "ServiceFabricManagedClusterId", id)
					} else {
						roleDefinitionId := path.Base(item.Ok.Properties.RoleDefinitionId)

						serviceFabricManagedClusterRoleAssignment := models.AzureRoleAssignment{
							Assignee:         item.Ok,
							ObjectId:         id,
							RoleDefinitionId: roleDefinitionId,
						}
						log.V(2).Info("found service fabric managed cluster role assignment", "serviceFabricManagedClusterRoleAssignment", serviceFabricManagedClusterRoleAssignment)
						count++
						serviceFabricManagedClusterRoleAssignments.RoleAssignments = append(serviceFabricManagedClusterRoleAssignments.RoleAssignments, serviceFabricManagedClusterRoleAssignment)
					}
				}
				if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
					Kind: enums.KindAZServiceFabricManagedClusterRoleAssignment,
					Data: serviceFabricManagedClusterRoleAssignments,
				}); !ok {
					return
				}
				log.V(1).Info("finished listing service fabric managed cluster role assignments", "ServiceFabricManagedClusterId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all service fabric managed cluster role assignments")
	}()

	return out
}
