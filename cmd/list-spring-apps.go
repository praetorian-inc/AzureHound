package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	listRootCmd.AddCommand(listSpringAppsCmd)
}

var listSpringAppsCmd = &cobra.Command{
	Use:          "spring-apps",
	Long:         "Lists Azure Spring Apps",
	Run:          listSpringAppsCmdImpl,
	SilenceUsage: true,
}

func listSpringAppsCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure spring apps...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		springAppServices := listSpringAppServices(ctx, azClient, subscriptions)
		stream := listSpringApps(ctx, azClient, springAppServices)
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listSpringApps(ctx context.Context, client client.AzureClient, springAppServices <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan interface{})
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)
		for result := range pipeline.OrDone(ctx.Done(), springAppServices) {
			if springAppService, ok := result.(AzureWrapper).Data.(models.SpringAppService); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating spring apps", "result", result)
				return
			} else {
				if ok := pipeline.SendAny(ctx.Done(), ids, springAppService); !ok {
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
			for springAppSvc := range stream {
				count := 0
				fixedSubscriptionId := strings.TrimPrefix(springAppSvc.(models.SpringAppService).SubscriptionId, "/subscriptions/")
				for item := range client.ListAzureSpringApps(ctx, fixedSubscriptionId, springAppSvc.(models.SpringAppService).ResourceGroupName, springAppSvc.(models.SpringAppService).Name) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing spring apps for this subscription", "subscriptionId", springAppSvc.(models.SpringAppService).SubscriptionId, "springAppServiceName", springAppSvc.(models.SpringAppService).Name)
					} else {
						springApp := models.SpringApp{
							SpringApp:          item.Ok,
							SpringAppServiceId: springAppSvc.(models.SpringAppService).SpringAppService.Id,
							SubscriptionId:     springAppSvc.(models.SpringAppService).SubscriptionId,
							ResourceGroupName:  springAppSvc.(models.SpringAppService).ResourceGroupName,
							ResourceGroupId:    item.Ok.ResourceGroupId(),
							TenantId:           client.TenantInfo().TenantId,
						}
						log.V(2).Info("found spring app", "springApp", springApp)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZSpringApp,
							Data: springApp,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing spring apps", "subscriptionId", springAppSvc.(models.SpringAppService).SubscriptionId, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all spring apps")
	}()

	return out
}
