package gc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/alerting"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/k8s/namespace"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/k8s/netpol"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/k8s/obc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/k8s/pvc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/k8s/ssh"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubeovn/eip"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubeovn/loadbalancer"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubeovn/subnet"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubeovn/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubevirt/datavolume"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubevirt/vm"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubevirt/vmSnapshot"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/kubevirt/volumeSnapshot"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	gcLog "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func InitGarbageCollection(ctx context.Context) {
	log.Info().Ctx(ctx).Msgf(
		"Initialize Garbage collection - running every : %s and timeout after %s",
		config.Global.GarbageCollection.Interval,
		config.Global.GarbageCollection.Timeout)

	informers.InitResourceInformers(ctx)

	for range time.Tick(config.Global.GarbageCollection.Interval) {
		go CallWithTimeout(ctx, garbageCollection)
	}
}

func CallWithTimeout(ctx context.Context, call func(context.Context, chan<- bool)) {
	// Create a context with a timeout
	collectionCtx, cancel := context.WithTimeout(ctx, config.Global.GarbageCollection.Timeout)
	defer cancel()

	processId := uuid.New().String()
	collectionCtx = context.WithValue(collectionCtx, gcLog.ProcessIdKey, fmt.Sprintf("collection-%s", processId))

	processDone := make(chan bool, 1)

	// Wait for the context to be canceled or timeout
	go call(collectionCtx, processDone)
	l := gcLog.GetProcessLogger(collectionCtx)
	select {
	case <-processDone:
		l.Info().Ctx(collectionCtx).Msgf("Operation completed successfully.")
		return
	case <-collectionCtx.Done():
		l.Error().Ctx(collectionCtx).Msgf("Operation canceled or timed out.")
		alerting.RaiseAlert(collectionCtx, alerting.ProcessCleaning, alerting.ErrorTimeout, "", nil)
		return
	}
}

func garbageCollection(ctx context.Context, processDone chan<- bool) {
	logger := gcLog.GetProcessLogger(ctx)

	logger.Info().Msgf("Garbage collection started at : %s", time.Now().String())

	// Deletion level 1 - Concurrent
	var wgLvl1 sync.WaitGroup

	cleanersLvl1 := []utils.Cleaner{
		&vm.Cleaner{ResourceType: "VirtualMachine", Logger: logger},
		&vmSnapshot.Cleaner{ResourceType: "VirtualMachineSnapshot", Logger: logger},
		&eip.Cleaner{ResourceType: "EIP (FIP + SNAT + DNAT)", Logger: logger},
		&ssh.Cleaner{ResourceType: "SSH Key", Logger: logger},
		&loadbalancer.Cleaner{ResourceType: "Load Balancer", Logger: logger},
		&netpol.Cleaner{ResourceType: "Network Policy", Logger: logger},
	}

	for _, cleaner := range cleanersLvl1 {
		wgLvl1.Go(func() {
			if err := cleaner.Clean(ctx); err != nil {
				logger.Error().Err(err).Msg(cleaner.ErrorMessage())
				cleaner.RaiseListingFailureAlert(ctx, alerting.ProcessCleaning)
			}
		})
	}

	wgLvl1.Wait()

	// Context probably timeout, just cancel the process
	if ctx.Err() != nil {
		return
	}
	// Wait 1 min to ensure resources have been fully deleted (mostly for kubeovn resources)
	time.Sleep(1 * time.Minute)
	// Deletion level 2 - Concurrent
	var wgLvl2 sync.WaitGroup

	cleanersLvl2 := []utils.Cleaner{
		&subnet.Cleaner{ResourceType: "Subnet (NAD + NatGW)", Logger: logger},
		&datavolume.Cleaner{ResourceType: "DataVolume", Logger: logger},
		&volumeSnapshot.Cleaner{ResourceType: "Volume Snapshot", Logger: logger},
		&obc.Cleaner{ResourceType: "ObjectBucketClaim", Logger: logger},
	}

	for _, cleaner := range cleanersLvl2 {
		wgLvl2.Go(func() {
			if err := cleaner.Clean(ctx); err != nil {
				logger.Error().Err(err).Msg(cleaner.ErrorMessage())
				cleaner.RaiseListingFailureAlert(ctx, alerting.ProcessCleaning)
			}
		})
	}

	wgLvl2.Wait()

	// Context probably timeout, just cancel the process
	if ctx.Err() != nil {
		return
	}
	// Wait 1 min to ensure resources have been fully deleted (mostly for kubeovn resources)
	time.Sleep(1 * time.Minute)
	// Deletion level 3 - Sequential

	cleanersLvl3 := []utils.Cleaner{
		&pvc.Cleaner{ResourceType: "PVC", Logger: logger},
		&vpc.Cleaner{ResourceType: "VPC", Logger: logger},
		&namespace.Cleaner{ResourceType: "Namespace", Logger: logger},
	}

	for _, cleaner := range cleanersLvl3 {
		if err := cleaner.Clean(ctx); err != nil {
			logger.Error().Err(err).Msg(cleaner.ErrorMessage())
			cleaner.RaiseListingFailureAlert(ctx, alerting.ProcessCleaning)
		}
	}

	logger.Info().Msgf("Garbage collection finished at : %s", time.Now().String())
	processDone <- true
}
