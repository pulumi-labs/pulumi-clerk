// Command pulumi-resource-clerk is the provider resource server binary.
// It is invoked by the Pulumi CLI at runtime to serve resource CRUD operations.
package main

import (
	"context"

	clerk "github.com/pulumi/pulumi-clerk/provider"
	pf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
)

// PackageSchema is the Pulumi package schema produced by `make generate_schema`.
// This placeholder is replaced by the real schema bytes during the build; it must be
// non-nil so the bridge runtime assertion (exactly one of PackageSchema/XGetSchema must
// be set) is satisfied in pre-tfgen scaffolds.
var PackageSchema = []byte("{}")

func main() {
	meta := pf.ProviderMetadata{PackageSchema: PackageSchema}
	pf.Main(context.Background(), "clerk", clerk.Provider(), meta)
}
