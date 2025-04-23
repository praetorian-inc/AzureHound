package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	listRootCmd.AddCommand(listServiceFabricClusterAppsCmd)
}

var listServiceFabricClusterAppsCmd = &cobra.Command{
	Use:          "service-fabric-cluster-apps",
	Long:         "Lists Azure Service Fabric Cluster Apps",
	Run:          listServiceFabricClusterAppsCmdImpl,
	SilenceUsage: true,
}

func listServiceFabricClusterAppsCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure service fabric cluster apps...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		serviceFabricClusters := listServiceFabricClusters(ctx, azClient, subscriptions)
		stream := listServiceFabricClusterApps(ctx, azClient, serviceFabricClusters)
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listServiceFabricClusterApps(ctx context.Context, client client.AzureClient, serviceFabricClusters <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan interface{})
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)
		for result := range pipeline.OrDone(ctx.Done(), serviceFabricClusters) {
			if serviceFabricCluster, ok := result.(AzureWrapper).Data.(models.ServiceFabricCluster); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating service fabric cluster apps", "result", result)
				return
			} else {
				if ok := pipeline.SendAny(ctx.Done(), ids, serviceFabricCluster); !ok {
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
			for svcFabricCluster := range stream {
				count := 0
				fixedSubscriptionId := strings.TrimPrefix(svcFabricCluster.(models.ServiceFabricCluster).SubscriptionId, "/subscriptions/")
				for item := range client.ListAzureServiceFabricClusterApps(ctx, fixedSubscriptionId, svcFabricCluster.(models.ServiceFabricCluster).ResourceGroupName, svcFabricCluster.(models.ServiceFabricCluster).Name) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing service fabric cluster apps for this subscription", "subscriptionId", svcFabricCluster.(models.ServiceFabricCluster).SubscriptionId, "serviceFabricClusterAppServiceName", svcFabricCluster.(models.ServiceFabricCluster).Name)
					} else {
						serviceFabricClusterApp := models.ServiceFabricClusterApp{
							ServiceFabricClusterApp: item.Ok,
							ServiceFabricClusterId:  svcFabricCluster.(models.ServiceFabricCluster).ServiceFabricCluster.Id,
							SubscriptionId:          svcFabricCluster.(models.ServiceFabricCluster).SubscriptionId,
							ResourceGroupName:       svcFabricCluster.(models.ServiceFabricCluster).ResourceGroupName,
							ResourceGroupId:         item.Ok.ResourceGroupId(),
							TenantId:                client.TenantInfo().TenantId,
						}
						log.V(2).Info("found service fabric cluster app", "serviceFabricClusterApp", serviceFabricClusterApp)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZServiceFabricClusterApp,
							Data: serviceFabricClusterApp,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing service fabric clsuter apps", "subscriptionId", svcFabricCluster.(models.ServiceFabricCluster).SubscriptionId, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all service fabric cluster apps")
	}()

	return out
}
