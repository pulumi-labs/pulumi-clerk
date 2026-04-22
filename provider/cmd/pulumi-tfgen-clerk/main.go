// Command pulumi-tfgen-clerk generates the Pulumi schema and multi-language SDKs
// by introspecting the bridged Clerk Terraform provider at build time.
package main

import (
	clerk "github.com/pulumi/pulumi-clerk/provider"
	pftfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
)

func main() {
	pftfgen.Main("clerk", clerk.Provider())
}
