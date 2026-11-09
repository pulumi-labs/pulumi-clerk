# Changelog

All notable changes to the Clerk Pulumi provider are documented here.

## [Unreleased]

### Fixed

- BREAKING: `ApplicationSettings`: deleting the resource (or `pulumi destroy`) no longer disables Organizations on the instance. It only removes the settings from state and leaves the instance as it is, because the settings are a singleton every instance has, turning Organizations off affects every user of the instance (not only what this stack created) and Clerk keeps no earlier value to restore. The Terraform provider disabled Organizations on delete; this one deliberately does not. If you relied on destroy turning Organizations off, set `enableOrganizations: false` and apply before removing the resource. No new input. Mirror issue https://github.com/pulumi-labs/pulumi-clerk/issues/12 (#1683).
- Security: updated the indirect dependency `github.com/klauspost/compress` from v1.18.7 to v1.20.2, which fixes GO-2026-6865 (GHSA-fpmh-frq8-7rj8, a panic on two-block frames). No behaviour changes.
- Security: `SvixWebhook.svixUrl` and the `getSvixWebhook` result `svixUrl` are secrets. The Svix dashboard URL embeds a login key (`https://app.svix.com/login#key=...`) that grants access to the webhook dashboard and its signing secrets, but it was stored and printed as plaintext in state and outputs. A plaintext value already in the state becomes a secret on the next refresh. Mirror issue https://github.com/pulumi-labs/pulumi-clerk/issues/10 (#1630).
- Security: `getInstanceConfig` returns `config` as a secret, like `InstanceConfig.config`, because instance configuration can contain provider client secrets; a lookup result that was plaintext in the state is a secret now. `Read` also marks a secret nested inside an object (`ApplicationInstance.secretKey`) and retries loading the schema after an error instead of caching the failure (#1599).
- Security: if the provider schema cannot be loaded, `Read` now fails instead of returning the recovered secrets unmarked (which would have stored them as plaintext in the state). Behaviour is otherwise unchanged from the #1564 fix: every schema-secret property is returned as a secret on import and refresh (#1577).
- Changing the provider configuration (a rotated token, API key or secret, a new endpoint or region) is an in-place update of the default provider. By default pulumi-go-provider replaced the provider on any config change, and replacing a provider replaces every resource that uses it, so rotating a credential would have deleted and recreated every resource. Each resource's own diff still decides whether it changes. Mirror issue https://github.com/pulumi-labs/pulumi-huggingface/issues/8 (#1591).
- The six `examples/team-auth-*` programs and the README snippet pass the permission's `id` to `OrganizationRole.permissions`, which takes permission IDs (`perm_...`), not keys. With the key, `pulumi up` failed with HTTP 404 `Organization permission not found`. Mirror issue https://github.com/pulumi-labs/pulumi-clerk/issues/8 (#1568).
- `templates/clerk-cloud-yaml` runs: its `claimsJson` config value is passed through `fn::toJSON`. The YAML runtime parses a JSON-looking config value into an object even when it is declared as a string, so `claims: ${claimsJson}` failed `pulumi preview` with `Cannot assign 'Map<any>' to 'string'` (default `{}` and real claims alike). Checked with `pulumi preview` against the built provider for the default, `{"qa":"arqa"}` and unset config; a Go test keeps the template wired through `fn::toJSON`. Mirror issue https://github.com/pulumi-labs/pulumi-clerk/issues/7 (#1567).
- Secrets recovered by `Read` are stored as secrets. On import there is no prior state, and the engine kept only the secretness it was given, so `Machine.secretKey` and `ApiKey.secret` were written to state as plaintext (pulumi-labs/pulumi-clerk#4). Every output the schema marks secret is now returned as a secret by Read, for all resources.
- `AllowlistIdentifier`: setting `notify` on an entry that has no recorded value (an imported entry, since Clerk never reports it, or one created without it) is no change. It used to delete and recreate the entry, and with `notify: true` send the invitation email again (pulumi-labs/pulumi-clerk#2). Changing a recorded `notify`, or the identifier, still replaces the entry (a changed `notify` deletes the old entry first, since Clerk rejects a duplicate).
- Import no longer copies Clerk's server-side defaults into inputs for `JwtTemplate` (`lifetime`, `allowedClockSkew`, `customSigningKey`, `signingAlgorithm`), `Machine` (`defaultTokenTtl`), `OauthApplication` (`consentScreenEnabled`) and `ApplicationSettings` (all optional settings). Every imported resource planned a pending update against the program that created it; the defaults stay readable in the `effective*` / `current*` outputs. A program that declares one of them after an import sees an in-place update (a write of the same value) at most (pulumi-labs/pulumi-clerk#3).
- `OauthApplication.scopes`: order and the implicit scopes Clerk adds are not a change (`profile email` matches Clerk's `email offline_access profile`), as the property description already promised. The resource now has an explicit diff for this.
- `OrganizationDomain`: creating a domain without `enrollmentMode` failed with HTTP 422, because Clerk requires the parameter and has no default. It now sends `manual_invitation` (the most restrictive mode) when the input is unset; the description no longer claims an instance default and lists `enterprise_sso` (pulumi-labs/pulumi-clerk#5).
- `getOrganizationDomain` accepts the `OrganizationDomain` resource id (`<organizationId>/<domainId>`) as `organizationDomainId`, as well as the bare domain id; passing the resource's `id` output used to fail with "was not found". The error text now names `organizationDomainId` (pulumi-labs/pulumi-clerk#6).

## [0.1.0] - 2026-11-04

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
