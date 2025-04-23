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
	listRootCmd.AddCommand(listContainerGroupRoleAssignmentCmd)
}

var listContainerGroupRoleAssignmentCmd = &cobra.Command{
	Use:          "container-group-role-assignments",
	Long:         "Lists Azure Container Group Service Role Assignments",
	Run:          listContainerGroupRoleAssignmentsImpl,
	SilenceUsage: true,
}

func listContainerGroupRoleAssignmentsImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure container group role assignments...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		stream := listContainerGroupRoleAssignments(ctx, azClient, listContainerGroups(ctx, azClient, subscriptions))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listContainerGroupRoleAssignments(ctx context.Context, client client.AzureClient, containerGroups <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)

		for result := range pipeline.OrDone(ctx.Done(), containerGroups) {
			if containerGroup, ok := result.(AzureWrapper).Data.(models.ContainerGroup); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating container group role assignments", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, containerGroup.Id); !ok {
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
					containerGroupRoleAssignments = models.AzureRoleAssignments{
						ObjectId: id,
					}
					count = 0
				)
				for item := range client.ListRoleAssignmentsForResource(ctx, id, "", "") {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing role assignments for this container group", "containerGroupId", id)
					} else {
						roleDefinitionId := path.Base(item.Ok.Properties.RoleDefinitionId)

						containerGroupRoleAssignment := models.AzureRoleAssignment{
							Assignee:         item.Ok,
							ObjectId:         id,
							RoleDefinitionId: roleDefinitionId,
						}
						log.V(2).Info("found container group role assignment", "containerGroupRoleAssignment", containerGroupRoleAssignment)
						count++
						containerGroupRoleAssignments.RoleAssignments = append(containerGroupRoleAssignments.RoleAssignments, containerGroupRoleAssignment)
					}
				}
				if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
					Kind: enums.KindAZContainerGroupRoleAssignment,
					Data: containerGroupRoleAssignments,
				}); !ok {
					return
				}
				log.V(1).Info("finished listing container group role assignments", "containerGroupId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all container group role assignments")
	}()

	return out
}
