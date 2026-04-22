// Package shim wraps the upstream github.com/buildwithdeck/terraform-provider-clerk
// for use with pulumi-terraform-bridge. It re-exports the provider factory so the
// main provider module can import it without taking on a direct dependency on the
// upstream module's internal packages.
//
// This package's module is declared as a sub-path of the upstream module
// (github.com/buildwithdeck/terraform-provider-clerk/pulumi-bridge-shim) so that
// Go's module system cleanly separates upstream dependencies from the bridge layer.
// See shim/go.mod for details.
package shim

import (
	upstream "github.com/buildwithdeck/terraform-provider-clerk/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider"
)

// NewProvider returns a new instance of the upstream Clerk Terraform provider.
// The version string is intentionally "dev" here; the bridge injects the correct
// version at runtime via ldflags in the main provider binary.
func NewProvider() provider.Provider {
	// upstream.New returns a provider factory func() provider.Provider.
	// Call it immediately to get the concrete provider instance.
	return upstream.New("dev")()
}
