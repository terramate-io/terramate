package module

// Vendor will vendor the given module inside the provided vendor
// dir. If the project is already vendored it will do nothing and return
// as a success.
//
// Vendored modules will be located at:
//
// - <vendordir>/<domain>/<dir>/<subdir>/<ref>
//
// The whole path inside the vendor dir will be created if it not exists.
//
// The module source must be a valid Terraform source reference as documented in:
//
// - https://www.terraform.io/language/modules/sources
func Vendor(vendordir, modsource string) error {
	// Use: https://pkg.go.dev/github.com/hashicorp/go-getter#Detect
	// To detect modsource to a valid URL and then extract the path/reference
	// from the URL, so we can define the final path inside vendor.
	// go-getter can also download for us but it will update things if the dir
	// already exists, so we need to make sure that we don't call it if the dir
	// already exists.
	return nil
}
