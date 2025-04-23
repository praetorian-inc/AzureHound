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
	listRootCmd.AddCommand(listSpringAppServiceRoleAssignment)
}

var listSpringAppServiceRoleAssignment = &cobra.Command{
	Use:          "spring-app-service-role-assignments",
	Long:         "Lists Azure Spring App Service Role Assignments",
	Run:          listSpringAppServiceRoleAssignmentImpl,
	SilenceUsage: true,
}

func listSpringAppServiceRoleAssignmentImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure spring app service role assignments...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		stream := listSpringAppServiceRoleAssignments(ctx, azClient, listSpringAppServices(ctx, azClient, subscriptions))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listSpringAppServiceRoleAssignments(ctx context.Context, client client.AzureClient, springAppServices <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)

		for result := range pipeline.OrDone(ctx.Done(), springAppServices) {
			if springAppService, ok := result.(AzureWrapper).Data.(models.SpringAppService); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating spring app service role assignments", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, springAppService.Id); !ok {
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
					springAppServiceRoleAssignments = models.AzureRoleAssignments{
						ObjectId: id,
					}
					count = 0
				)
				for item := range client.ListRoleAssignmentsForResource(ctx, id, "", "") {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing role assignments for this spring app service", "springAppServiceId", id)
					} else {
						roleDefinitionId := path.Base(item.Ok.Properties.RoleDefinitionId)

						springAppServiceRoleAssignment := models.AzureRoleAssignment{
							Assignee:         item.Ok,
							ObjectId:         id,
							RoleDefinitionId: roleDefinitionId,
						}
						log.V(2).Info("found spring app role assignment", "SpringAppServiceAssignment", springAppServiceRoleAssignment)
						count++
						springAppServiceRoleAssignments.RoleAssignments = append(springAppServiceRoleAssignments.RoleAssignments, springAppServiceRoleAssignment)
					}
				}
				if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
					Kind: enums.KindAZSpringAppServiceRoleAssignment,
					Data: springAppServiceRoleAssignments,
				}); !ok {
					return
				}
				log.V(1).Info("finished listing spring app service role assignments", "springAppServiceId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all spring app service role assignments")
	}()

	return out
}
