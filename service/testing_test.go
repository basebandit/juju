// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/os/v2/series"
	"github.com/juju/retry"
	"github.com/juju/testing"
	"github.com/juju/version/v2"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/service/common"
	svctesting "github.com/juju/juju/service/common/testing"
)

// Stub stubs out the external functions used in the service package.
type Stub struct {
	*testing.Stub

	Version version.Binary
	Service Service
}

// GetVersion stubs out .
func (s *Stub) GetVersion() version.Binary {
	s.AddCall("GetVersion")

	// Pop the next error off the queue, even though we don't use it.
	s.NextErr()
	return s.Version
}

// DiscoverService stubs out service.DiscoverService.
func (s *Stub) DiscoverService(name string) (Service, error) {
	s.AddCall("DiscoverService", name)

	return s.Service, s.NextErr()
}

// BaseSuite is the base test suite for the application package.
type BaseSuite struct {
	testing.IsolationSuite

	Dirname string
	Name    string
	Conf    common.Conf
	Failure error

	Stub    *testing.Stub
	Service *svctesting.FakeService
	Patched *Stub
}

func (s *BaseSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)

	s.Dirname = c.MkDir()
	s.Name = "juju-agent-machine-0"
	s.Conf = common.Conf{
		Desc:      "some service",
		ExecStart: "/bin/jujud machine 0",
	}
	s.Failure = errors.New("<failed>")

	s.Service = svctesting.NewFakeService(s.Name, s.Conf)
	s.Stub = &s.Service.Stub
	s.Patched = &Stub{Stub: s.Stub}
	s.PatchValue(&discoverService, s.Patched.DiscoverService)
}

func (s *BaseSuite) PatchAttempts(retries int) {
	s.PatchValue(&installStartRetryStrategy, retry.CallArgs{
		Clock:    clock.WallClock,
		Delay:    time.Millisecond,
		Attempts: retries,
	})
}

func (s *BaseSuite) PatchSeries(ser string) {
	s.PatchValue(&series.HostSeries, func() (string, error) { return ser, nil })
}

func NewDiscoveryCheck(name string, running bool, failure error) discoveryCheck {
	return discoveryCheck{
		name: name,
		isRunning: func() (bool, error) {
			return running, failure
		},
	}
}

func (s *BaseSuite) PatchLocalDiscovery(checks ...discoveryCheck) {
	s.PatchValue(&discoveryFuncs, checks)
}

func (s *BaseSuite) PatchLocalDiscoveryDisable() {
	s.PatchLocalDiscovery()
}

func (s *BaseSuite) PatchLocalDiscoveryNoMatch(expected string) {
	// TODO(ericsnow) Pull from a list of supported init systems.
	names := []string{
		InitSystemUpstart,
		InitSystemSystemd,
		InitSystemWindows,
	}
	var checks []discoveryCheck
	for _, name := range names {
		checks = append(checks, NewDiscoveryCheck(name, name == expected, nil))
	}
	s.PatchLocalDiscovery(checks...)
}

func (s *BaseSuite) CheckFailure(c *gc.C, err error) {
	c.Check(errors.Cause(err), gc.Equals, s.Failure)
}
