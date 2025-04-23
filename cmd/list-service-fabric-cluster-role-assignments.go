package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"sync"
	"time"

	"github.com/praetorian-inc/AzureHound/client"
	"github.com/praetorian-inc/AzureHound/config"
	"github.com/praetorian-inc/AzureHound/enums"
	"github.com/praetorian-inc/AzureHound/models"
	"github.com/praetorian-inc/AzureHound/panicrecovery"
	"github.com/praetorian-inc/AzureHound/pipeline"
	"github.com/spf13/cobra"
)

func init() {
	listRootCmd.AddCommand(listServiceFabricClusterRoleAssignment)
}

var listServiceFabricClusterRoleAssignment = &cobra.Command{
	Use:          "service-fabric-cluster-role-assignments",
	Long:         "Lists Azure Service Fabric Cluster Role Assignments",
	Run:          listServiceFabricClusterRoleAssignmentImpl,
	SilenceUsage: true,
}

func listServiceFabricClusterRoleAssignmentImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure service fabric cluster role assignments...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		stream := listServiceFabricClusterRoleAssignments(ctx, azClient, listServiceFabricClusters(ctx, azClient, subscriptions))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listServiceFabricClusterRoleAssignments(ctx context.Context, client client.AzureClient, serviceFabricClusters <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)

		for result := range pipeline.OrDone(ctx.Done(), serviceFabricClusters) {
			if serviceFabricCluster, ok := result.(AzureWrapper).Data.(models.ServiceFabricCluster); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating service fabric cluster role assignments", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, serviceFabricCluster.Id); !ok {
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
					serviceFabricClusterRoleAssignments = models.AzureRoleAssignments{
						ObjectId: id,
					}
					count = 0
				)
				for item := range client.ListRoleAssignmentsForResource(ctx, id, "", "") {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing role assignments for this service fabric cluster", "serviceFabricClusterId", id)
					} else {
						roleDefinitionId := path.Base(item.Ok.Properties.RoleDefinitionId)

						serviceFabricClusterRoleAssignment := models.AzureRoleAssignment{
							Assignee:         item.Ok,
							ObjectId:         id,
							RoleDefinitionId: roleDefinitionId,
						}
						log.V(2).Info("found service fabric cluster role assignment", "serviceFabricClusterRoleAssignment", serviceFabricClusterRoleAssignment)
						count++
						serviceFabricClusterRoleAssignments.RoleAssignments = append(serviceFabricClusterRoleAssignments.RoleAssignments, serviceFabricClusterRoleAssignment)
					}
				}
				if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
					Kind: enums.KindAZServiceFabricClusterRoleAssignment,
					Data: serviceFabricClusterRoleAssignments,
				}); !ok {
					return
				}
				log.V(1).Info("finished listing service fabric cluster role assignments", "serviceFabricClusterId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all service fabric cluster role assignments")
	}()

	return out
}
