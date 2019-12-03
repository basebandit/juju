// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc_test

import (
	"fmt"

	"github.com/juju/cmd"
	"github.com/juju/cmd/cmdtesting"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/worker/uniter/runner/jujuc"
)

type ActionFailSuite struct {
	ContextSuite
}

type actionFailContext struct {
	jujuc.Context
	actionFailed  bool
	actionMessage string
}

func (ctx *actionFailContext) SetActionMessage(message string) error {
	ctx.actionMessage = message
	return nil
}

func (ctx *actionFailContext) SetActionFailed() error {
	ctx.actionFailed = true
	return nil
}

type nonActionFailContext struct {
	jujuc.Context
}

func (ctx *nonActionFailContext) SetActionMessage(message string) error {
	return fmt.Errorf("not running an action")
}

func (ctx *nonActionFailContext) SetActionFailed() error {
	return fmt.Errorf("not running an action")
}

var _ = gc.Suite(&ActionFailSuite{})

func (s *ActionFailSuite) TestActionFail(c *gc.C) {
	var actionFailTests = []struct {
		summary string
		command []string
		message string
		failed  bool
		errMsg  string
		code    int
	}{{
		summary: "no parameters sets a default message",
		command: []string{},
		message: "action/function failed without reason given, check action/function for errors",
		failed:  true,
	}, {
		summary: "a message sent is set as the failure reason",
		command: []string{"a failure message"},
		message: "a failure message",
		failed:  true,
	}, {
		summary: "extra arguments are an error, leaving the action not failed",
		command: []string{"a failure message", "something else"},
		errMsg:  "ERROR unrecognized args: [\"something else\"]\n",
		code:    2,
	}}

	for i, t := range actionFailTests {
		c.Logf("test %d: %s", i, t.summary)
		hctx := &actionFailContext{}
		com, err := jujuc.NewCommand(hctx, cmdString("action-fail"))
		c.Assert(err, jc.ErrorIsNil)
		ctx := cmdtesting.Context(c)
		code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(com), ctx, t.command)
		c.Check(code, gc.Equals, t.code)
		c.Check(bufferString(ctx.Stderr), gc.Equals, t.errMsg)
		c.Check(hctx.actionMessage, gc.Equals, t.message)
		c.Check(hctx.actionFailed, gc.Equals, t.failed)
	}
}

func (s *ActionFailSuite) TestNonActionSetActionFailedFails(c *gc.C) {
	hctx := &nonActionFailContext{}
	com, err := jujuc.NewCommand(hctx, cmdString("action-fail"))
	c.Assert(err, jc.ErrorIsNil)
	ctx := cmdtesting.Context(c)
	code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(com), ctx, []string{"oops"})
	c.Check(code, gc.Equals, 1)
	c.Check(bufferString(ctx.Stderr), gc.Equals, "ERROR not running an action\n")
	c.Check(bufferString(ctx.Stdout), gc.Equals, "")
}

func (s *ActionFailSuite) TestHelp(c *gc.C) {
	hctx, _ := s.NewHookContext()
	com, err := jujuc.NewCommand(hctx, cmdString("action-fail"))
	c.Assert(err, jc.ErrorIsNil)
	ctx := cmdtesting.Context(c)
	code := cmd.Main(jujuc.NewJujucCommandWrappedForTest(com), ctx, []string{"--help"})
	c.Assert(code, gc.Equals, 0)
	c.Assert(bufferString(ctx.Stdout), gc.Equals, `Usage: action-fail ["<failure message>"]

Summary:
set action/function fail status with message

Details:
action-fail sets the fail state of the action/function with a given error message.  Using
action-fail without a failure message will set a default message indicating a
problem with the action/function.
`)
	c.Assert(bufferString(ctx.Stderr), gc.Equals, "")
}
