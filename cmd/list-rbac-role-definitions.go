package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
	listRootCmd.AddCommand(listRBACRoleDefinitionsCmd)
}

var listRBACRoleDefinitionsCmd = &cobra.Command{
	Use:          "rbac-role-definitions",
	Long:         "Lists RBAC Role Definitions",
	Run:          listRBACRoleDefinitionsCmdImpl,
	SilenceUsage: true,
}

func listRBACRoleDefinitionsCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure RBAC role definitions...")
		start := time.Now()
		stream := listRBACRoleDefinitions(ctx, azClient, listSubscriptions(ctx, azClient))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listRBACRoleDefinitions(ctx context.Context, client client.AzureClient, subscriptions <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan string)
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)
		for result := range pipeline.OrDone(ctx.Done(), subscriptions) {
			if subscription, ok := result.(AzureWrapper).Data.(models.Subscription); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating RBAC role definitions", "result", result)
				return
			} else {
				if ok := pipeline.Send(ctx.Done(), ids, subscription.SubscriptionId); !ok {
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
				count := 0
				for item := range client.ListAzureRBACRoleDefinitions(ctx, id) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing RBAC role definitions for this subscription", "subscriptionId", id)
					} else {
						rbacRoleDefinition := models.RBACRoleDefinition{
							RBACRoleDefinition:   item.Ok,
							SubscriptionId:       "/subscriptions/" + id,
							RBACRoleDefinitionId: item.Ok.Id,
							TenantId:             client.TenantInfo().TenantId,
						}
						log.V(2).Info("found RBAC role definition", "rbacRoleDefinition", rbacRoleDefinition)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZRBACRoleDefinition,
							Data: rbacRoleDefinition,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing RBAC role definitions", "subscriptionId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all RBAC role definitions")
	}()

	return out
}
