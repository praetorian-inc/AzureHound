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
	listRootCmd.AddCommand(listServiceFabricClustersCmd)
}

var listServiceFabricClustersCmd = &cobra.Command{
	Use:          "service-fabric-clusters",
	Long:         "Lists Azure Service Fabric Clusters",
	Run:          listServiceFabricClustersCmdImpl,
	SilenceUsage: true,
}

func listServiceFabricClustersCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure service fabric clusters...")
		start := time.Now()
		stream := listServiceFabricClusters(ctx, azClient, listSubscriptions(ctx, azClient))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listServiceFabricClusters(ctx context.Context, client client.AzureClient, subscriptions <-chan interface{}) <-chan interface{} {
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
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating service fabric clusters", "result", result)
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
				for item := range client.ListAzureServiceFabricClusters(ctx, id) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing service fabric clusters for this subscription", "subscriptionId", id)
					} else {
						serviceFabricCluster := models.ServiceFabricCluster{
							ServiceFabricCluster: item.Ok,
							SubscriptionId:       "/subscriptions/" + id,
							ResourceGroupId:      item.Ok.ResourceGroupId(),
							ResourceGroupName:    item.Ok.ResourceGroupName(),
							TenantId:             client.TenantInfo().TenantId,
						}
						log.V(2).Info("found service fabric cluster", "serviceFabricCluster", serviceFabricCluster)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZServiceFabricCluster,
							Data: serviceFabricCluster,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing service fabric clusters", "subscriptionId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all service fabric clusters")
	}()

	return out
}
