// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelupgrader_test

import (
	"net/http"

	"github.com/golang/mock/gomock"
	"github.com/juju/errors"
	"github.com/juju/names/v4"
	"github.com/juju/replicaset/v2"
	jujutesting "github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v3"
	"github.com/juju/version/v2"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/apiserver/facades/client/modelupgrader"
	"github.com/juju/juju/apiserver/facades/client/modelupgrader/mocks"
	apiservertesting "github.com/juju/juju/apiserver/testing"
	"github.com/juju/juju/controller"
	coreos "github.com/juju/juju/core/os"
	"github.com/juju/juju/docker"
	"github.com/juju/juju/docker/registry"
	"github.com/juju/juju/docker/registry/image"
	registrymocks "github.com/juju/juju/docker/registry/mocks"
	"github.com/juju/juju/environs"
	environscloudspec "github.com/juju/juju/environs/cloudspec"
	"github.com/juju/juju/environs/context"
	envtools "github.com/juju/juju/environs/tools"
	"github.com/juju/juju/provider/lxd"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
	coretesting "github.com/juju/juju/testing"
	coretools "github.com/juju/juju/tools"
	"github.com/juju/juju/upgrades/upgradevalidation"
	upgradevalidationmocks "github.com/juju/juju/upgrades/upgradevalidation/mocks"
)

type modelUpgradeSuite struct {
	jujutesting.IsolationSuite

	adminUser   names.UserTag
	authoriser  apiservertesting.FakeAuthorizer
	callContext context.ProviderCallContext

	statePool        *mocks.MockStatePool
	toolsFinder      *mocks.MockToolsFinder
	bootstrapEnviron *mocks.MockBootstrapEnviron
	blockChecker     *mocks.MockBlockCheckerInterface
	registryProvider *registrymocks.MockRegistry
	cloudSpec        environscloudspec.CloudSpec
}

var _ = gc.Suite(&modelUpgradeSuite{})

func (s *modelUpgradeSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)

	adminUser := "admin"
	s.adminUser = names.NewUserTag(adminUser)

	s.authoriser = apiservertesting.FakeAuthorizer{
		Tag: s.adminUser,
	}

	s.callContext = context.NewEmptyCloudCallContext()
	s.cloudSpec = environscloudspec.CloudSpec{Type: "lxd"}
}

func (s *modelUpgradeSuite) getModelUpgraderAPI(c *gc.C) (*gomock.Controller, *modelupgrader.ModelUpgraderAPI) {
	ctrl := gomock.NewController(c)
	s.statePool = mocks.NewMockStatePool(ctrl)
	s.toolsFinder = mocks.NewMockToolsFinder(ctrl)
	s.bootstrapEnviron = mocks.NewMockBootstrapEnviron(ctrl)
	s.blockChecker = mocks.NewMockBlockCheckerInterface(ctrl)
	s.registryProvider = registrymocks.NewMockRegistry(ctrl)

	api, err := modelupgrader.NewModelUpgraderAPI(
		coretesting.ControllerTag,
		s.statePool,
		s.toolsFinder,
		func() (environs.BootstrapEnviron, error) {
			return s.bootstrapEnviron, nil
		},
		s.blockChecker, s.authoriser, s.callContext,
		func(docker.ImageRepoDetails) (registry.Registry, error) {
			return s.registryProvider, nil
		},
		func(names.ModelTag) (environscloudspec.CloudSpec, error) {
			return s.cloudSpec, nil
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	return ctrl, api
}

func (s *modelUpgradeSuite) TestUpgradeModelWithInvalidModelTag(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	_, err := api.UpgradeModel(params.UpgradeModelParams{ModelTag: "!!!"})
	c.Assert(err, gc.ErrorMatches, `"!!!" is not a valid tag`)
}

func (s *modelUpgradeSuite) TestUpgradeModelWithModelWithNoPermission(c *gc.C) {
	s.authoriser = apiservertesting.FakeAuthorizer{
		Tag: names.NewUserTag("user"),
	}
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	_, err := api.UpgradeModel(
		params.UpgradeModelParams{
			ModelTag:      coretesting.ModelTag.String(),
			TargetVersion: version.MustParse("3.0.0"),
		},
	)
	c.Assert(err, gc.ErrorMatches, `permission denied`)
}

func (s *modelUpgradeSuite) TestUpgradeModelWithChangeNotAllowed(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	s.blockChecker.EXPECT().ChangeAllowed().Return(errors.Errorf("the operation has been blocked"))

	_, err := api.UpgradeModel(
		params.UpgradeModelParams{
			ModelTag:      coretesting.ModelTag.String(),
			TargetVersion: version.MustParse("3.0.0"),
		},
	)
	c.Assert(err, gc.ErrorMatches, `the operation has been blocked`)
}

func (s *modelUpgradeSuite) assertUpgradeModelForControllerModelJuju3(c *gc.C, dryRun bool) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	s.PatchValue(&upgradevalidation.MinMajorUpgradeVersions, map[int]version.Number{
		3: version.MustParse("2.9.1"),
	})

	server := upgradevalidationmocks.NewMockServer(ctrl)
	serverFactory := upgradevalidationmocks.NewMockServerFactory(ctrl)
	s.PatchValue(&upgradevalidation.NewServerFactory,
		func(httpClient *http.Client) lxd.ServerFactory {
			return serverFactory
		},
	)

	ctrlModelTag := coretesting.ModelTag
	model1ModelUUID, err := utils.NewUUID()
	c.Assert(err, jc.ErrorIsNil)
	ctrlModel := mocks.NewMockModel(ctrl)
	model1 := mocks.NewMockModel(ctrl)
	ctrlModel.EXPECT().IsControllerModel().Return(true)

	ctrlState := mocks.NewMockState(ctrl)
	state1 := mocks.NewMockState(ctrl)
	ctrlState.EXPECT().Release().AnyTimes()
	ctrlState.EXPECT().Model().Return(ctrlModel, nil)
	state1.EXPECT().Release()

	s.statePool.EXPECT().Get(ctrlModelTag.Id()).Return(ctrlState, nil)
	var agentStream string
	assertions := []*gomock.Call{
		s.blockChecker.EXPECT().ChangeAllowed().Return(nil),

		// Decide/validate target version.
		ctrlModel.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 3, MinorVersion: -1}).Return(
			params.FindToolsResult{
				List: []*coretools.Tools{
					{Version: version.MustParseBinary("3.9.99-ubuntu-amd64")},
				},
			}, nil,
		),
		ctrlModel.EXPECT().Type().Return(state.ModelTypeIAAS),
		ctrlModel.EXPECT().Name().Return("controller"),

		// 1. Check controller model.
		// - check agent version;
		ctrlModel.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		// - check mongo status;
		ctrlState.EXPECT().MongoCurrentStatus().Return(&replicaset.Status{
			Members: []replicaset.MemberStatus{
				{
					Id:      1,
					Address: "1.1.1.1",
					State:   replicaset.PrimaryState,
				},
				{
					Id:      2,
					Address: "2.2.2.2",
					State:   replicaset.SecondaryState,
				},
				{
					Id:      3,
					Address: "3.3.3.3",
					State:   replicaset.SecondaryState,
				},
			},
		}, nil),
		// - check mongo version;
		s.statePool.EXPECT().MongoVersion().Return("4.4", nil),
		// - check if the model has win machines;
		ctrlState.EXPECT().MachineCountForSeries(
			"win10", "win2008r2", "win2012", "win2012hv", "win2012hvr2", "win2012r2",
			"win2016", "win2016hv", "win2019", "win7", "win8", "win81",
		).Return(nil, nil),
		// - check if the model has deprecated ubuntu machines;
		ctrlState.EXPECT().MachineCountForSeries(
			"artful",
			"bionic",
			"cosmic",
			"disco",
			"eoan",
			"groovy",
			"hirsute",
			"impish",
			"kinetic",
			"precise",
			"quantal",
			"raring",
			"saucy",
			"trusty",
			"utopic",
			"vivid",
			"wily",
			"xenial",
			"yakkety",
			"zesty",
		).Return(nil, nil),
		// - check LXD version.
		serverFactory.EXPECT().RemoteServer(s.cloudSpec).Return(server, nil),
		server.EXPECT().ServerVersion().Return("5.2"),

		ctrlState.EXPECT().AllModelUUIDs().Return([]string{ctrlModelTag.Id(), model1ModelUUID.String()}, nil),

		// 2. Check other models.
		s.statePool.EXPECT().Get(model1ModelUUID.String()).Return(state1, nil),
		state1.EXPECT().Model().Return(model1, nil),
		// - check agent version;
		model1.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		//  - check if model migration is ongoing;
		model1.EXPECT().MigrationMode().Return(state.MigrationModeNone),
		// - check if the model has win machines;
		state1.EXPECT().MachineCountForSeries(
			"win10", "win2008r2", "win2012", "win2012hv", "win2012hvr2", "win2012r2",
			"win2016", "win2016hv", "win2019", "win7", "win8", "win81",
		).Return(nil, nil),
		// - check if the model has deprecated ubuntu machines;
		state1.EXPECT().MachineCountForSeries(
			"artful",
			"bionic",
			"cosmic",
			"disco",
			"eoan",
			"groovy",
			"hirsute",
			"impish",
			"kinetic",
			"precise",
			"quantal",
			"raring",
			"saucy",
			"trusty",
			"utopic",
			"vivid",
			"wily",
			"xenial",
			"yakkety",
			"zesty",
		).Return(nil, nil),
		// - check LXD version.
		serverFactory.EXPECT().RemoteServer(s.cloudSpec).Return(server, nil),
		server.EXPECT().ServerVersion().Return("5.2"),
	}
	if !dryRun {
		assertions = append(assertions,
			// s.statePool.EXPECT().Get(ctrlModelTag.Id()).Return(ctrlState, nil),
			ctrlState.EXPECT().SetModelAgentVersion(version.MustParse("3.9.99"), nil, false).Return(nil),
		)
	}
	gomock.InOrder(assertions...)

	result, err := api.UpgradeModel(
		params.UpgradeModelParams{
			ModelTag:      ctrlModelTag.String(),
			TargetVersion: version.MustParse("3.9.99"),
			AgentStream:   agentStream,
			DryRun:        dryRun,
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, gc.DeepEquals, params.UpgradeModelResult{
		ChosenVersion: version.MustParse("3.9.99"),
	})
}

func (s *modelUpgradeSuite) TestUpgradeModelForControllerModelJuju3(c *gc.C) {
	s.assertUpgradeModelForControllerModelJuju3(c, false)
}

func (s *modelUpgradeSuite) TestUpgradeModelForControllerModelJuju3DryRun(c *gc.C) {
	s.assertUpgradeModelForControllerModelJuju3(c, true)
}

func (s *modelUpgradeSuite) TestUpgradeModelForControllerModelJuju3Failed(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	s.PatchValue(&upgradevalidation.MinMajorUpgradeVersions, map[int]version.Number{
		3: version.MustParse("2.9.2"),
	})

	server := upgradevalidationmocks.NewMockServer(ctrl)
	serverFactory := upgradevalidationmocks.NewMockServerFactory(ctrl)
	s.PatchValue(&upgradevalidation.NewServerFactory,
		func(httpClient *http.Client) lxd.ServerFactory {
			return serverFactory
		},
	)

	ctrlModelTag := coretesting.ModelTag
	model1ModelUUID, err := utils.NewUUID()
	c.Assert(err, jc.ErrorIsNil)
	ctrlModel := mocks.NewMockModel(ctrl)
	model1 := mocks.NewMockModel(ctrl)
	ctrlModel.EXPECT().IsControllerModel().Return(true)

	ctrlState := mocks.NewMockState(ctrl)
	state1 := mocks.NewMockState(ctrl)
	ctrlState.EXPECT().Release().AnyTimes()
	ctrlState.EXPECT().Model().Return(ctrlModel, nil)
	state1.EXPECT().Release()

	s.statePool.EXPECT().Get(ctrlModelTag.Id()).Return(ctrlState, nil)

	gomock.InOrder(
		s.blockChecker.EXPECT().ChangeAllowed().Return(nil),

		// Decide/validate target version.
		ctrlModel.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 3, MinorVersion: -1}).Return(
			params.FindToolsResult{
				List: []*coretools.Tools{
					{Version: version.MustParseBinary("3.9.99-ubuntu-amd64")},
				},
			}, nil,
		),
		ctrlModel.EXPECT().Type().Return(state.ModelTypeIAAS),
		ctrlModel.EXPECT().Name().Return("controller"),

		// 1. Check controller model.
		// - check agent version;
		ctrlModel.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		// - check mongo status;
		ctrlState.EXPECT().MongoCurrentStatus().Return(&replicaset.Status{
			Members: []replicaset.MemberStatus{
				{
					Id:      1,
					Address: "1.1.1.1",
					State:   replicaset.FatalState,
				},
				{
					Id:      2,
					Address: "2.2.2.2",
					State:   replicaset.ArbiterState,
				},
				{
					Id:      3,
					Address: "3.3.3.3",
					State:   replicaset.RecoveringState,
				},
			},
		}, nil),
		// - check mongo version;
		s.statePool.EXPECT().MongoVersion().Return("4.3", nil),
		// - check if the model has win machines;
		ctrlState.EXPECT().MachineCountForSeries(
			"win10", "win2008r2", "win2012", "win2012hv", "win2012hvr2", "win2012r2",
			"win2016", "win2016hv", "win2019", "win7", "win8", "win81",
		).Return(map[string]int{"win10": 1, "win7": 2}, nil),
		// - check if the model has deprecated ubuntu machines;
		ctrlState.EXPECT().MachineCountForSeries(
			"artful",
			"bionic",
			"cosmic",
			"disco",
			"eoan",
			"groovy",
			"hirsute",
			"impish",
			"kinetic",
			"precise",
			"quantal",
			"raring",
			"saucy",
			"trusty",
			"utopic",
			"vivid",
			"wily",
			"xenial",
			"yakkety",
			"zesty",
		).Return(map[string]int{"xenial": 2}, nil),
		// - check LXD version.
		serverFactory.EXPECT().RemoteServer(s.cloudSpec).Return(server, nil),
		server.EXPECT().ServerVersion().Return("4.0"),
		ctrlModel.EXPECT().Owner().Return(names.NewUserTag("admin")),
		ctrlModel.EXPECT().Name().Return("controller"),

		ctrlState.EXPECT().AllModelUUIDs().Return([]string{ctrlModelTag.Id(), model1ModelUUID.String()}, nil),
		// 2. Check other models.
		s.statePool.EXPECT().Get(model1ModelUUID.String()).Return(state1, nil),
		state1.EXPECT().Model().Return(model1, nil),
		// - check agent version;
		model1.EXPECT().AgentVersion().Return(version.MustParse("2.9.0"), nil),
		//  - check if model migration is ongoing;
		model1.EXPECT().MigrationMode().Return(state.MigrationModeExporting),
		// - check if the model has win machines;
		state1.EXPECT().MachineCountForSeries(
			"win10", "win2008r2", "win2012", "win2012hv", "win2012hvr2", "win2012r2",
			"win2016", "win2016hv", "win2019", "win7", "win8", "win81",
		).Return(map[string]int{"win10": 1, "win7": 3}, nil),
		// - check if the model has deprecated ubuntu machines;
		state1.EXPECT().MachineCountForSeries(
			"artful",
			"bionic",
			"cosmic",
			"disco",
			"eoan",
			"groovy",
			"hirsute",
			"impish",
			"kinetic",
			"precise",
			"quantal",
			"raring",
			"saucy",
			"trusty",
			"utopic",
			"vivid",
			"wily",
			"xenial",
			"yakkety",
			"zesty",
		).Return(map[string]int{
			"artful": 1, "cosmic": 2, "disco": 3, "eoan": 4, "groovy": 5,
			"hirsute": 6, "impish": 7, "precise": 8, "quantal": 9, "raring": 10,
			"saucy": 11, "trusty": 12, "utopic": 13, "vivid": 14, "wily": 15,
			"xenial": 16, "yakkety": 17, "zesty": 18,
		}, nil),
		// - check LXD version.
		serverFactory.EXPECT().RemoteServer(s.cloudSpec).Return(server, nil),
		server.EXPECT().ServerVersion().Return("4.0"),
		model1.EXPECT().Owner().Return(names.NewUserTag("admin")),
		model1.EXPECT().Name().Return("model-1"),
	)

	result, err := api.UpgradeModel(
		params.UpgradeModelParams{
			ModelTag:      ctrlModelTag.String(),
			TargetVersion: version.MustParse("3.9.99"),
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result.Error.Error(), gc.Equals, `
cannot upgrade to "3.9.99" due to issues with these models:
"admin/controller":
- current model ("2.9.1") has to be upgraded to "2.9.2" at least
- unable to upgrade, database node 1 (1.1.1.1) has state FATAL, node 2 (2.2.2.2) has state ARBITER, node 3 (3.3.3.3) has state RECOVERING
- mongo version has to be "4.4" at least, but current version is "4.3"
- the model hosts deprecated windows machine(s): win10(1) win7(2)
- the model hosts deprecated ubuntu machine(s): xenial(2)
- LXD version has to be at least "5.0.0", but current version is only "4.0.0"
"admin/model-1":
- current model ("2.9.0") has to be upgraded to "2.9.2" at least
- model is under "exporting" mode, upgrade blocked
- the model hosts deprecated windows machine(s): win10(1) win7(3)
- the model hosts deprecated ubuntu machine(s): artful(1) cosmic(2) disco(3) eoan(4) groovy(5) hirsute(6) impish(7) precise(8) quantal(9) raring(10) saucy(11) trusty(12) utopic(13) vivid(14) wily(15) xenial(16) yakkety(17) zesty(18)
- LXD version has to be at least "5.0.0", but current version is only "4.0.0"`[1:])
}

func (s *modelUpgradeSuite) assertUpgradeModelJuju3(c *gc.C, dryRun bool) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	s.PatchValue(&upgradevalidation.MinMajorUpgradeVersions, map[int]version.Number{
		3: version.MustParse("2.9.1"),
	})

	server := upgradevalidationmocks.NewMockServer(ctrl)
	serverFactory := upgradevalidationmocks.NewMockServerFactory(ctrl)
	s.PatchValue(&upgradevalidation.NewServerFactory,
		func(httpClient *http.Client) lxd.ServerFactory {
			return serverFactory
		},
	)

	modelUUID := coretesting.ModelTag.Id()
	model := mocks.NewMockModel(ctrl)
	st := mocks.NewMockState(ctrl)
	st.EXPECT().Release().AnyTimes()

	s.statePool.EXPECT().Get(modelUUID).AnyTimes().Return(st, nil)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	var agentStream string
	assertions := []*gomock.Call{
		s.blockChecker.EXPECT().ChangeAllowed().Return(nil),

		// Decide/validate target version.
		model.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 3, MinorVersion: -1}).Return(
			params.FindToolsResult{
				List: []*coretools.Tools{
					{Version: version.MustParseBinary("3.9.99-ubuntu-amd64")},
				},
			}, nil,
		),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
		model.EXPECT().Name().Return("model-1"),

		model.EXPECT().IsControllerModel().Return(false),

		// - check no upgrade series in process.
		st.EXPECT().HasUpgradeSeriesLocks().Return(false, nil),

		// - check if the model has win machines;
		st.EXPECT().MachineCountForSeries(
			"win10", "win2008r2", "win2012", "win2012hv", "win2012hvr2", "win2012r2",
			"win2016", "win2016hv", "win2019", "win7", "win8", "win81",
		).Return(nil, nil),
		// - check if the model has deprecated ubuntu machines;
		st.EXPECT().MachineCountForSeries(
			"artful",
			"bionic",
			"cosmic",
			"disco",
			"eoan",
			"groovy",
			"hirsute",
			"impish",
			"kinetic",
			"precise",
			"quantal",
			"raring",
			"saucy",
			"trusty",
			"utopic",
			"vivid",
			"wily",
			"xenial",
			"yakkety",
			"zesty",
		).Return(nil, nil),
		// - check LXD version.
		serverFactory.EXPECT().RemoteServer(s.cloudSpec).Return(server, nil),
		server.EXPECT().ServerVersion().Return("5.2"),
	}
	if !dryRun {
		assertions = append(assertions,
			st.EXPECT().SetModelAgentVersion(version.MustParse("3.9.99"), nil, false).Return(nil),
		)
	}
	gomock.InOrder(assertions...)

	result, err := api.UpgradeModel(
		params.UpgradeModelParams{
			ModelTag:      coretesting.ModelTag.String(),
			TargetVersion: version.MustParse("3.9.99"),
			AgentStream:   agentStream,
			DryRun:        dryRun,
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, gc.DeepEquals, params.UpgradeModelResult{
		ChosenVersion: version.MustParse("3.9.99"),
	})
}

func (s *modelUpgradeSuite) TestUpgradeModelJuju3(c *gc.C) {
	s.assertUpgradeModelJuju3(c, false)
}

func (s *modelUpgradeSuite) TestUpgradeModelJuju3DryRun(c *gc.C) {
	s.assertUpgradeModelJuju3(c, true)
}

func (s *modelUpgradeSuite) TestUpgradeModelJuju3Failed(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	s.PatchValue(&upgradevalidation.MinMajorUpgradeVersions, map[int]version.Number{
		3: version.MustParse("2.9.1"),
	})

	server := upgradevalidationmocks.NewMockServer(ctrl)
	serverFactory := upgradevalidationmocks.NewMockServerFactory(ctrl)
	s.PatchValue(&upgradevalidation.NewServerFactory,
		func(httpClient *http.Client) lxd.ServerFactory {
			return serverFactory
		},
	)

	modelUUID := coretesting.ModelTag.Id()
	model := mocks.NewMockModel(ctrl)
	st := mocks.NewMockState(ctrl)
	st.EXPECT().Release()

	s.statePool.EXPECT().Get(modelUUID).AnyTimes().Return(st, nil)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	gomock.InOrder(
		s.blockChecker.EXPECT().ChangeAllowed().Return(nil),

		// Decide/validate target version.
		model.EXPECT().AgentVersion().Return(version.MustParse("2.9.1"), nil),
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 3, MinorVersion: -1}).Return(
			params.FindToolsResult{
				List: []*coretools.Tools{
					{Version: version.MustParseBinary("3.9.99-ubuntu-amd64")},
				},
			}, nil,
		),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
		model.EXPECT().Name().Return("model-1"),

		model.EXPECT().IsControllerModel().Return(false),

		// - check no upgrade series in process.
		st.EXPECT().HasUpgradeSeriesLocks().Return(true, nil),

		// - check if the model has win machines;
		st.EXPECT().MachineCountForSeries(
			"win10", "win2008r2", "win2012", "win2012hv", "win2012hvr2", "win2012r2",
			"win2016", "win2016hv", "win2019", "win7", "win8", "win81",
		).Return(map[string]int{"win10": 1, "win7": 3}, nil),
		// - check if the model has deprecated ubuntu machines;
		st.EXPECT().MachineCountForSeries(
			"artful",
			"bionic",
			"cosmic",
			"disco",
			"eoan",
			"groovy",
			"hirsute",
			"impish",
			"kinetic",
			"precise",
			"quantal",
			"raring",
			"saucy",
			"trusty",
			"utopic",
			"vivid",
			"wily",
			"xenial",
			"yakkety",
			"zesty",
		).Return(map[string]int{
			"artful": 1, "cosmic": 2, "disco": 3, "eoan": 4, "groovy": 5,
			"hirsute": 6, "impish": 7, "precise": 8, "quantal": 9, "raring": 10,
			"saucy": 11, "trusty": 12, "utopic": 13, "vivid": 14, "wily": 15,
			"xenial": 16, "yakkety": 17, "zesty": 18,
		}, nil),
		// - check LXD version.
		serverFactory.EXPECT().RemoteServer(s.cloudSpec).Return(server, nil),
		server.EXPECT().ServerVersion().Return("4.0"),
		model.EXPECT().Owner().Return(names.NewUserTag("admin")),
		model.EXPECT().Name().Return("model-1"),
	)
	result, err := api.UpgradeModel(
		params.UpgradeModelParams{
			ModelTag:      coretesting.ModelTag.String(),
			TargetVersion: version.MustParse("3.9.99"),
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result.Error.Error(), gc.Equals, `
cannot upgrade to "3.9.99" due to issues with these models:
"admin/model-1":
- unexpected upgrade series lock found
- the model hosts deprecated windows machine(s): win10(1) win7(3)
- the model hosts deprecated ubuntu machine(s): artful(1) cosmic(2) disco(3) eoan(4) groovy(5) hirsute(6) impish(7) precise(8) quantal(9) raring(10) saucy(11) trusty(12) utopic(13) vivid(14) wily(15) xenial(16) yakkety(17) zesty(18)
- LXD version has to be at least "5.0.0", but current version is only "4.0.0"`[1:])
}

func (s *modelUpgradeSuite) TestAbortCurrentUpgrade(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	modelUUID := coretesting.ModelTag.Id()
	st := mocks.NewMockState(ctrl)
	st.EXPECT().Release()

	gomock.InOrder(
		s.blockChecker.EXPECT().ChangeAllowed().Return(nil),
		s.statePool.EXPECT().Get(modelUUID).Return(st, nil),
		st.EXPECT().AbortCurrentUpgrade().Return(nil),
	)
	err := api.AbortModelUpgrade(params.ModelParam{ModelTag: coretesting.ModelTag.String()})
	c.Assert(err, jc.ErrorIsNil)
}

func (s *modelUpgradeSuite) TestFindToolsIAAS(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)

	simpleStreams := params.FindToolsResult{
		List: []*coretools.Tools{
			{Version: version.MustParseBinary("2.9.6-ubuntu-amd64")},
		},
	}

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 2}).
			Return(simpleStreams, nil),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
	)

	result, err := api.FindTools(
		st, model, 2, 0, version.MustParse("2.9.9"), "", "", "",
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, gc.DeepEquals, coretools.Versions{
		&coretools.Tools{Version: version.MustParseBinary("2.9.6-ubuntu-amd64")},
	})
}

func (s *modelUpgradeSuite) TestFindToolsCAASReleased(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	simpleStreams := params.FindToolsResult{
		List: []*coretools.Tools{
			{Version: version.MustParseBinary("2.9.9-ubuntu-amd64")},
			{Version: version.MustParseBinary("2.9.10-ubuntu-amd64")},
			{Version: version.MustParseBinary("2.9.11-ubuntu-amd64")},
		},
	}
	s.PatchValue(&coreos.HostOS, func() coreos.OSType { return coreos.Ubuntu })

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 2}).
			Return(simpleStreams, nil),
		model.EXPECT().Type().Return(state.ModelTypeCAAS),

		st.EXPECT().ControllerConfig().Return(controller.Config{
			controller.ControllerUUIDKey: coretesting.ControllerTag.Id(),
			controller.CAASImageRepo: `
{
    "serveraddress": "quay.io",
    "auth": "xxxxx==",
    "repository": "test-account"
}
`[1:],
		}, nil),
		s.registryProvider.EXPECT().Tags("jujud-operator").Return(coretools.Versions{
			image.NewImageInfo(version.MustParse("2.9.8")),    // skip: older than current version.
			image.NewImageInfo(version.MustParse("2.9.9")),    // skip: older than current version.
			image.NewImageInfo(version.MustParse("2.9.10.1")), // skip: current is stable build.
			image.NewImageInfo(version.MustParse("2.9.10")),
			image.NewImageInfo(version.MustParse("2.9.11")),
			image.NewImageInfo(version.MustParse("2.9.12")), // skip: it's not released in simplestream yet.
		}, nil),
		s.registryProvider.EXPECT().GetArchitecture("jujud-operator", "2.9.10").Return("amd64", nil),
		s.registryProvider.EXPECT().GetArchitecture("jujud-operator", "2.9.11").Return("amd64", nil),
		s.registryProvider.EXPECT().Close().Return(nil),
	)

	result, err := api.FindTools(
		st, model, 2, 0, version.MustParse("2.9.9"), "", "", "",
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, gc.DeepEquals, coretools.Versions{
		&coretools.Tools{Version: version.MustParseBinary("2.9.10-ubuntu-amd64")},
		&coretools.Tools{Version: version.MustParseBinary("2.9.11-ubuntu-amd64")},
	})
}

func (s *modelUpgradeSuite) TestFindToolsCAASNonReleased(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	simpleStreams := params.FindToolsResult{
		List: []*coretools.Tools{
			{Version: version.MustParseBinary("2.9.9-ubuntu-amd64")},
			{Version: version.MustParseBinary("2.9.10-ubuntu-amd64")},
			{Version: version.MustParseBinary("2.9.11-ubuntu-amd64")},
			{Version: version.MustParseBinary("2.9.12-ubuntu-amd64")},
		},
	}
	s.PatchValue(&coreos.HostOS, func() coreos.OSType { return coreos.Ubuntu })

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{MajorVersion: 2, AgentStream: envtools.DevelStream}).
			Return(simpleStreams, nil),
		model.EXPECT().Type().Return(state.ModelTypeCAAS),

		st.EXPECT().ControllerConfig().Return(controller.Config{
			controller.ControllerUUIDKey: coretesting.ControllerTag.Id(),
			controller.CAASImageRepo: `
{
    "serveraddress": "quay.io",
    "auth": "xxxxx==",
    "repository": "test-account"
}
`[1:],
		}, nil),
		s.registryProvider.EXPECT().Tags("jujud-operator").Return(coretools.Versions{
			image.NewImageInfo(version.MustParse("2.9.8")), // skip: older than current version.
			image.NewImageInfo(version.MustParse("2.9.9")), // skip: older than current version.
			image.NewImageInfo(version.MustParse("2.9.10.1")),
			image.NewImageInfo(version.MustParse("2.9.10")),
			image.NewImageInfo(version.MustParse("2.9.11")),
			image.NewImageInfo(version.MustParse("2.9.12")),
			image.NewImageInfo(version.MustParse("2.9.13")), // skip: it's not released in simplestream yet.
		}, nil),
		s.registryProvider.EXPECT().GetArchitecture("jujud-operator", "2.9.10.1").Return("amd64", nil),
		s.registryProvider.EXPECT().GetArchitecture("jujud-operator", "2.9.10").Return("amd64", nil),
		s.registryProvider.EXPECT().GetArchitecture("jujud-operator", "2.9.11").Return("amd64", nil),
		s.registryProvider.EXPECT().GetArchitecture("jujud-operator", "2.9.12").Return("", errors.NotFoundf("2.9.12")), // This can only happen on a non-official registry account.
		s.registryProvider.EXPECT().Close().Return(nil),
	)

	result, err := api.FindTools(
		st, model, 2, 0, version.MustParse("2.9.9.1"), "", "", envtools.DevelStream,
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, gc.DeepEquals, coretools.Versions{
		&coretools.Tools{Version: version.MustParseBinary("2.9.10.1-ubuntu-amd64")},
		&coretools.Tools{Version: version.MustParseBinary("2.9.10-ubuntu-amd64")},
		&coretools.Tools{Version: version.MustParseBinary("2.9.11-ubuntu-amd64")},
	})
}

func (s *modelUpgradeSuite) TestDecideVersionFindToolUseAgentVersionMajor(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{
			MajorVersion: 3, // client major.
			MinorVersion: -1,
		}).Return(params.FindToolsResult{}, errors.New(`fail to exit early`)),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
	)

	targetVersion, err := api.DecideVersion(
		version.Zero, version.MustParse("3.9.99"), "", st, model,
	)
	c.Assert(err, gc.ErrorMatches, `cannot find tool version from simple streams: fail to exit early`)
	c.Assert(targetVersion, gc.DeepEquals, version.Zero)
}

func (s *modelUpgradeSuite) TestDecideVersionFindToolUseTargetMajor(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{
			MajorVersion: 4, // target major.
			MinorVersion: -1,
		}).Return(params.FindToolsResult{}, errors.New(`fail to exit early`)),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
	)

	targetVersion, err := api.DecideVersion(
		version.MustParse("4.9.99"), version.MustParse("3.9.99"), "", st, model,
	)
	c.Assert(err, gc.ErrorMatches, `cannot find tool version from simple streams: fail to exit early`)
	c.Assert(targetVersion, gc.DeepEquals, version.Zero)
}

func (s *modelUpgradeSuite) TestDecideVersionValidateAndUseTargetVersion(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	simpleStreams := params.FindToolsResult{
		List: []*coretools.Tools{
			{Version: version.MustParseBinary("3.9.98-ubuntu-amd64")},
		},
	}

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{
			MajorVersion: 3, MinorVersion: -1,
		}).Return(simpleStreams, nil),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
	)

	targetVersion, err := api.DecideVersion(
		version.MustParse("3.9.98"), version.MustParse("2.9.99"), "", st, model,
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(targetVersion, gc.DeepEquals, version.MustParse("3.9.98"))
}

func (s *modelUpgradeSuite) TestDecideVersionNextStable(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	simpleStreams := params.FindToolsResult{
		List: []*coretools.Tools{
			{Version: version.MustParseBinary("2.9.100-ubuntu-amd64")},
			{Version: version.MustParseBinary("2.10.99-ubuntu-amd64")},
		},
	}

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{
			MajorVersion: 2, MinorVersion: -1,
		}).Return(simpleStreams, nil),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
	)

	targetVersion, err := api.DecideVersion(
		version.Zero, version.MustParse("2.9.99"), "", st, model,
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(targetVersion, gc.DeepEquals, version.MustParse("2.10.99"))
}

func (s *modelUpgradeSuite) TestDecideVersionNewestCurrent(c *gc.C) {
	ctrl, api := s.getModelUpgraderAPI(c)
	defer ctrl.Finish()

	st := mocks.NewMockState(ctrl)
	model := mocks.NewMockModel(ctrl)
	st.EXPECT().Model().AnyTimes().Return(model, nil)

	simpleStreams := params.FindToolsResult{
		List: []*coretools.Tools{
			{Version: version.MustParseBinary("2.9.100-ubuntu-amd64")},
		},
	}

	gomock.InOrder(
		s.toolsFinder.EXPECT().FindTools(params.FindToolsParams{
			MajorVersion: 2, MinorVersion: -1,
		}).Return(simpleStreams, nil),
		model.EXPECT().Type().Return(state.ModelTypeIAAS),
	)

	targetVersion, err := api.DecideVersion(
		version.Zero, version.MustParse("2.9.99"), "", st, model,
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(targetVersion, gc.DeepEquals, version.MustParse("2.9.100"))
}
