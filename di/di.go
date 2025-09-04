// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package di

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type bindingsKey struct{}

// Bindings map interfaces to implementations and allow to retrieve them from a context.
type Bindings struct {
	ctx          context.Context
	initializers map[string]func(context.Context) (any, error)
	instances    map[string]any
	cycleChecker []string
}

// NewBindings creates a new bindings object and adds it to the given context.
func NewBindings(ctx context.Context) *Bindings {
	b := &Bindings{
		initializers: map[string]func(context.Context) (any, error){},
		instances:    map[string]any{},
		cycleChecker: []string{},
	}
	b.ctx = context.WithValue(ctx, bindingsKey{}, b)
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
func WithBindings(ctx context.Context, b *Bindings) context.Context {
	return context.WithValue(ctx, bindingsKey{}, b)
}

// Bind binds an interface to the given factory function, which will be used to instantiate the implementation.
// This instance exists once within the scope of the bindings.
func Bind[Ifc, Impl any](b *Bindings, factory func(context.Context) (Impl, error)) error {
	ifcTyp := reflect.TypeFor[Ifc]()
	implTyp := reflect.TypeFor[Impl]()
	if !implTyp.Implements(ifcTyp) {
		return fmt.Errorf("%s does not implement %s", implTyp, ifcTyp)
	}

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
func Override[Ifc, Impl any](b *Bindings, factory func(context.Context, Ifc) (Impl, error)) error {
	ifcTyp := reflect.TypeFor[Ifc]()
	implTyp := reflect.TypeFor[Impl]()
	if !implTyp.Implements(ifcTyp) {
		return fmt.Errorf("%s does not implement %s", implTyp, ifcTyp)
	}

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

	if v, ok := inst.(Ifc); ok {
		return v, nil
	} else {
		return zero, fmt.Errorf("mismatched instance type for %s: %T", ifcKey, v)
	}
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
		} else {
			return nil, fmt.Errorf("invalid bindings type")
		}
	} else {
		return nil, fmt.Errorf("context contains no instance bindings")
	}
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
