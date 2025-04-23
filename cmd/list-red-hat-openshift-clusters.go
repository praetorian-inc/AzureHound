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
	listRootCmd.AddCommand(listRedHatOpenShiftClustersCmd)
}

var listRedHatOpenShiftClustersCmd = &cobra.Command{
	Use:          "red-hat-openshift-clusters",
	Long:         "Lists Azure Red Hat OpenShift Clusters",
	Run:          listRedHatOpenShiftClustersCmdImpl,
	SilenceUsage: true,
}

func listRedHatOpenShiftClustersCmdImpl(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, os.Kill)
	defer gracefulShutdown(stop)

	log.V(1).Info("testing connections")
	if err := testConnections(); err != nil {
		exit(err)
	} else if azClient, err := newAzureClient(); err != nil {
		exit(err)
	} else {
		log.Info("collecting azure red hat openshift clusters...")
		start := time.Now()
		stream := listRedHatOpenShiftClusters(ctx, azClient, listSubscriptions(ctx, azClient))
		panicrecovery.HandleBubbledPanic(ctx, stop, log)
		outputStream(ctx, stream)
		duration := time.Since(start)
		log.Info("collection completed", "duration", duration.String())
	}
}

func listRedHatOpenShiftClusters(ctx context.Context, client client.AzureClient, subscriptions <-chan interface{}) <-chan interface{} {
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
				log.Error(fmt.Errorf("failed type assertion"), "unable to continue enumerating red hat openshift clusters", "result", result)
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
				for item := range client.ListAzureRedHatOpenShiftClusters(ctx, id) {
					if item.Error != nil {
						log.Error(item.Error, "unable to continue processing red hat openshift clusters for this subscription", "subscriptionId", id)
					} else {
						redHatOpenShiftCluster := models.RedHatOpenShiftCluster{
							RedHatOpenShiftCluster: item.Ok,
							SubscriptionId:         "/subscriptions/" + id,
							ResourceGroupId:        item.Ok.ResourceGroupId(),
							TenantId:               client.TenantInfo().TenantId,
						}
						redHatOpenShiftCluster.PopulateIdentity()
						log.V(2).Info("found red hat openshift cluster", "redHatOpenShiftCluster", redHatOpenShiftCluster)
						count++
						if ok := pipeline.SendAny(ctx.Done(), out, AzureWrapper{
							Kind: enums.KindAZRedHatOpenShiftCluster,
							Data: redHatOpenShiftCluster,
						}); !ok {
							return
						}
					}
				}
				log.V(1).Info("finished listing red hat openshift clusters", "subscriptionId", id, "count", count)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
		log.Info("finished listing all red hat openshift clusters")
	}()

	return out
}
