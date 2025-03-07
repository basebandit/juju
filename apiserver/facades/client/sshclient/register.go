// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package sshclient

import (
	stdcontext "context"
	"reflect"

	"github.com/juju/errors"

	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/caas"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/state/stateenvirons"
)

// Register is called to expose a package of facades onto a given registry.
func Register(registry facade.FacadeRegistry) {
	registry.MustRegister("SSHClient", 1, func(ctx facade.Context) (facade.Facade, error) {
		return newFacadeV2(ctx)
	}, reflect.TypeOf((*FacadeV2)(nil)))
	registry.MustRegister("SSHClient", 2, func(ctx facade.Context) (facade.Facade, error) {
		return newFacadeV2(ctx) // v2 adds AllAddresses() method.
	}, reflect.TypeOf((*FacadeV2)(nil)))
	registry.MustRegister("SSHClient", 3, func(ctx facade.Context) (facade.Facade, error) {
		return newFacadeV3(ctx) // v3 adds Leader() method.
	}, reflect.TypeOf((*Facade)(nil)))
	registry.MustRegister("SSHClient", 4, func(ctx facade.Context) (facade.Facade, error) {
		return newFacade(ctx) // v4 adds ModelCredentialForSSH() method.
	}, reflect.TypeOf((*Facade)(nil)))
}

func newFacade(ctx facade.Context) (*Facade, error) {
	st := ctx.State()
	m, err := st.Model()
	if err != nil {
		return nil, errors.Trace(err)
	}
	leadershipReader, err := ctx.LeadershipReader(m.UUID())
	if err != nil {
		return nil, errors.Trace(err)
	}
	facadeBackend := backend{
		State:               st,
		EnvironConfigGetter: stateenvirons.EnvironConfigGetter{Model: m},
		controllerTag:       m.ControllerTag(),
		modelTag:            m.ModelTag(),
	}
	return internalFacade(
		&facadeBackend,
		leadershipReader,
		ctx.Auth(),
		context.CallContext(st),
		func(ctx stdcontext.Context, args environs.OpenParams) (Broker, error) {
			return caas.New(ctx, args)
		},
	)
}

// newFacadeV3 is used for API registration.
func newFacadeV3(ctx facade.Context) (*FacadeV3, error) {
	f, err := newFacade(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &FacadeV3{f}, nil
}

// newFacadeV2 is used for API registration.
func newFacadeV2(ctx facade.Context) (*FacadeV2, error) {
	f, err := newFacadeV3(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &FacadeV2{f}, nil
}
