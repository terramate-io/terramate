// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package di provides a lightweight dependency injection framework.
package di

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Bindings map interfaces to implementations and allow to retrieve them from a context.
type Bindings struct {
	ctx          context.Context
	initializers map[string]func(context.Context) (any, error)
	instances    map[string]any
	cycleChecker []string
}

// Factory is the factory function type.
// The given context supports di.Get to lookup other interfaces.
type Factory[Ifc any] func(context.Context) (Ifc, error)

// OverrideFactory is the override factory function type.
// It is passed the overridden instance as additional argument.
type OverrideFactory[Ifc any] func(context.Context, Ifc) (Ifc, error)

// NewBindings creates a new bindings object.
// The given context will be used for calls to factory functions.
func NewBindings(ctx context.Context) *Bindings {
	b := &Bindings{
		initializers: map[string]func(context.Context) (any, error){},
		instances:    map[string]any{},
		cycleChecker: []string{},
	}
	b.ctx = WithBindings(ctx, b)
	return b
}

// Require declares an interface as required to be bound.
// Any unbound interfaces will result in an error when calling Validate().
func Require[Ifc any](b *Bindings) {
	ifcKey := reflect.TypeFor[Ifc]().String()

	if _, found := b.initializers[ifcKey]; !found {
		b.initializers[ifcKey] = nil
	}
}

// WithBindings returns a copy of the given context with bindings.
// The context can then be used with di.Get.
func WithBindings(ctx context.Context, b *Bindings) context.Context {
	return context.WithValue(ctx, bindingsKey{}, b)
}

// Bind binds an interface to the given factory function, which will be used to instantiate the implementation.
// The created instance exists once within the scope of the bindings.
// Calling Bind twice on the same interface will result in an error - see Override instead.
func Bind[Ifc any](b *Bindings, factory Factory[Ifc]) error {
	ifcTyp := reflect.TypeFor[Ifc]()
	ifcKey := ifcTyp.String()
	if initFn := b.initializers[ifcKey]; initFn != nil {
		return fmt.Errorf("interface %s is already bound", ifcKey)
	}

	b.initializers[ifcKey] = func(ctx context.Context) (any, error) {
		inst, err := factory(ctx)
		if err != nil {
			return nil, err
		}

		return inst, nil
	}

	return nil
}

// Override overrides an already bound interface.
// Calling Override on an unbound interface will result in an error.
func Override[Ifc any](b *Bindings, factory OverrideFactory[Ifc]) error {
	ifcTyp := reflect.TypeFor[Ifc]()

	ifcKey := ifcTyp.String()
	parentInitFn := b.initializers[ifcKey]
	if parentInitFn == nil {
		return fmt.Errorf("interface %s is not yet bound", ifcKey)
	}

	b.initializers[ifcKey] = func(ctx context.Context) (any, error) {
		parentInstAny, err := parentInitFn(ctx)
		if err != nil {
			return nil, err
		}

		parentInst, ok := parentInstAny.(Ifc)
		if !ok {
			return nil, fmt.Errorf("mismatched instance type for %s: %T", ifcKey, parentInst)
		}

		inst, err := factory(ctx, parentInst)
		if err != nil {
			return nil, err
		}
		return inst, nil
	}

	return nil
}

// Validate checks if all required interfaces are bound.
func Validate(b *Bindings) error {
	for ifcKey, v := range b.initializers {
		if v == nil {
			return fmt.Errorf("no initializer for type %s", ifcKey)
		}
	}
	return nil
}

// Get returns the implementation bound to the given interface.
// If the instance has not been initialized, it will be initialized lazily.
func Get[Ifc any](ctx context.Context) (Ifc, error) {
	var zero Ifc
	ifcKey := reflect.TypeFor[Ifc]().String()

	b, err := bindingsFromContext(ctx)
	if err != nil {
		return zero, err
	}

	inst, err := getOrInit(b, ifcKey)
	if err != nil {
		return zero, err
	}

	v, ok := inst.(Ifc)
	if !ok {
		return zero, fmt.Errorf("mismatched instance type for %s: %T", ifcKey, v)
	}

	return v, nil
}

// InitAll initializes all currently bound interfaces.
func InitAll(b *Bindings) error {
	for ifcKey := range b.initializers {
		_, err := getOrInit(b, ifcKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func bindingsFromContext(ctx context.Context) (*Bindings, error) {
	if v := ctx.Value(bindingsKey{}); v != nil {
		if u, ok := v.(*Bindings); ok {
			return u, nil
		}
		return nil, fmt.Errorf("invalid bindings type")
	}
	return nil, fmt.Errorf("context contains no instance bindings")
}

func getOrInit(b *Bindings, ifcKey string) (any, error) {
	inst, found := b.instances[ifcKey]
	if !found {
		initFn, found := b.initializers[ifcKey]
		if !found || initFn == nil {
			return nil, fmt.Errorf("no initializer for type %s", ifcKey)
		}

		if slices.Contains(b.cycleChecker, ifcKey) {
			return nil, fmt.Errorf("circular initialization %s -> %s", strings.Join(b.cycleChecker, " -> "), ifcKey)
		}

		b.cycleChecker = append(b.cycleChecker, ifcKey)

		var err error
		inst, err = initFn(b.ctx)

		b.cycleChecker = b.cycleChecker[:len(b.cycleChecker)-1]

		if err != nil {
			return nil, err
		}

		b.instances[ifcKey] = inst
	}

	return inst, nil
}

type bindingsKey struct{}
