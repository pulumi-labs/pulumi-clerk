# Changelog

All notable changes to the Clerk Pulumi provider are documented here.

## [Unreleased]

### Changed

- The provider is now native (`pulumi-go-provider`) instead of bridged from `buildwithdeck/terraform-provider-clerk` v0.6.1 (#1413). It calls the Clerk Backend API and the Platform API (beta) directly. Package name, `clerk:index:<Resource>` tokens and the `clerk:apiKey` / `clerk:platformApiKey` config keys are unchanged.
- BREAKING: lookup functions. The `getApplication` data source is replaced by native lookup functions, and every resource now has a `get<Name>` function. Function arguments and results cannot be named `id`, so `getApplication` now takes and returns `applicationId` (was `id`).
- BREAKING: `OrganizationDomain` and `OrganizationMembership` IDs are composite (`<organizationId>/<id>`); the Clerk membership ID is exposed as `membershipId`. Existing state is not migrated: re-import or re-create.
- BREAKING: where Clerk fills in a default for an optional input, the default is now an output next to the input instead of overwriting it: `effective*` on `JwtTemplate`, `Machine`, `OauthApplication`, `ApiKey`, `SamlConnection` and `InstanceDomain`, `current*` on `ApplicationSettings`. Programs that read the old computed value from the input property should read the new output.
- BREAKING: `Application.instances` is now secret as a whole (it holds instance secret keys).
- `SamlConnection.idpCertificate` is no longer marked secret (it is a public certificate); `InstanceConfig.config` and `Organization.privateMetadata` are now secret.
- `ApiKey.name` and `ApiKey.createdBy` now force replacement (the API cannot change them); `OauthApplication`, `Machine` and `ApiKey` secrets are kept in state.
- `Machine` and `ApiKey` secrets are recovered on import.
- The Terraform shim, `bridge-metadata.json` and the tfgen binary are gone. The `Makefile` follows the other native providers (`make build`, `make gen-sdk`).
- Templates use the default provider; credentials are declared as clerk:apiKey in template config (#1183).
- The `team-auth` examples use `callbackUrl` (a single URL) for `OauthApplication`; the Java examples no longer need Jackson (#1183).

### Added

Provider resources:

- `clerk:index:AllowlistIdentifier` -- Allowlist entries for restricted-signup instances.
- `clerk:index:ApiKey` -- (Instance) Manage static API keys for M2M authentication.
- `clerk:index:Application` -- (Platform API, beta) Provision entire Clerk applications.
- `clerk:index:ApplicationSettings` -- Instance-level organization feature settings.
- `clerk:index:BlocklistIdentifier` -- Blocklist entries to prevent specific users from signing up.
- `clerk:index:Domain` -- (Platform API, beta) Application-level custom domains.
- `clerk:index:InstanceConfig` -- (Platform API, beta) Instance-wide authentication configuration.
- `clerk:index:InstanceDomain` -- (Platform API, beta) Per-instance domain assignment.
- `clerk:index:JwtTemplate` -- Custom JWT claim templates for access and ID tokens.
- `clerk:index:Machine` -- M2M tokens and service accounts.
- `clerk:index:OauthApplication` -- OAuth 2.0 application registrations.
- `clerk:index:Organization` -- Organizations for B2B multi-tenancy.
- `clerk:index:OrganizationDomain` -- Email-domain-based organization auto-join rules.
- `clerk:index:OrganizationMembership` -- User memberships in organizations.
- `clerk:index:OrganizationPermission` -- Custom permissions scoped to organizations.
- `clerk:index:OrganizationRole` -- Custom org roles with associated permission lists.
- `clerk:index:RedirectUrl` -- OAuth redirect URL allowlist entries.
- `clerk:index:RoleSet` -- Named collections of OrganizationRoles.
- `clerk:index:SamlConnection` -- SAML SSO connections for enterprise authentication.
- `clerk:index:SvixWebhook` -- Webhook endpoint subscriptions via the Svix platform.

Provider data sources:

- `clerk:index:getApplication` -- Data source: look up Clerk application metadata (Platform API beta).

Templates (starter scaffolds, one resource each):

- `templates/clerk-cloud-ts` -- TypeScript
- `templates/clerk-cloud-python` -- Python
- `templates/clerk-cloud-go` -- Go
- `templates/clerk-cloud-csharp` -- C#
- `templates/clerk-cloud-java` -- Java
- `templates/clerk-cloud-yaml` -- YAML

Examples (two scenarios, six languages each = 12 examples):

- `examples/jwt-templates-{ts,python,go,csharp,java,yaml}` -- Two JWT templates
  demonstrating access_token and id_token claim patterns.
- `examples/team-auth-{ts,python,go,csharp,java,yaml}` -- Team authorization scenario:
  Organization, OrganizationRole, OrganizationPermission, JwtTemplate (org-aware claims),
  and OauthApplication (CI/CD integration). Uses instance API key only.
