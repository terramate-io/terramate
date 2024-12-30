// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run

import "sync"

// OnceMap contains map of key/value pairs that is safe for concurrent initialization.
type OnceMap[K ~string, V any] struct {
	mtx  sync.RWMutex
	data map[K]V
}

// NewOnceMap returns a new empty OnceMap.
func NewOnceMap[K ~string, V any]() *OnceMap[K, V] {
	return &OnceMap[K, V]{data: make(map[K]V)}
}

// GetOrInit obtains a value for the given key if the value exists in the map, or
// initializes it with the given init function.
// This function is safe to be called concurrently.
func (m *OnceMap[K, V]) GetOrInit(k K, init func() (V, error)) (V, error) {
	// Read-lock and check if value already exists.
	m.mtx.RLock()
	v, found := m.data[k]
	m.mtx.RUnlock()

	if found {
		return v, nil
	}

	// If not, write-lock, check again, and maybe initialize it.
	m.mtx.Lock()
	defer m.mtx.Unlock()

	v, found = m.data[k]
	if found {
		return v, nil
	}

	v, err := init()
	if err != nil {
		return v, err
	}
	m.data[k] = v
	return v, nil
}
