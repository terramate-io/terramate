package module

// Vendor will vendor the given module inside the provided vendor
// dir. If the project is already vendored it will return as a success.
//
// Vendored modules will be located at:
//
// - <vendordir>/<domain>/<dir>/<subdir>/<ref>
//
// The whole path inside the vendor dir will be created if it not exists.
//
// The module source must be a Github source or a generic git one,
// identical to how it is used on Terraform module sources:
//
// - https://www.terraform.io/language/modules/sources#github
// - https://www.terraform.io/language/modules/sources#generic-git-repository
func Vendor(vendordir, modsource string) error {
	return nil
}
