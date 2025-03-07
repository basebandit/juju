// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provisioner

import (
	"sort"

	"github.com/juju/names/v4"
	"github.com/juju/version/v2"

	apiprovisioner "github.com/juju/juju/api/agent/provisioner"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/config"
)

func SetObserver(p Provisioner, observer chan<- *config.Config) {
	var configObserver *configObserver
	if ep, ok := p.(*environProvisioner); ok {
		configObserver = &ep.configObserver
		configObserver.catacomb = &ep.catacomb
	} else {
		cp := p.(*containerProvisioner)
		configObserver = &cp.configObserver
		configObserver.catacomb = &cp.catacomb
	}
	configObserver.Lock()
	configObserver.observer = observer
	configObserver.Unlock()
}

func GetRetryWatcher(p Provisioner) (watcher.NotifyWatcher, error) {
	return p.getRetryWatcher()
}

var (
	GetContainerInitialiser = &getContainerInitialiser
	GetToolsFinder          = &getToolsFinder
	RetryStrategyDelay      = &retryStrategyDelay
	RetryStrategyCount      = &retryStrategyCount
)

var ClassifyMachine = classifyMachine

// GetCopyAvailabilityZoneMachines returns a copy of p.(*provisionerTask).availabilityZoneMachines
func GetCopyAvailabilityZoneMachines(p ProvisionerTask) []AvailabilityZoneMachine {
	task := p.(*provisionerTask)
	task.machinesMutex.RLock()
	defer task.machinesMutex.RUnlock()
	// Sort to make comparisons in the tests easier.
	zoneMachines := task.availabilityZoneMachines
	sort.Slice(task.availabilityZoneMachines, func(i, j int) bool {
		switch {
		case zoneMachines[i].MachineIds.Size() < zoneMachines[j].MachineIds.Size():
			return true
		case zoneMachines[i].MachineIds.Size() == zoneMachines[j].MachineIds.Size():
			return zoneMachines[i].ZoneName < zoneMachines[j].ZoneName
		}
		return false
	})
	retvalues := make([]AvailabilityZoneMachine, len(zoneMachines))
	for i := range zoneMachines {
		retvalues[i] = *zoneMachines[i]
	}
	return retvalues
}

func SetupToStartMachine(p ProvisionerTask, machine apiprovisioner.MachineProvisioner, version *version.Number) (
	environs.StartInstanceParams,
	error,
) {
	return p.(*provisionerTask).setupToStartMachine(machine, version)
}

func MachineSupportsContainers(cfg ContainerManifoldConfig, pr ContainerMachineGetter, mTag names.MachineTag) (ContainerMachine, error) {
	return cfg.machineSupportsContainers(pr, mTag)
}
