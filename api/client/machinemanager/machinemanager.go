// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machinemanager

import (
	"time"

	"github.com/juju/errors"
	"github.com/juju/names/v4"

	"github.com/juju/juju/api/base"
	apiwatcher "github.com/juju/juju/api/watcher"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	coreseries "github.com/juju/juju/core/series"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/rpc/params"
)

const machineManagerFacade = "MachineManager"

// Client provides access to the machinemanager, used to add machines to state.
type Client struct {
	base.ClientFacade
	facade base.FacadeCaller
}

// ConstructClient is a constructor function for a machine manager client
func ConstructClient(clientFacade base.ClientFacade, facadeCaller base.FacadeCaller) *Client {
	return &Client{ClientFacade: clientFacade, facade: facadeCaller}
}

// NewClient returns a new machinemanager client.
func NewClient(st base.APICallCloser) *Client {
	frontend, backend := base.NewClientFacade(st, machineManagerFacade)
	return ConstructClient(frontend, backend)
}

// ModelUUID returns the model UUID from the client connection.
func (c *Client) ModelUUID() (string, bool) {
	tag, ok := c.facade.RawAPICaller().ModelTag()
	return tag.Id(), ok
}

// AddMachines adds new machines with the supplied parameters, creating any requested disks.
func (client *Client) AddMachines(machineParams []params.AddMachineParams) ([]params.AddMachinesResult, error) {
	if client.BestAPIVersion() == 8 {
		for i, m := range machineParams {
			if m.Base == nil || m.Base.Name != "centos" {
				continue
			}
			m.Base.Channel = coreseries.ToLegacyCentosChannel(m.Base.Channel)
			machineParams[i] = m
		}
	} else if client.BestAPIVersion() >= 9 {
		for i, m := range machineParams {
			if m.Base == nil || m.Base.Name != "centos" {
				continue
			}
			m.Base.Channel = coreseries.FromLegacyCentosChannel(m.Base.Channel)
			machineParams[i] = m
		}
	}
	args := params.AddMachines{
		MachineParams: machineParams,
	}
	results := new(params.AddMachinesResults)

	err := client.facade.FacadeCall("AddMachines", args, results)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(results.Machines) != len(machineParams) {
		return nil, errors.Errorf("expected %d result, got %d", len(machineParams), len(results.Machines))
	}

	return results.Machines, nil
}

// DestroyMachinesWithParams removes the given set of machines, the semantics of which
// is determined by the force and keep parameters.
// TODO(wallyworld) - for Juju 3.0, this should be the preferred api to use.
func (client *Client) DestroyMachinesWithParams(force, keep bool, maxWait *time.Duration, machines ...string) ([]params.DestroyMachineResult, error) {
	args := params.DestroyMachinesParams{
		Force:       force,
		Keep:        keep,
		MachineTags: make([]string, 0, len(machines)),
	}
	if client.BestAPIVersion() > 5 {
		args.MaxWait = maxWait
	}
	allResults := make([]params.DestroyMachineResult, len(machines))
	index := make([]int, 0, len(machines))
	for i, machineId := range machines {
		if !names.IsValidMachine(machineId) {
			allResults[i].Error = &params.Error{
				Message: errors.NotValidf("machine ID %q", machineId).Error(),
			}
			continue
		}
		index = append(index, i)
		args.MachineTags = append(args.MachineTags, names.NewMachineTag(machineId).String())
	}
	if len(args.MachineTags) > 0 {
		var result params.DestroyMachineResults
		if err := client.facade.FacadeCall("DestroyMachineWithParams", args, &result); err != nil {
			return nil, errors.Trace(err)
		}
		if n := len(result.Results); n != len(args.MachineTags) {
			return nil, errors.Errorf("expected %d result(s), got %d", len(args.MachineTags), n)
		}
		for i, result := range result.Results {
			allResults[index[i]] = result
		}
	}
	return allResults, nil
}

func (client *Client) destroyMachines(method string, machines []string) ([]params.DestroyMachineResult, error) {
	args := params.Entities{
		Entities: make([]params.Entity, 0, len(machines)),
	}
	allResults := make([]params.DestroyMachineResult, len(machines))
	index := make([]int, 0, len(machines))
	for i, machineId := range machines {
		if !names.IsValidMachine(machineId) {
			allResults[i].Error = &params.Error{
				Message: errors.NotValidf("machine ID %q", machineId).Error(),
			}
			continue
		}
		index = append(index, i)
		args.Entities = append(args.Entities, params.Entity{
			Tag: names.NewMachineTag(machineId).String(),
		})
	}
	if len(args.Entities) > 0 {
		var result params.DestroyMachineResults
		if err := client.facade.FacadeCall(method, args, &result); err != nil {
			return nil, errors.Trace(err)
		}
		if n := len(result.Results); n != len(args.Entities) {
			return nil, errors.Errorf("expected %d result(s), got %d", len(args.Entities), n)
		}
		for i, result := range result.Results {
			allResults[index[i]] = result
		}
	}
	return allResults, nil
}

// ProvisioningScript returns a shell script that, when run,
// provisions a machine agent on the machine executing the script.
func (c *Client) ProvisioningScript(args params.ProvisioningScriptParams) (script string, err error) {
	var result params.ProvisioningScriptResult
	if err = c.facade.FacadeCall("ProvisioningScript", args, &result); err != nil {
		return "", err
	}
	return result.Script, nil
}

// RetryProvisioning updates the provisioning status of a machine allowing the
// provisioner to retry.
func (c *Client) RetryProvisioning(all bool, machines ...names.MachineTag) ([]params.ErrorResult, error) {
	p := params.RetryProvisioningArgs{
		All: all,
	}
	p.Machines = make([]string, len(machines))
	for i, machine := range machines {
		p.Machines[i] = machine.String()
	}
	var results params.ErrorResults
	err := c.facade.FacadeCall("RetryProvisioning", p, &results)
	return results.Results, err
}

// UpgradeSeriesPrepare notifies the controller that a series upgrade is taking
// place for a given machine and as such the machine is guarded against
// operations that would impede, fail, or interfere with the upgrade process.
func (client *Client) UpgradeSeriesPrepare(machineName, series string, force bool) error {
	if client.BestAPIVersion() < 5 {
		return errors.NotSupportedf("upgrade-series prepare")
	}
	base, err := coreseries.GetBaseFromSeries(series)
	if err != nil {
		return errors.Trace(err)
	}
	args := params.UpdateChannelArg{
		Entity: params.Entity{
			Tag: names.NewMachineTag(machineName).String(),
		},
		Series:  series,
		Channel: base.Channel.String(),
		Force:   force,
	}
	var result params.ErrorResult
	if err := client.facade.FacadeCall("UpgradeSeriesPrepare", args, &result); err != nil {
		return errors.Trace(err)
	}

	if err := result.Error; err != nil {
		return apiservererrors.RestoreError(err)
	}
	return nil
}

// UpgradeSeriesComplete notifies the controller that a given machine has
// successfully completed the managed series upgrade process.
func (client *Client) UpgradeSeriesComplete(machineName string) error {
	if client.BestAPIVersion() < 5 {
		return errors.NotSupportedf("UpgradeSeriesComplete")
	}
	args := params.UpdateChannelArg{
		Entity: params.Entity{Tag: names.NewMachineTag(machineName).String()},
	}
	result := new(params.ErrorResult)
	err := client.facade.FacadeCall("UpgradeSeriesComplete", args, result)
	if err != nil {
		return errors.Trace(err)
	}
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (client *Client) UpgradeSeriesValidate(machineName, series string) ([]string, error) {
	if client.BestAPIVersion() < 5 {
		return nil, errors.NotSupportedf("UpgradeSeriesValidate")
	}
	base, err := coreseries.GetBaseFromSeries(series)
	if err != nil {
		return nil, errors.Trace(err)
	}
	args := params.UpdateChannelArgs{
		Args: []params.UpdateChannelArg{
			{
				Entity:  params.Entity{Tag: names.NewMachineTag(machineName).String()},
				Series:  series,
				Channel: base.Channel.String(),
			},
		},
	}
	results := new(params.UpgradeSeriesUnitsResults)
	err = client.facade.FacadeCall("UpgradeSeriesValidate", args, results)
	if err != nil {
		return nil, err
	}
	if n := len(results.Results); n != 1 {
		return nil, errors.Errorf("expected 1 result, got %d", n)
	}
	if results.Results[0].Error != nil {
		return nil, results.Results[0].Error
	}
	return results.Results[0].UnitNames, nil
}

// WatchUpgradeSeriesNotifications returns a NotifyWatcher for observing the state of
// a series upgrade.
func (client *Client) WatchUpgradeSeriesNotifications(machineName string) (watcher.NotifyWatcher, string, error) {
	if client.BestAPIVersion() < 5 {
		return nil, "", errors.NotSupportedf("WatchUpgradeSeriesNotifications")
	}
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: names.NewMachineTag(machineName).String()}},
	}
	err := client.facade.FacadeCall("WatchUpgradeSeriesNotifications", args, &results)
	if err != nil {
		return nil, "", errors.Trace(err)
	}
	if len(results.Results) != 1 {
		return nil, "", errors.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, "", result.Error
	}
	w := apiwatcher.NewNotifyWatcher(client.facade.RawAPICaller(), result)
	return w, result.NotifyWatcherId, nil
}

// GetUpgradeSeriesMessages returns a StringsWatcher for observing the state of
// a series upgrade.
func (client *Client) GetUpgradeSeriesMessages(machineName, watcherId string) ([]string, error) {
	if client.BestAPIVersion() < 5 {
		return nil, errors.NotSupportedf("GetUpgradeSeriesMessages")
	}
	var results params.StringsResults
	args := params.UpgradeSeriesNotificationParams{
		Params: []params.UpgradeSeriesNotificationParam{
			{
				Entity:    params.Entity{Tag: names.NewMachineTag(machineName).String()},
				WatcherId: watcherId,
			},
		},
	}
	err := client.facade.FacadeCall("GetUpgradeSeriesMessages", args, &results)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(results.Results) != 1 {
		return nil, errors.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}

	return result.Result, nil
}
