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
	listRootCmd.AddCommand(listContainerAppRoleAssignmentCmd)
}

var listContainerAppRoleAssignmentCmd = &cobra.Command{
	Use:          "container-app-role-assignments",
	Long:         "Lists Azure Container App Role Assignments",
	Run:          listContainerAppRoleAssignmentsImpl,
	SilenceUsage: true,
}

func listContainerAppRoleAssignmentsImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure container app role assignments...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		stream := listContainerAppRoleAssignments(ctx, azClient, listContainerApps(ctx, azClient, subscriptions))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listContainerAppRoleAssignments(ctx context.Context, client client.AzureClient, containerApps <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)

		for result := range pipeline.OrDone(ctx.Done(), containerApps) {
			if containerApp, ok := result.(AzureWrapper).Data.(models.ContainerApp); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating container app role assignments", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, containerApp.Id); !ok {
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
					containerAppRoleAssignments = models.AzureRoleAssignments{
						ObjectId: id,
					}
					count = 0
				)
				for item := range client.ListRoleAssignmentsForResource(ctx, id, "", "") {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing role assignments for this container app", "containerAppId", id)
					} else {
						roleDefinitionId := path.Base(item.Ok.Properties.RoleDefinitionId)

						containerAppRoleAssignment := models.AzureRoleAssignment{
							Assignee:         item.Ok,
							ObjectId:         id,
							RoleDefinitionId: roleDefinitionId,
						}
						log.V(2).Info("found container app role assignment", "containerAppRoleAssignment", containerAppRoleAssignment)
						count++
						containerAppRoleAssignments.RoleAssignments = append(containerAppRoleAssignments.RoleAssignments, containerAppRoleAssignment)
					}
				}
				if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
					Kind: enums.KindAZContainerAppRoleAssignment,
					Data: containerAppRoleAssignments,
				}); !ok {
					return
				}
				log.V(1).Info("finished listing container app role assignments", "containerAppId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all container app role assignments")
	}()

	return out
}
