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
	listRootCmd.AddCommand(listRedHatOpenShiftClusterRoleAssignmentCmd)
}

var listRedHatOpenShiftClusterRoleAssignmentCmd = &cobra.Command{
	Use:          "red-hat-openshift-cluster-role-assignments",
	Long:         "Lists Red Hat OpenShift Cluster Role Assignments",
	Run:          listRedHatOpenShiftClusterRoleAssignmentImpl,
	SilenceUsage: true,
}

func listRedHatOpenShiftClusterRoleAssignmentImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure red hat openshift cluster role assignments...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		stream := listRedHatOpenShiftClusterRoleAssignments(ctx, azClient, listRedHatOpenShiftClusters(ctx, azClient, subscriptions))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listRedHatOpenShiftClusterRoleAssignments(ctx context.Context, client client.AzureClient, openshiftClusters <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)

		for result := range pipeline.OrDone(ctx.Done(), openshiftClusters) {
			if openshiftCluster, ok := result.(AzureWrapper).Data.(models.RedHatOpenShiftCluster); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating red hat openshift cluster role assignments", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, openshiftCluster.Id); !ok {
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
					redHatOpenShiftClusterRoleAssignments = models.AzureRoleAssignments{
						ObjectId: id,
					}
					count = 0
				)
				for item := range client.ListRoleAssignmentsForResource(ctx, id, "", "") {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing role assignments for this red hat openshift cluster", "redHatOpenShiftClusterId", id)
					} else {
						roleDefinitionId := path.Base(item.Ok.Properties.RoleDefinitionId)

						redHatOpenShiftClusterRoleAssignment := models.AzureRoleAssignment{
							Assignee:         item.Ok,
							ObjectId:         id,
							RoleDefinitionId: roleDefinitionId,
						}
						log.V(2).Info("found red hat openshift cluster role assignment", "redHatOpenShiftClusterRoleAssignment", redHatOpenShiftClusterRoleAssignment)
						count++
						redHatOpenShiftClusterRoleAssignments.RoleAssignments = append(redHatOpenShiftClusterRoleAssignments.RoleAssignments, redHatOpenShiftClusterRoleAssignment)
					}
				}
				if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
					Kind: enums.KindAZRedHatOpenShiftClusterRoleAssignment,
					Data: redHatOpenShiftClusterRoleAssignments,
				}); !ok {
					return
				}
				log.V(1).Info("finished listing red hat openshift cluster role assignments", "redHatOpenShiftClusterId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all red hat openshift cluster role assignments")
	}()

	return out
}
