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
	listRootCmd.AddCommand(listServiceFabricManagedClusterAppsCmd)
}

var listServiceFabricManagedClusterAppsCmd = &cobra.Command{
	Use:          "service-fabric-managed-cluster-apps",
	Long:         "Lists Azure Service Fabric Managed Cluster Apps",
	Run:          listServiceFabricManagedClusterAppsCmdImpl,
	SilenceUsage: true,
}

func listServiceFabricManagedClusterAppsCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure service fabric managed cluster apps...")
		start := time.Now()
		subscriptions := listSubscriptions(ctx, azClient)
		serviceFabricManagedClusters := listServiceFabricManagedClusters(ctx, azClient, subscriptions)
		stream := listServiceFabricManagedClusterApps(ctx, azClient, serviceFabricManagedClusters)
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listServiceFabricManagedClusterApps(ctx context.Context, client client.AzureClient, serviceFabricManagedClusters <-chan interface{}) <-chan interface{} {
	var (
		out     = make(chan interface{})
		ids     = make(chan interface{})
		streams = pipeline.Demux(ctx.Done(), ids, config.ColStreamCount.Value().(int))
		wg      sync.WaitGroup
	)

	go func() {
		defer panicrecovery.PanicRecovery()
		defer close(ids)
		for result := range pipeline.OrDone(ctx.Done(), serviceFabricManagedClusters) {
			if serviceFabricManagedCluster, ok := result.(AzureWrapper).Data.(models.ServiceFabricManagedCluster); !ok {
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating service fabric managed cluster apps", "result", result)
				return
			} else {
				if ok := pipeline.SendAny(ctx.Done(), ids, serviceFabricManagedCluster); !ok {
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
			for svcFabricManagedCluster := range stream {
				count := 0
				fixedSubscriptionId := strings.TrimPrefix(svcFabricManagedCluster.(models.ServiceFabricManagedCluster).SubscriptionId, "/subscriptions/")
				for item := range client.ListAzureServiceFabricManagedClusterApps(ctx, fixedSubscriptionId, svcFabricManagedCluster.(models.ServiceFabricManagedCluster).ResourceGroupName, svcFabricManagedCluster.(models.ServiceFabricManagedCluster).Name) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing service fabric managed cluster apps for this subscription", "subscriptionId", svcFabricManagedCluster.(models.ServiceFabricManagedCluster).SubscriptionId, "serviceFabricClusterAppServiceName", svcFabricManagedCluster.(models.ServiceFabricManagedCluster).Name)
					} else {
						serviceFabricManagedClusterApp := models.ServiceFabricManagedClusterApp{
							ServiceFabricManagedClusterApp: item.Ok,
							ServiceFabricManagedClusterId:  svcFabricManagedCluster.(models.ServiceFabricManagedCluster).ServiceFabricManagedCluster.Id,
							SubscriptionId:                 svcFabricManagedCluster.(models.ServiceFabricManagedCluster).SubscriptionId,
							ResourceGroupName:              svcFabricManagedCluster.(models.ServiceFabricManagedCluster).ResourceGroupName,
							ResourceGroupId:                item.Ok.ResourceGroupId(),
							TenantId:                       client.TenantInfo().TenantId,
						}
						log.V(2).Info("found service fabric cluster app", "serviceManagedApp", serviceFabricManagedClusterApp)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZServiceFabricManagedClusterApp,
							Data: serviceFabricManagedClusterApp,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing service fabric clsuter apps", "subscriptionId", svcFabricManagedCluster.(models.ServiceFabricManagedCluster).SubscriptionId, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all service fabric managed cluster apps")
	}()

	return out
}
