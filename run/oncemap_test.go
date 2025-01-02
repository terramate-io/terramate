// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package run_test

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/run"
)

func TestOnceMap(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		outerCount := 0
		innerCount := 0

		data := run.NewOnceMap[string, *run.OnceMap[string, string]]()

		for outer := 0; outer < 10; outer++ {
			k1 := strconv.Itoa(outer)
			v1, err1 := data.GetOrInit(k1, func() (*run.OnceMap[string, string], error) {
				outerCount++
				return run.NewOnceMap[string, string](), nil
			})
			assert.NoError(t, err1)

			for inner := 0; inner < 10; inner++ {
				k2 := strconv.Itoa(inner)
				v2, err2 := v1.GetOrInit(k2, func() (string, error) {
					innerCount++
					return fmt.Sprintf("%d_%d", outer, inner), nil
				})
				assert.NoError(t, err2)
				assert.EqualStrings(t, fmt.Sprintf("%d_%d", outer, inner), v2)
			}
		}

		assert.EqualInts(t, 10, outerCount, "outer count")
		assert.EqualInts(t, 100, innerCount, "inner count")
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		count := 0

		m := run.NewOnceMap[string, string]()

		_, err := m.GetOrInit("k", func() (string, error) {
			count++
			return "", errors.New("failed")
		})

		assert.EqualErrs(t, err, errors.New("failed"))

		v, err := m.GetOrInit("k", func() (string, error) {
			count++
			return "success", nil
		})

		assert.EqualStrings(t, "success", v)
		assert.NoError(t, err)
	})

	t.Run("concurrent", func(t *testing.T) {
		t.Parallel()

		var outerCount atomic.Int32
		var innerCount atomic.Int32

		data := run.NewOnceMap[string, *run.OnceMap[string, string]]()

		var wg sync.WaitGroup

		for outer := 0; outer < 10; outer++ {
			outer := outer

			wg.Add(1 + 10)

			go func() {
				defer wg.Done()

				k1 := strconv.Itoa(outer)
				v1, _ := data.GetOrInit(k1, func() (*run.OnceMap[string, string], error) {
					outerCount.Add(1)
					return run.NewOnceMap[string, string](), nil
				})

				for inner := 0; inner < 10; inner++ {
					inner := inner

					go func() {
						defer wg.Done()

						k2 := strconv.Itoa(inner)
						_, _ = v1.GetOrInit(k2, func() (string, error) {
							innerCount.Add(1)
							return "blah", nil
						})
					}()
				}
			}()
		}

		wg.Wait()

		assert.EqualInts(t, 10, int(outerCount.Load()), "outer count")
		assert.EqualInts(t, 100, int(innerCount.Load()), "inner inner")
	})
}
