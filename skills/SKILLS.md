# Clerk Provider -- Engineering Notes

## Overview

This provider bridges the community
[buildwithdeck/terraform-provider-clerk](https://github.com/buildwithdeck/terraform-provider-clerk)
v0.6.1 via the Pulumi Terraform Bridge (TPF / terraform-plugin-framework path).

**Bridge path**: TPF (terraform-plugin-framework v1.17.0), NOT SDKv2.
Use `pf.ShimProvider(...)` in resources.go, not `shimv2.NewProvider(...)`.

## Key Technical Facts

### Upstream Provider

- **Module**: `github.com/buildwithdeck/terraform-provider-clerk`
- **Tag**: `v0.6.1` (2026-04-06) -- pinned; see Schema Churn section below
- **Entry point**: `internal/provider.New(version string) func() provider.Provider`
- **TF registry**: `registry.terraform.io/buildwithdeck/clerk`
- **Upstream SDK**: `github.com/clerk/clerk-sdk-go/v2 v2.5.1`

### Token Scheme

Single module strategy: `tokens.SingleModule("clerk_", "index", tokens.MakeStandard("clerk"))`

All 20 resources and 1 data source are explicitly declared in resources.go as a safety
net against automatic token computation producing wrong names.

### Authentication

Two API key scopes:

| Config key | Env var | Scope |
|---|---|---|
| `api_key` | `CLERK_API_KEY` | Instance API -- most resources |
| `platform_api_key` | `CLERK_PLATFORM_API_KEY` | Platform API beta -- Application, Domain, InstanceConfig, InstanceDomain |

Most users only need `CLERK_API_KEY`. Platform API is in beta and requires a separate
key. Document this clearly in examples and README.

## Resources by API Scope

### Instance API (standard, `CLERK_API_KEY`)

These work for any Clerk instance with a regular secret key:

- AllowlistIdentifier, BlocklistIdentifier
- ApiKey, Machine
- ApplicationSettings
- JwtTemplate
- OauthApplication
- Organization, OrganizationDomain, OrganizationMembership
- OrganizationPermission, OrganizationRole, RoleSet
- RedirectUrl
- SamlConnection
- SvixWebhook

### Platform API beta (`CLERK_PLATFORM_API_KEY`)

These require Platform API beta access:

- Application
- Domain
- InstanceConfig
- InstanceDomain
- getApplication (data source)

**Example guideline**: Do NOT use Platform API resources in general-purpose examples.
Stick to Instance API resources so examples work for any Clerk user.

## Build Notes

### Required go.mod Replace Directive

The main provider go.mod MUST include:

```
replace github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20260318212141-5525259d096b
```

This is required because pulumi-terraform-bridge uses a fork of terraform-plugin-sdk/v2
that adds CloseEphemeralResource support. The replace directive from the bridge's own
go.mod is not automatically inherited, so we mirror it explicitly.

Without this, `go build` fails with:
```
cannot use (*GRPCProviderServer)(nil) as tfprotov5.ProviderServer value:
*GRPCProviderServer does not implement tfprotov5.ProviderServer (missing method GenerateResourceConfig)
```

### Shim Module

Located at `provider/shim/`. Module name:
`github.com/buildwithdeck/terraform-provider-clerk/pulumi-bridge-shim`

The shim imports `github.com/buildwithdeck/terraform-provider-clerk/internal/provider`
directly. The `internal/` package is accessible because the shim module is declared as
a sub-path of the upstream module path.

### Build Commands

```bash
# Tidy dependencies
make tidy

# Build provider and tfgen binaries
make build

# Run tests (8 tests, all must pass)
make test

# Generate schema (requires pulumictl + pulumi CLI)
make generate_schema
```

## Schema Churn Warning

The upstream `buildwithdeck/terraform-provider-clerk` is **pre-1.0**. Breaking changes
between versions are expected. When upgrading from v0.6.1:

1. Bump the version in `provider/shim/go.mod`
2. Run `make tidy`
3. Verify the resource count in `resources_test.go` still matches
4. If resources were added/removed, update the explicit map in `resources.go`
5. Re-run `make test` and `make generate_schema`

## Common Pitfalls

1. **Using SDKv2 bridge path**: This provider uses TPF. Never use `shimv2.NewProvider()`
   here; always use `pf.ShimProvider(shim.NewProvider())`.

2. **Forgetting the tf-plugin-sdk replace directive**: See Build Notes above. Without it,
   the provider compiles against the wrong version of tf-plugin-sdk and panics.

3. **Phantom resources**: Do not add resources not present in upstream v0.6.1. The test
   file explicitly checks for 20 resources and 1 data source. Run `make test` to verify.

4. **Platform API resources in general examples**: The team-auth examples deliberately
   avoid Platform API resources. Keep it that way -- not all users have Platform API access.

5. **JSON claims in JwtTemplate**: The `claims` field is a JSON string, not an object.
   In TypeScript use `JSON.stringify({...})`, in Python use `json.dumps({...})`, in Java
   use `ObjectMapper` to serialize a Map, and in Go use `json.Marshal(map[string]string{...})`.
