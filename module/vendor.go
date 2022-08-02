// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package module

import (
	"fmt"
	"strings"

	"github.com/mineiros-io/terramate/errors"
)

// Source represents a module source
type Source struct {
	// Remote is the remote of the source that will be referenced
	// directly when downloading a source.
	Remote string

	// Ref is the specific reference of this source, if any.
	Ref string
}

const (
	// ErrUnsupportedModSrc indicates that a module source string is invalid.
	ErrUnsupportedModSrc errors.Kind = "unsupported module source"
)

// Vendor will vendor the given module inside the provided vendor
// dir. If the project is already vendored it will do nothing and return
// as a success.
//
// Vendored modules will be located at:
//
// - <vendordir>/<domain>/<dir>/<subdir>/<ref>
//
// The whole path inside the vendor dir will be created if it not exists.
// Vendoring is not recursive, so dependencies won't have their dependencies vendored.
// Vendoring will also not download git submodules, if any.
//
// The module source must be a valid Terraform Git/Github source reference as documented in:
//
// - https://www.terraform.io/language/modules/sources
//
// Source references that are not Git/Github are not supported.
func Vendor(vendordir, src Source) error {
	// Use: https://pkg.go.dev/github.com/hashicorp/go-getter#Detect
	// To detect modsource to a valid URL and then extract the path/reference
	// from the URL, so we can define the final path inside vendor.
	// go-getter can also download for us but it will update things if the dir
	// already exists, so we need to make sure that we don't call it if the dir
	// already exists.
	//
	// More details: https://github.com/hashicorp/terraform/blob/main/internal/getmodules/getter.go#L14
	return nil
}

// ParseSource parses the given modsource string. It returns an error if the modsource
// string is unsupported.
func ParseSource(modsource string) (Source, error) {
	ref := ""
	splitParams := strings.Split(modsource, "?")
	if len(splitParams) > 1 {
		// TODO(katcipis): handle invalid multiple ??
		// TODO(katcipis): handle missing ref
		// TODO(katcipis): handle extra params
		ref = strings.Split(splitParams[1], "=")[1]
	}
	modsource = splitParams[0]

	switch {
	case strings.HasPrefix(modsource, "github.com"):
		return Source{
			Remote: fmt.Sprintf("https://%s.git", modsource),
			Ref:    ref,
		}, nil
	case strings.HasPrefix(modsource, "git@"):
		return Source{
			Remote: modsource,
			Ref:    ref,
		}, nil
	case strings.HasPrefix(modsource, "git::"):
		return Source{
			Remote: strings.TrimPrefix(modsource, "git::"),
			Ref:    ref,
		}, nil

	default:
		return Source{}, errors.E(ErrUnsupportedModSrc)
	}
}
