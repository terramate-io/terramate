// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package download_test

import (
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/modvendor/download"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/test"
)

func TestMergeVendorReportsNoReports(t *testing.T) {
	t.Parallel()
	reports := make(chan download.Report)
	merged := download.MergeVendorReports(reports)

	close(reports)

	assertReportsIsClosed(t, merged)
}

func TestMergeVendorReportsSingleReport(t *testing.T) {
	t.Parallel()
	reports := make(chan download.Report)
	merged := download.MergeVendorReports(reports)

	want := download.NewReport(project.NewPath("/test"))

	reports <- want
	close(reports)

	got := <-merged
	assertVendorReport(t, want, got)
	assertReportsIsClosed(t, merged)
}

func TestMergeVendorReportsNReports(t *testing.T) {
	t.Parallel()
	reports := make(chan download.Report)
	merged := download.MergeVendorReports(reports)

	vendorDir := project.NewPath("/test")

	rep1 := download.NewReport(vendorDir)
	vendored1key := project.NewPath("/test")
	vendored1val := download.Vendored{
		Source: test.ParseSource(t, "github.com/terramate-io/terramate?ref=v1"),
		Dir:    project.NewPath("/dir"),
	}
	rep1.Vendored[vendored1key] = vendored1val
	ignored1 := []download.IgnoredVendor{
		{
			RawSource: "some test source",
			Reason:    errors.E("some error"),
		},
	}
	rep1.Ignored = ignored1

	rep2 := download.NewReport(vendorDir)
	vendored2key := project.NewPath("/test2")
	vendored2val := download.Vendored{
		Source: test.ParseSource(t, "github.com/terramate-io/terramate?ref=v2"),
		Dir:    project.NewPath("/dir2"),
	}
	rep2.Vendored[vendored2key] = vendored2val
	ignored2 := []download.IgnoredVendor{
		{
			RawSource: "some test source 2",
			Reason:    errors.E("some error 2"),
		},
	}
	rep2.Ignored = ignored2

	reports <- rep1
	reports <- rep2
	close(reports)

	want := download.NewReport(vendorDir)
	want.Vendored[vendored1key] = vendored1val
	want.Vendored[vendored2key] = vendored2val
	want.Ignored = append(ignored1, ignored2...)

	got := <-merged
	assertVendorReport(t, want, got)
	assertReportsIsClosed(t, merged)
}

func TestRemoveIgnoredFromReportByErrKind(t *testing.T) {
	t.Parallel()
	got := download.NewReport(project.NewPath("/test"))
	got.Ignored = []download.IgnoredVendor{
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrAlreadyVendored, "some error"),
		},
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrUnsupportedModSrc, "some error"),
		},
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrAlreadyVendored, "some error"),
		},
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrModRefEmpty, "some error"),
		},
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrAlreadyVendored, "some error"),
		},
	}

	got.RemoveIgnoredByKind(download.ErrAlreadyVendored)

	want := download.NewReport(project.NewPath("/test"))
	want.Ignored = []download.IgnoredVendor{
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrUnsupportedModSrc, "some error"),
		},
		{
			RawSource: "some test source",
			Reason:    errors.E(download.ErrModRefEmpty, "some error"),
		},
	}

	assertVendorReport(t, want, got)
}

func assertReportsIsClosed(t *testing.T, r <-chan download.Report) {
	t.Helper()

	v, ok := <-r
	assert.IsTrue(t, !ok, "want closed channel got report: %v", v)
}
