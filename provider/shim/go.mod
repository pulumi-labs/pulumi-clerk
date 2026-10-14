// This shim module is declared as a sub-path of the upstream
// github.com/buildwithdeck/terraform-provider-clerk module. The shim is
// an independent Go module that imports the upstream provider package; by
// placing it in a sub-path, we follow the established pattern used for TPF
// providers in pulumi/autoregistry.
//
// Declaring the shim here as
// github.com/buildwithdeck/terraform-provider-clerk/pulumi-bridge-shim
// matches the replace directive in the main provider go.mod.
module github.com/buildwithdeck/terraform-provider-clerk/pulumi-bridge-shim

go 1.26.0

require (
	github.com/buildwithdeck/terraform-provider-clerk v0.6.1
	github.com/hashicorp/terraform-plugin-framework v1.17.0
)

require (
	github.com/clerk/clerk-sdk-go/v2 v2.5.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.5 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/terraform-plugin-go v0.29.0 // indirect
	github.com/hashicorp/terraform-plugin-log v0.10.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)
