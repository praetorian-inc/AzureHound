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
	listRootCmd.AddCommand(listSpringAppServicesCmd)
}

var listSpringAppServicesCmd = &cobra.Command{
	Use:          "spring-app-services",
	Long:         "Lists Azure Spring Services",
	Run:          listSpringAppServicesCmdImpl,
	SilenceUsage: true,
}

func listSpringAppServicesCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure spring app services...")
		start := time.Now()
		stream := listSpringAppServices(ctx, azClient, listSubscriptions(ctx, azClient))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listSpringAppServices(ctx context.Context, client client.AzureClient, subscriptions <-chan interface{}) <-chan interface{} {
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
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating spring services", "result", result)
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
				for item := range client.ListAzureSpringAppServices(ctx, id) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing spring app services for this subscription", "subscriptionId", id)
					} else {
						springAppService := models.SpringAppService{
							SpringAppService:  item.Ok,
							SubscriptionId:    "/subscriptions/" + id,
							ResourceGroupId:   item.Ok.ResourceGroupId(),
							ResourceGroupName: item.Ok.ResourceGroupName(),
							TenantId:          client.TenantInfo().TenantId,
						}
						log.V(2).Info("found spring app service", "springAppService", springAppService)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZSpringAppService,
							Data: springAppService,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing spring app services", "subscriptionId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all spring app services")
	}()

	return out
}
