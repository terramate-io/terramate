package generate_test

import (
	"testing"
)

func TestGenerateFile(t *testing.T) {
	//tcases := []testcase{
	//{
	//name: "no generated HCL",
	//layout: []string{
	//"s:stacks/stack-1",
	//"s:stacks/stack-2",
	//},
	//},
	//{
	//name: "empty generate_hcl block generates nothing",
	//layout: []string{
	//"s:stacks/stack-1",
	//"s:stacks/stack-2",
	//},
	//configs: []hclconfig{
	//{
	//path: "/stacks",
	//add: generateHCL(
	//labels("empty"),
	//content(),
	//),
	//},
	//},
	//},
	//{
	//name: "generate HCL for all stacks on parent",
	//layout: []string{
	//"s:stacks/stack-1",
	//"s:stacks/stack-2",
	//},
	//configs: []hclconfig{
	//{
	//path: "/stacks",
	//add: hcldoc(
	//generateHCL(
	//labels("backend.tf"),
	//content(
	//backend(
	//labels("test"),
	//expr("prefix", "global.backend_prefix"),
	//),
	//),
	//),
	//generateHCL(
	//labels("locals.tf"),
	//content(
	//locals(
	//expr("stackpath", "terramate.path"),
	//expr("local_a", "global.local_a"),
	//expr("local_b", "global.local_b"),
	//expr("local_c", "global.local_c"),
	//expr("local_d", "tm_try(global.local_d.field, null)"),
	//),
	//),
	//),
	//generateHCL(
	//labels("provider.tf"),
	//content(
	//provider(
	//labels("name"),
	//expr("data", "global.provider_data"),
	//),
	//terraform(
	//requiredProviders(
	//expr("name", `{
	//source  = "integrations/name"
	//version = global.provider_version
	//}`),
	//),
	//),
	//terraform(
	//expr("required_version", "global.terraform_version"),
	//),
	//),
	//),
	//),
	//},
	//{
	//path: "/stacks/stack-1",
	//add: globals(
	//str("local_a", "stack-1-local"),
	//boolean("local_b", true),
	//number("local_c", 666),
	//attr("local_d", `{ field = "local_d_field"}`),
	//str("backend_prefix", "stack-1-backend"),
	//str("provider_data", "stack-1-provider-data"),
	//str("provider_version", "stack-1-provider-version"),
	//str("terraform_version", "stack-1-terraform-version"),
	//),
	//},
	//{
	//path: "/stacks/stack-2",
	//add: globals(
	//str("local_a", "stack-2-local"),
	//boolean("local_b", false),
	//number("local_c", 777),
	//attr("local_d", `{ oopsie = "local_d_field"}`),
	//str("backend_prefix", "stack-2-backend"),
	//str("provider_data", "stack-2-provider-data"),
	//str("provider_version", "stack-2-provider-version"),
	//str("terraform_version", "stack-2-terraform-version"),
	//),
	//},
	//},
	//wantHCL: []generatedHCL{
	//{
	//stack: "/stacks/stack-1",
	//hcls: map[string]fmt.Stringer{
	//"backend.tf": backend(
	//labels("test"),
	//str("prefix", "stack-1-backend"),
	//),
	//"locals.tf": locals(
	//str("local_a", "stack-1-local"),
	//boolean("local_b", true),
	//number("local_c", 666),
	//str("local_d", "local_d_field"),
	//str("stackpath", "/stacks/stack-1"),
	//),
	//"provider.tf": hcldoc(
	//provider(
	//labels("name"),
	//str("data", "stack-1-provider-data"),
	//),
	//terraform(
	//requiredProviders(
	//attr("name", `{
	//source  = "integrations/name"
	//version = "stack-1-provider-version"
	//}`),
	//),
	//),
	//terraform(
	//str("required_version", "stack-1-terraform-version"),
	//),
	//),
	//},
	//},
	//{
	//stack: "/stacks/stack-2",
	//hcls: map[string]fmt.Stringer{
	//"backend.tf": backend(
	//labels("test"),
	//str("prefix", "stack-2-backend"),
	//),
	//"locals.tf": locals(
	//str("local_a", "stack-2-local"),
	//boolean("local_b", false),
	//number("local_c", 777),
	//attr("local_d", "null"),
	//str("stackpath", "/stacks/stack-2"),
	//),
	//"provider.tf": hcldoc(
	//provider(
	//labels("name"),
	//str("data", "stack-2-provider-data"),
	//),
	//terraform(
	//requiredProviders(
	//attr("name", `{
	//source  = "integrations/name"
	//version = "stack-2-provider-version"
	//}`),
	//),
	//),
	//terraform(
	//str("required_version", "stack-2-terraform-version"),
	//),
	//),
	//},
	//},
	//},
	//wantReport: generate.Report{
	//Successes: []generate.Result{
	//{
	//StackPath: "/stacks/stack-1",
	//Created:   []string{"backend.tf", "locals.tf", "provider.tf"},
	//},
	//{
	//StackPath: "/stacks/stack-2",
	//Created:   []string{"backend.tf", "locals.tf", "provider.tf"},
	//},
	//},
	//},
	//},
	//{
	//name: "generate HCL with traversal of unknown namespaces",
	//layout: []string{
	//"s:stacks/stack-1",
	//"s:stacks/stack-2",
	//},
	//configs: []hclconfig{
	//{
	//path: "/stacks",
	//add: hcldoc(
	//generateHCL(
	//labels("traversal.tf"),
	//content(
	//block("traversal",
	//expr("locals", "local.hi"),
	//expr("some_anything", "something.should_work"),
	//expr("multiple_traversal", "one.two.three.four.five"),
	//),
	//),
	//),
	//),
	//},
	//},
	//wantHCL: []generatedHCL{
	//{
	//stack: "/stacks/stack-1",
	//hcls: map[string]fmt.Stringer{
	//"traversal.tf": hcldoc(
	//block("traversal",
	//expr("locals", "local.hi"),
	//expr("multiple_traversal", "one.two.three.four.five"),
	//expr("some_anything", "something.should_work"),
	//),
	//),
	//},
	//},
	//{
	//stack: "/stacks/stack-2",
	//hcls: map[string]fmt.Stringer{
	//"traversal.tf": hcldoc(
	//block("traversal",
	//expr("locals", "local.hi"),
	//expr("multiple_traversal", "one.two.three.four.five"),
	//expr("some_anything", "something.should_work"),
	//),
	//),
	//},
	//},
	//},
	//wantReport: generate.Report{
	//Successes: []generate.Result{
	//{
	//StackPath: "/stacks/stack-1",
	//Created:   []string{"traversal.tf"},
	//},
	//{
	//StackPath: "/stacks/stack-2",
	//Created:   []string{"traversal.tf"},
	//},
	//},
	//},
	//},
	//{
	//// TODO(katcipis): define a proper behavior where
	//// directories are allowed but in a constrained fashion.
	//// This is a quick fix to avoid creating files on arbitrary
	//// places around the file system.
	//name: "generate HCL with dir separators on label name fails",
	//layout: []string{
	//"s:stacks/stack-1",
	//"s:stacks/stack-2",
	//"s:stacks/stack-3",
	//"s:stacks/stack-4",
	//},
	//configs: []hclconfig{
	//{
	//path: "/stacks/stack-1",
	//add: hcldoc(
	//generateHCL(
	//labels("/name.tf"),
	//content(
	//block("something"),
	//),
	//),
	//),
	//},
	//{
	//path: "/stacks/stack-2",
	//add: hcldoc(
	//generateHCL(
	//labels("./name.tf"),
	//content(
	//block("something"),
	//),
	//),
	//),
	//},
	//{
	//path: "/stacks/stack-3",
	//add: hcldoc(
	//generateHCL(
	//labels("./dir/name.tf"),
	//content(
	//block("something"),
	//),
	//),
	//),
	//},
	//{
	//path: "/stacks/stack-4",
	//add: hcldoc(
	//generateHCL(
	//labels("dir/name.tf"),
	//content(
	//block("something"),
	//),
	//),
	//),
	//},
	//},
	//wantReport: generate.Report{
	//Failures: []generate.FailureResult{
	//{
	//Result: generate.Result{
	//StackPath: "/stacks/stack-1",
	//},
	//Error: errors.E(generate.ErrInvalidFilePath),
	//},
	//{
	//Result: generate.Result{
	//StackPath: "/stacks/stack-2",
	//},
	//Error: errors.E(generate.ErrInvalidFilePath),
	//},
	//{
	//Result: generate.Result{
	//StackPath: "/stacks/stack-3",
	//},
	//Error: errors.E(generate.ErrInvalidFilePath),
	//},
	//{
	//Result: generate.Result{
	//StackPath: "/stacks/stack-4",
	//},
	//Error: errors.E(generate.ErrInvalidFilePath),
	//},
	//},
	//},
	//},
	//}
}
