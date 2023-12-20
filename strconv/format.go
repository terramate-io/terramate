// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package strconv

import "strconv"

// Itoa64 returns the string representation of the provided int64 using base 10.
func Itoa64(i int64) string { return strconv.FormatInt(i, 10) }

// Atoi64 returns the int64 represented in the given string.
func Atoi64(a string) (int64, error) {
	return strconv.ParseInt(a, 10, 64)
}
