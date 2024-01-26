// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package safeguard

import "github.com/terramate-io/terramate/errors"

// Keyword is a safeguard keyword.
type Keyword string

// Keywords is list of ketywords.
type Keywords []Keyword

// Available keywords.
const (
	All            Keyword = "all"
	None           Keyword = "none"
	Git            Keyword = "git"
	GitUntracked   Keyword = "git-untracked"
	GitUncommitted Keyword = "git-uncommitted"
	GitOutOfSync   Keyword = "git-out-of-sync"
	Outdated       Keyword = "outdated-code"
)

// FromStrings constructs a Keywords list out of a list of strings.
func FromStrings(strs []string) (Keywords, error) {
	var keywords Keywords
	for _, str := range strs {
		k := Keyword(str)
		if !k.IsValid() {
			return nil, errors.E(`invalid keyword %q`, str)
		}
		keywords = append(keywords, k)
	}
	return keywords, nil
}

// Validate the keywords.
func (ks Keywords) Validate() error {
	for _, k := range ks {
		if !k.IsValid() {
			return errors.E("invalid safeguard keyword: %s", k)
		}
	}
	return nil
}

// Has tell if target exists in the list of keywords.
func (ks Keywords) Has(target ...Keyword) bool {
	for _, t := range target {
		for _, k := range ks {
			if k == t {
				return true
			}
		}
	}
	return false
}

// IsValid checks if k is valid.
func (k Keyword) IsValid() bool {
	valid := map[Keyword]bool{
		All:            true,
		None:           true,
		Git:            true,
		GitUntracked:   true,
		GitUncommitted: true,
		GitOutOfSync:   true,
		Outdated:       true,
	}
	return valid[k]
}
