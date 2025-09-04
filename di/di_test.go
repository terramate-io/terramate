// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package di_test

import (
	"context"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/di"
)

type MyService1 interface {
	DoStuff1() string
}

type MyService2 interface {
	DoStuff2() string
}

type MyService3 interface {
	DoStuff3()
}

type MyService1Impl struct {
}

type MyService2Impl struct {
}

func newService1Impl(ctx context.Context) (*MyService1Impl, error) {
	return &MyService1Impl{}, nil
}

func newService2Impl(ctx context.Context) (*MyService2Impl, error) {
	return &MyService2Impl{}, nil
}

func (*MyService1Impl) DoStuff1() string {
	return "hi"
}

func (*MyService2Impl) DoStuff2() string {
	return "bye"
}

func TestDI(t *testing.T) {
	b := di.NewBindings(t.Context())
	assert.IsTrue(t, b != nil)

	di.Require[MyService1](b)

	var err error
	err = di.Bind[MyService1](b, newService1Impl)
	assert.NoError(t, err)

	err = di.Bind[MyService1](b, newService2Impl)
	assert.IsTrue(t, err != nil)
	assert.StringMatch(t, ".*not implement.*", err.Error())

	err = di.Bind[MyService2](b, newService2Impl)
	assert.NoError(t, err)

	assert.IsTrue(t, di.Validate(b) == nil)

	di.Require[MyService3](b)
	err = di.Validate(b)
	assert.IsTrue(t, err != nil)
	assert.StringMatch(t, "no initializer.*MyService3", err.Error())

	runCtx := di.WithBindings(t.Context(), b)

	svc1, err := di.Get[MyService1](runCtx)
	assert.NoError(t, err)

	svc2, err := di.Get[MyService2](runCtx)
	assert.NoError(t, err)

	assert.EqualStrings(t, svc1.DoStuff1(), "hi")
	assert.EqualStrings(t, svc2.DoStuff2(), "bye")
}

func TestDep(t *testing.T) {
	newService1Impl := func(ctx context.Context) (*MyService1Impl, error) {
		_, err := di.Get[MyService2](ctx)
		if err != nil {
			return nil, err
		}
		return &MyService1Impl{}, nil
	}

	newService2Impl := func(ctx context.Context) (*MyService2Impl, error) {
		return &MyService2Impl{}, nil
	}

	b := di.NewBindings(t.Context())

	err := di.Bind[MyService1](b, newService1Impl)
	assert.NoError(t, err)
	err = di.Bind[MyService2](b, newService2Impl)
	assert.NoError(t, err)

	err = di.InitAll(b)
	assert.NoError(t, err)
}

func TestCircularDep(t *testing.T) {
	newService1CircularImpl := func(ctx context.Context) (*MyService1Impl, error) {
		_, err := di.Get[MyService2](ctx)
		if err != nil {
			return nil, err
		}
		return &MyService1Impl{}, nil
	}

	newService2CircularImpl := func(ctx context.Context) (*MyService2Impl, error) {
		_, err := di.Get[MyService1](ctx)
		if err != nil {
			return nil, err
		}
		return &MyService2Impl{}, nil
	}

	b := di.NewBindings(t.Context())

	err := di.Bind[MyService1](b, newService1CircularImpl)
	assert.NoError(t, err)
	err = di.Bind[MyService2](b, newService2CircularImpl)
	assert.NoError(t, err)

	err = di.InitAll(b)
	assert.IsTrue(t, err != nil)
	assert.StringMatch(t, "circular initialization.*", err.Error())
}

type MyService2ImplOverride struct {
	parent MyService2
}

func newService2ImplOverride(ctx context.Context, parent MyService2) (*MyService2ImplOverride, error) {
	return &MyService2ImplOverride{parent: parent}, nil
}

func (svc *MyService2ImplOverride) DoStuff2() string {
	return svc.parent.DoStuff2() + " and farewell"
}

func TestOverride(t *testing.T) {
	b := di.NewBindings(t.Context())
	assert.IsTrue(t, b != nil)

	di.Require[MyService2](b)

	var err error

	err = di.Override[MyService2](b, newService2ImplOverride)
	assert.IsTrue(t, err != nil)
	assert.StringMatch(t, "is not yet bound", err.Error())

	err = di.Bind[MyService2](b, newService2Impl)
	assert.NoError(t, err)

	err = di.Override[MyService2](b, newService2ImplOverride)
	assert.NoError(t, err)

	err = di.Validate(b)
	assert.NoError(t, err)

	runCtx := di.WithBindings(t.Context(), b)

	svc2, err := di.Get[MyService2](runCtx)
	assert.NoError(t, err)

	assert.EqualStrings(t, svc2.DoStuff2(), "bye and farewell")
}
